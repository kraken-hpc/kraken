/* events.go: event handling methods for imageapi
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

package imageapi

import (
	fmt "fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/kraken-hpc/kraken/core"
	cpb "github.com/kraken-hpc/kraken/core/proto"
	ia "github.com/kraken-hpc/kraken/extensions/imageapi"
	"github.com/kraken-hpc/kraken/lib/types"
	"github.com/kraken-hpc/kraken/lib/util"
)

/////////////////////
// Event handlers //
///////////////////

func (is *ImageAPI) handleMutation(m types.Event) {
	if m.Type() != types.Event_STATE_MUTATION {
		is.api.Log(types.LLINFO, "got an unexpected event type on mutation channel")
	}
	me := m.Data().(*core.MutationEvent)

	// mutation switch
	switch me.Type {
	case core.MutationEvent_MUTATE:
		is.clearAllTriggers()
		is.clearAllActions()
		is.clearRunning()
		switch me.Mutation[1] {
		case "UKtoIDLE":
			is.mUKtoIDLE(me)
		case "UPDATEtoACTIVE":
			time.Sleep(1 * time.Second) // FIXME: this is a hack to make sure more updates have accumulated
			fallthrough
		case "IDLEtoACTIVE":
			is.mANYtoACTIVE(me)
		case "ACTIVEtoIDLE", "UPDATEtoIDLE":
			is.mANYtoIDLE(me)
		case "ERRORtoACTIVE":
			if is.mRecoverError(me) {
				is.mANYtoACTIVE(me)
			}
		case "ERRORtoIDLE":
			if is.mRecoverError(me) {
				is.mANYtoIDLE(me)
			}
		case "SYNCtoINIT":
			// we just directly discover the state
			if is.pollTicker != nil {
				is.pollTicker.Stop()
			}
			url := util.NodeURLJoin(is.api.Self().String(), issURL)
			// fixme: this should really be automatically handled by the sme
			is.dchan <- core.NewEvent(
				types.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{
					URL:     url,
					ValueID: ia.ImageState_UNKNOWN.String(),
				},
			)
			url = util.NodeURLJoin(is.api.Self().String(), "/RunState")
			is.dchan <- core.NewEvent(
				types.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{
					URL:     url,
					ValueID: cpb.Node_INIT.String(),
				},
			)
		default:
			is.api.Logf(types.LLERROR, "asked to perform unknown mutation: %s", me.Mutation[1])
		}
	case core.MutationEvent_INTERRUPT:
		is.clearAllActions()
		is.clearAllTriggers()
		is.clearRunning()
	}
}

// we only care about changes to containers themselves
var reEvent = regexp.MustCompile(fmt.Sprintf(`^\/?%s\/Images/([A-Za-z.\-_]+)(\/?.*)$`, regexp.QuoteMeta(isURL)))

// handle event handles the flood of all events and filters for state changes that we care about
func (is *ImageAPI) handleEvent(m types.Event) {
	if m.Type() != types.Event_STATE_CHANGE {
		return
	}
	// we only care about self events having to do with imagestate
	node, url := util.NodeURLSplit(m.URL())
	if node != is.api.Self().String() {
		// this event isn't about us
		return
	}
	match := reEvent.FindAllStringSubmatch(url, -1)
	if len(match) != 1 {
		// not our url
		return
	}
	name := match[0][1]
	sub := match[0][2]
	// ok, this is a self ImageState statechanmge event
	sce := m.Data().(*core.StateChangeEvent)
	switch sce.Type {
	case cpb.StateChangeControl_CFG_UPDATE:
		// container configuration changed
		is.cfgChange(name, sub)
	case cpb.StateChangeControl_UPDATE:
		// container state changed
		is.dscChange(name, sub, sce.Value)
	}
}

// cfg changes general mark discover nodes as needing some kind of update
func (is *ImageAPI) cfgChange(name, sub string) {
	cfg, _ := is.api.QueryRead(is.api.Self().String())
	dsc, _ := is.api.QueryReadDsc(is.api.Self().String())
	ctnURL := util.URLPush(util.URLPush(isURL, "Images"), name)
	if _, err := cfg.GetValue(ctnURL); err != nil {
		// this Image is no longer configured.  It should be marked for deletion
		if _, err := dsc.GetValue(ctnURL); err != nil {
			// it seems to have already been deleted.  We're done.
			is.updateSetState()
			return
		}
		// mark node for update/delete
		is.setValues(map[string]interface{}{
			util.URLPush(ctnURL, "Action"): ia.Image_DELETE,
			util.URLPush(ctnURL, "State"):  ia.ImageState_UPDATE,
		})
	} else {
		_, err := dsc.GetValue(ctnURL)
		if err != nil {
			// this is a newly defined image
			is.api.Logf(types.LLINFO, "new image definition found for %s", name)
			is.setValues(map[string]interface{}{
				ctnURL: ia.Image{
					Container: &ia.Container{State: ContainerState_DELETED},
					State:     ia.ImageState_UPDATE,
					Action:    ia.Image_CREATE,
				}})
		} else {
			// this Image needs to be updated/reloaded
			is.setValues(map[string]interface{}{
				util.URLPush(ctnURL, "Action"): ia.Image_RELOAD,
				util.URLPush(ctnURL, "State"):  ia.ImageState_UPDATE,
			})
		}
	}
	is.updateSetState()
}

// dsc changes mean something about the container(s) changed
// We handle four cases:
// 1) /Action changes, in which case we activate
// 2) /Container/State changes in which case we fire triggers or handle unexpected
// 3) /State changes, in which case we call updateSetState
// 4) "" change, in which case we assume everything changed
func (is *ImageAPI) dscChange(name, sub string, value reflect.Value) {
	ctnURL := util.URLPush(util.URLPush(isURL, "Images"), name)
	v, err := is.api.QueryGetValueDsc(is.api.Self().String(), ctnURL)
	if err != nil || v == nil {
		// image was deleted
		is.api.Logf(types.LLDDDEBUG, "firing any triggers for no longer defined image: %s", name)
		is.fireTriggers(name, ContainerState_DELETED)
		is.updateSetState()
		return
	}

	idc := v.(ia.Image)
	if idc.Container == nil { // deleted
		is.api.Logf(types.LLDDDEBUG, "firing any triggers for no longer defined container: %s", name)
		is.fireTriggers(name, ContainerState_DELETED)
		is.updateSetState()
		return
	}

	switch sub {
	case "/Action":
		is.api.Logf(types.LLDDDEBUG, "processing dsc change path: %s", sub)
		is.activateImage(name)
	case "/State":
		is.api.Logf(types.LLDDDEBUG, "processing dsc change path: %s", sub)
		is.updateSetState()
	case "":
		// Action may have been included in a whole-image change
		if idc.Action != is.getAction(name) {
			is.activateImage(name)
		}
		fallthrough
	case "/Container/State":
		is.api.Logf(types.LLDDDEBUG, "processing dsc change path: %s", sub)
		if is.fireTriggers(name, idc.Container.State) {
			// we were expecting this change and we had triggers to fire
			is.updateSetState()
			return
		} else {
			is.unexpectedStateChange(name)
			is.updateSetState()
		}
	default:
		is.api.Logf(types.LLDDDEBUG, "not processing dsc change path: %s", sub)
	}
}

func (is *ImageAPI) unexpectedStateChange(name string) {
	ctnURL := util.URLPush(util.URLPush(isURL, "Images"), name)
	dsc, err := is.api.QueryGetValueDsc(is.api.Self().String(), ctnURL)
	if err != nil {
		return
	}
	idsc := dsc.(ia.Image)
	cfg, err := is.api.QueryGetValue(is.api.Self().String(), ctnURL)
	if err != nil {
		return
	}
	icfg := cfg.(ia.Image)
	target := is.target
	is.api.Logf(types.LLERROR, "got an unexpected state change for image container %s: %s", name, idsc.Container.State)

	if target == ia.ImageState_IDLE {
		if idsc.Container.State != ContainerState_DELETED {
			// we need to delete it
			is.setAction(name, ia.Image_DELETE)
			is.deleteImage(name, &icfg)
			is.setTrigger(name, ContainerState_DELETED, is.clearAction)
		}
		return
	}

	switch idsc.Container.State {
	case ContainerState_DEAD:
		if idsc.State != ia.ImageState_ERROR && idsc.State != ia.ImageState_FATAL {
			is.imageRaiseError(name, ia.Image_DIED)
		}
	case ContainerState_EXITED:
		if icfg.Container.State != ContainerState_EXITED {
			if idsc.State != ia.ImageState_ERROR && idsc.State != ia.ImageState_FATAL {
				is.imageRaiseError(name, ia.Image_DIED)
			}
		}
	case ContainerState_RUNNING:
		// TODO: we should probably deal with unexpected running states
	case ContainerState_STOPPING:
	case ContainerState_CREATED:
	}
}
