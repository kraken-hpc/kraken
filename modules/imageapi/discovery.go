/* discovery.go: methods for handling imageapi state discovery
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

package imageapi

import (
	math "math"

	"github.com/hpc/kraken/core"
	ia "github.com/hpc/kraken/extensions/imageapi"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
	"github.com/jlowellwofford/imageapi/client/containers"
)

////////////////////////
// Discovery methods //
//////////////////////

// discover's job is to periodically poll the imageapi for current data and update discoverable state
// discover does not make decisions on ImageState
// ironically, discover uses SetValue + Update instead of discover, since it updates a complex data structure
func (is *ImageAPI) discover() {
	is.mutex.Lock()
	is.api.Logf(types.LLDDDEBUG, "initiating image discovery")
	iscv, _ := is.api.QueryGetValue(is.api.Self().String(), isURL)
	isc := iscv.(ia.ImageSet)
	isdv, _ := is.api.QueryGetValueDsc(is.api.Self().String(), isURL)
	isd := isdv.(ia.ImageSet)
	client := is.getAPIClient()
	resp, err := client.Containers.ListContainers(containers.NewListContainersParams())
	if err != nil {
		// set error stuff
		is.mutex.Unlock()
		if cerr, ok := err.(*containers.ListContainersDefault); ok {
			is.api.Logf(types.LLERROR, "failed to list containers: Code: %d Message: %s", cerr.Payload.Code, *cerr.Payload.Message)
		} else {
			is.api.Logf(types.LLERROR, "failed to call imageapi: %v", err)
		}
		return
	}
	ctnsList := resp.GetPayload()

	// dset is where we will build the ImageSet that we will discover for the node
	is.api.Logf(types.LLDDDEBUG, "discover found %d containers", len(ctnsList))
	dset := &ia.ImageSet{
		State:  isd.State,              // we preserve state values
		Images: map[string]*ia.Image{}, // but start with an empty list
	}
	for _, ca := range ctnsList {
		cp := is.apiToProto(ca)
		if cp == nil { // if conversion fails, don't crash, just report an error
			is.api.Logf(types.LLERROR, "apiToProto conversion failed for container object")
			continue
		}
		if ds, ok := isd.Images[cp.Name]; ok {
			// this is one we already knew
			ds.Container = cp
			dset.Images[cp.Name] = ds
			delete(isc.Images, cp.Name) // delete it so we know what we *didn't* find later
		} else {
			// this is a new one
			dset.Images[cp.Name] = &ia.Image{
				Container: cp,
			}
		}
	}
	// we now have a list of everything imageapi knows about...
	// we also want at least a stub for anything in the cfg set
	for name, cp := range isc.Images {
		action := ia.Image_CREATE
		if cur, ok := is.actions[name]; ok && cur == ia.Image_RELOAD {
			// don't clobber the reload action
			action = ia.Image_RELOAD
		}
		if cp.Container.State != ContainerState_RUNNING {
			action = ia.Image_NONE
		}
		if _, ok := dset.Images[name]; !ok {
			// we don't have an entry, does dsc?
			if ds, ok := isd.Images[name]; ok {
				ds.Container = &ia.Container{State: ContainerState_DELETED}
				ds.Action = action
				dset.Images[name] = ds
			} else {
				// nope, make a stub
				dset.Images[name] = &ia.Image{
					Container: &ia.Container{State: ContainerState_DELETED},
					State:     ia.ImageState_IDLE,
					Action:    action,
				}
			}
		}
	}

	// ok, we're ready to ship it
	// we do a query instead of a discover because we're setting various values
	is.api.QuerySetValueDsc(is.api.Self().String(), isURL, *dset)
	is.mutex.Unlock()
}

// updateSetState looks at all of the individual image states and derives the set state
// it then discovers the state if it varies from the exiting one
func (is *ImageAPI) updateSetState() {
	v, _ := is.api.QueryGetValueDsc(is.api.Self().String(), isURL)
	s := v.(ia.ImageSet)

	state := float64(ia.ImageState_UNKNOWN)

	for _, img := range s.Images {
		// we rely on the numeric sequence here
		state = math.Max(float64(img.State), state)
	}

	// this is a special case.  If there are no images, the state is whatever we want it to be.
	v, _ = is.api.QueryGetValue(is.api.Self().String(), isURL)
	cs := v.(ia.ImageSet)
	if len(cs.Images) == 0 && len(s.Images) == 0 {
		state = float64(is.target)
	}

	if ia.ImageState(state) != s.State {
		// discover the imagestate update
		// this always means we have to stop running
		is.clearRunning()
		is.clearAllActions()
		is.clearAllTriggers()
		is.dchan <- core.NewEvent(
			types.Event_DISCOVERY,
			util.NodeURLJoin(is.api.Self().String(), issURL),
			&core.DiscoveryEvent{
				URL:     util.NodeURLJoin(is.api.Self().String(), issURL),
				ValueID: ia.ImageState(state).String(),
			},
		)
	}
}
