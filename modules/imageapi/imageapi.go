/* imageapi.go: mutations for image management via the ImageAPI
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. imageapi-config.proto

/*
 * This module will manipulate all or most of the Image extension fields.
 * The primary variable used for mutaiton is `ImageSet.State`
 */

package imageapi

import (
	"encoding/json"
	"fmt"
	math "math"
	"os"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	transport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	proto "github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	ia "github.com/hpc/kraken/extensions/imageapi"
	kjson "github.com/hpc/kraken/lib/json"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
	api "github.com/jlowellwofford/imageapi/client"
	"github.com/jlowellwofford/imageapi/client/containers"
	"github.com/jlowellwofford/imageapi/models"
)

////////////////////////////////////
// Mutation & Discovers fixtures //
//////////////////////////////////

const siName = "imageapi"                                    // name of our default service instance
const siURL = "/Services/imageapi/State"                     // URL to set our own state
const isURL = "type.googleapis.com/ImageAPI.ImageSet"        // state URL to the ImageSet object
const issURL = "type.googleapis.com/ImageAPI.ImageSet/State" // state URL to the ImageSet State value

// discovery structure for anything we don't mutate to
var discovers = map[string]map[string]reflect.Value{
	// fatal and error states are not implied by mutations
	issURL: {
		ia.ImageState_ERROR.String():   reflect.ValueOf(ia.ImageState_ERROR),
		ia.ImageState_FATAL.String():   reflect.ValueOf(ia.ImageState_FATAL),
		ia.ImageState_UPDATE.String():  reflect.ValueOf(ia.ImageState_UPDATE),
		ia.ImageState_UNKNOWN.String(): reflect.ValueOf(ia.ImageState_UNKNOWN),
	},
	siURL: {
		cpb.ServiceInstance_RUN.String(): reflect.ValueOf(cpb.ServiceInstance_RUN),
	},
	"/RunState": {
		cpb.Node_INIT.String():  reflect.ValueOf(cpb.Node_INIT),
		cpb.Node_ERROR.String(): reflect.ValueOf(cpb.Node_ERROR),
	},
}

// ismut helps us succinctly define our mutations
type ismut struct {
	f       ia.ImageState // from
	t       ia.ImageState // to
	timeout string        // timeout
	failto  [3]string
	reqs    map[string]reflect.Value
	excs    map[string]reflect.Value
}

// any mut that doesnt' provide reqs or excs will get these defaults
var reqs = map[string]reflect.Value{
	"/RunState": reflect.ValueOf(cpb.Node_SYNC),
}
var excs = map[string]reflect.Value{
	"/Busy": reflect.ValueOf(cpb.Node_BUSY),
}

// our mutation definitions
// also we discover anything we can mutate to
// also set the handler for mutations
var muts = map[string]ismut{
	// discovery mutation
	"UKtoIDLE": {
		f:       ia.ImageState_UNKNOWN,
		t:       ia.ImageState_IDLE,
		timeout: "10s",
		failto:  [3]string{siName, issURL, ia.ImageState_FATAL.String()},
		reqs: map[string]reflect.Value{
			"/RunState":  reflect.ValueOf(cpb.Node_SYNC),
			"/PhysState": reflect.ValueOf(cpb.Node_POWER_ON),
		},
		excs: map[string]reflect.Value{},
	},
	"IDLEtoACTIVE": {
		f:       ia.ImageState_IDLE,
		t:       ia.ImageState_ACTIVE,
		timeout: "1m",
		failto:  [3]string{siName, issURL, ia.ImageState_ERROR.String()},
	},
	"ACTIVEtoIDLE": {
		f:       ia.ImageState_ACTIVE,
		t:       ia.ImageState_IDLE,
		timeout: "1m",
		failto:  [3]string{siName, issURL, ia.ImageState_ERROR.String()},
	},
	"UPDATEtoACTIVE": {
		f:       ia.ImageState_UPDATE,
		t:       ia.ImageState_ACTIVE,
		timeout: "1m",
		failto:  [3]string{siName, issURL, ia.ImageState_ERROR.String()},
	},
	"UPDATEtoIDLE": {
		f:       ia.ImageState_UPDATE,
		t:       ia.ImageState_IDLE,
		timeout: "1m",
		failto:  [3]string{siName, issURL, ia.ImageState_ERROR.String()},
	},
	// we can mutate out of error (but not fatal)
	"ERRORtoACTIVE": {
		f:       ia.ImageState_ERROR,
		t:       ia.ImageState_ACTIVE,
		timeout: "1m", // systemd can take a bit to stop, for instance
		failto:  [3]string{siName, issURL, ia.ImageState_FATAL.String()},
	},
	"ERRORtoIDLE": {
		f:       ia.ImageState_ERROR,
		t:       ia.ImageState_IDLE,
		timeout: "1m",
		failto:  [3]string{siName, issURL, ia.ImageState_FATAL.String()},
	},
}

////////////////////////////
// Module Initialization //
//////////////////////////

func init() {
	mutations := make(map[string]types.StateMutation)
	// create mutation/discovers from our muts
	for m, mut := range muts {
		dur, _ := time.ParseDuration(muts[m].timeout)
		if mut.reqs == nil {
			mut.reqs = reqs
		}
		if mut.excs == nil {
			mut.excs = excs
		}
		mutations[m] = core.NewStateMutation(
			map[string][2]reflect.Value{
				issURL: {
					reflect.ValueOf(mut.f),
					reflect.ValueOf(mut.t),
				},
			},
			mut.reqs,
			mut.excs,
			types.StateMutationContext_SELF,
			dur,
			mut.failto,
		)
		discovers[issURL][mut.t.String()] = reflect.ValueOf(mut.t)
	}
	// we use SYNCtoINIT to make a barrier against powering off a node if it's in use
	mutations["SYNCtoINIT"] = core.NewStateMutation(
		map[string][2]reflect.Value{
			"/RunState": {
				reflect.ValueOf(cpb.Node_SYNC),
				reflect.ValueOf(cpb.Node_INIT),
			},
		},
		map[string]reflect.Value{
			issURL: reflect.ValueOf(ia.ImageState_IDLE),
		},
		map[string]reflect.Value{
			"/Busy": reflect.ValueOf(cpb.Node_BUSY),
		},
		types.StateMutationContext_SELF,
		time.Second*10,
		[3]string{siName, "/RunState", cpb.Node_ERROR.String()},
	)

	// Register it all
	module := &ImageAPI{}
	si := core.NewServiceInstance(siName, module.Name(), module.Entry)
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}

/////////////////////
// ImageAPI Object /
///////////////////

type trigger func(name string)

// ImageAPI provides layer1 image loading capabilities
type ImageAPI struct {
	api        types.ModuleAPIClient
	cfg        *Config
	mchan      <-chan types.Event
	dchan      chan<- types.Event
	echan      <-chan types.Event
	pollTicker *time.Ticker
	target     ia.ImageState
	triggers   map[string]map[string][]trigger // action triggers assigned to images
	running    bool                            // are we currently cleared to make changes?
	actions    map[string]ia.Image_Action      // currently running actions
	mutex      *sync.Mutex
}

/*
 *types.Module
 */
var _ types.Module = (*ImageAPI)(nil)

// Name returns the FQDN of the module
func (*ImageAPI) Name() string { return "github.com/hpc/kraken/modules/imageapi" }

/*
 * types.ModuleWithAllEvents
 * We subscribe to events so that we can capture state change events we care about
 */
var _ types.ModuleWithAllEvents = (*ImageAPI)(nil)

func (ia *ImageAPI) SetEventsChan(c <-chan types.Event) {
	ia.echan = c
}

/*
 * types.ModuleWithConfig
 */
var _ types.ModuleWithConfig = (*ImageAPI)(nil)

// NewConfig returns a fully initialized default config
func (*ImageAPI) NewConfig() proto.Message {
	r := &Config{
		// there's no reasonable way to assign a default image server really...
		ApiServer: &ApiServer{
			Server:  "127.0.0.1",
			Port:    8080,
			Https:   false,
			ApiBase: "/imageapi/v1",
		},
		PollingInterval: "1s",
		MaxRetries:      3,
	}
	return r
}

// UpdateConfig updates the running config
func (is *ImageAPI) UpdateConfig(cfg proto.Message) (e error) {
	if iscfg, ok := cfg.(*Config); ok {
		is.cfg = iscfg
		if is.pollTicker != nil {
			is.pollTicker.Stop()
			dur, _ := time.ParseDuration(is.cfg.GetPollingInterval())
			is.pollTicker = time.NewTicker(dur)
		}
		return
		// we don't need to do anything else, they'll be update on the next attempt to use them
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*ImageAPI) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * types.ModuleWithMutations & types.ModuleWithDiscovery
 */
var _ types.ModuleWithMutations = (*ImageAPI)(nil)
var _ types.ModuleWithDiscovery = (*ImageAPI)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (is *ImageAPI) SetMutationChan(c <-chan types.Event) { is.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (is *ImageAPI) SetDiscoveryChan(c chan<- types.Event) { is.dchan = c }

/*
 * types.ModuleSelfService
 */
var _ types.ModuleSelfService = (*ImageAPI)(nil)

// Entry is the module's executable entrypoint
func (is *ImageAPI) Entry() {
	// declare ourselves running
	url := util.NodeURLJoin(is.api.Self().String(), siURL)
	is.dchan <- core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: cpb.ServiceInstance_RUN.String(),
		},
	)
	// setup a ticker for polling discovery
	dur, _ := time.ParseDuration(is.cfg.GetPollingInterval())
	is.pollTicker = time.NewTicker(dur)

	// main loop
	for {
		select {
		case <-is.pollTicker.C:
			go is.discover()
		case m := <-is.mchan: // mutation request
			go is.handleMutation(m)
		case e := <-is.echan: // capture events
			go is.handleEvent(e)
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (is *ImageAPI) Init(api types.ModuleAPIClient) {
	is.api = api
	is.cfg = is.NewConfig().(*Config)
	is.mutex = &sync.Mutex{}
	is.target = ia.ImageState_UNKNOWN
	is.triggers = make(map[string]map[string][]trigger)
	is.running = false
	is.actions = make(map[string]ia.Image_Action)
}

// Stop should perform a graceful exit
func (is *ImageAPI) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

////////////////////////
// Mutation handlers //
//////////////////////

func (is *ImageAPI) mUKtoIDLE(me *core.MutationEvent) {
	// force a discovery run and go to idle
	is.clearAllTriggers()
	is.mutex.Lock()
	is.target = ia.ImageState_IDLE
	is.mutex.Unlock()

	is.discover()
	url := util.NodeURLJoin(is.api.Self().String(), issURL)
	is.dchan <- core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: ia.ImageState_IDLE.String(),
		},
	)
}

func (is *ImageAPI) mANYtoACTIVE(me *core.MutationEvent) {
	// Activate all Images (if they aren't already)
	is.mutex.Lock()
	is.target = ia.ImageState_ACTIVE
	is.mutex.Unlock()
	is.setRunning()
	v, _ := is.api.QueryGetValueDsc(is.api.Self().String(), isURL)
	isd := v.(ia.ImageSet)
	for name, _ := range isd.Images {
		is.activateImage(name)
	}
}

func (is *ImageAPI) activateImage(name string) {
	if !is.isRunning() || is.target != ia.ImageState_ACTIVE {
		// do nothing
		return
	}
	var cfgImage, dscImage ia.Image

	imgURL := util.URLPush(util.URLPush(isURL, "Images"), name)
	v, err := is.api.QueryGetValueDsc(is.api.Self().String(), imgURL)
	if err != nil {
		// shouldn't have been called
		is.api.Logf(types.LLDDEBUG, "actiaveImage called on an image that has no dsc state: %s", name)
	}
	dscImage = v.(ia.Image)

	action := dscImage.Action
	if action == is.getAction(name) {
		// we're already doing it...
		is.api.Logf(types.LLDDDEBUG, "asked to perform an action we're already performing: %s on %s", action.String(), name)
		return
	}
	is.api.Logf(types.LLDDEBUG, "activating %s (%s), dsc: %v\n", name, action.String(), spew.Sdump(dscImage))

	if action == ia.Image_DELETE {
		is.setAction(name, ia.Image_DELETE)
		is.deleteImage(name, &ia.Image{})
		is.setTrigger(name, ContainerState_DELETED, is.clearAction)
		return
	}

	// from this point we need the cfgImage
	v, err = is.api.QueryGetValue(is.api.Self().String(), imgURL)
	if err != nil {
		is.api.Logf(types.LLERROR, "asked to perform a non-delete action on a non-existent image: %s : %s", name, action.String())
		return
	}
	cfgImage = v.(ia.Image)

	switch action {
	case ia.Image_CREATE:
		is.setAction(name, ia.Image_CREATE)
		is.createImage(name, &cfgImage)
	case ia.Image_RELOAD:
		is.setAction(name, ia.Image_RELOAD)
		is.reloadImage(name, &cfgImage)
	case ia.Image_NONE:
		return
	}
	is.setTrigger(name, cfgImage.Container.State, is.tSetACTIVE)
	is.setTrigger(name, cfgImage.Container.State, is.clearAction)
}

func (is *ImageAPI) mANYtoIDLE(me *core.MutationEvent) {
	// Stop & delete all Images
	is.setRunning()
	is.mutex.Lock()
	is.target = ia.ImageState_IDLE
	is.mutex.Unlock()
	v, _ := me.NodeCfg.GetValue(isURL)
	isc := v.Interface().(ia.ImageSet)
	v, _ = me.NodeDsc.GetValue(isURL)
	isd := v.Interface().(ia.ImageSet)
	for name, image := range isc.Images {
		if _, ok := isd.Images[name]; ok {
			// this container needs to be deleted
			is.setAction(name, ia.Image_DELETE)
			is.deleteImage(name, image)
			is.setTrigger(name, ContainerState_DELETED, is.tSetIDLE)
			is.setTrigger(name, ContainerState_DELETED, is.clearAction)
		}
	}
}

func (is *ImageAPI) mRecoverError(me *core.MutationEvent) bool {
	// logic for when an error is recoverable
	cleared := true
	changed := false
	is.mutex.Lock()
	v, _ := is.api.QueryGetValueDsc(is.api.Self().String(), isURL)
	isd := v.(ia.ImageSet)
	for name, image := range isd.Images {
		if image.State == ia.ImageState_ERROR {
			image.Retries++
			changed = true
			if image.Retries > is.cfg.MaxRetries {
				// too many retries
				cleared = false
				image.State = ia.ImageState_FATAL
				image.LastError = ia.Image_MAX_ATTEMPTS
				is.api.Logf(types.LLERROR, "image reached maximum retries (%d/%d): %s", image.Retries, is.cfg.MaxRetries, name)
			} else {
				image.Action = ia.Image_RELOAD
				is.api.Logf(types.LLERROR, "image is retrying (%d/%d): %s", image.Retries, is.cfg.MaxRetries, name)
			}
		}
	}
	if changed {
		if err := is.api.QuerySetValueDsc(is.api.Self().String(), isURL, isd); err != nil {
			is.api.Logf(types.LLERROR, "failed to set updated error value(s): %v", err)
		}
	}
	is.mutex.Unlock()
	if changed {
		is.updateSetState()
	}
	return cleared
}

func (is *ImageAPI) setValues(vs map[string]interface{}) {
	is.mutex.Lock()
	if _, err := is.api.QuerySetValuesDsc(is.api.Self().String(), vs); err != nil {
		is.api.Logf(types.LLERROR, "setvalues failed: %v: %v", err, vs)
	}
	is.mutex.Unlock()
	is.updateSetState()
}

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

// dsc changes mean something about the container(s) changed
func (is *ImageAPI) dscChange(name, sub string, value reflect.Value) {
	// Note: currently sub always == "" because map value changes represent a single change
	if sub == "/Action" {
		is.activateImage(name)
		return
	}
	if sub != "" && sub != "/State" && sub != "/Container/State" {
		is.api.Logf(types.LLDDDEBUG, "unhandled dsc change path: %s", sub)
	}
	is.api.Logf(types.LLDDDEBUG, "processing dsc change path: %s", sub)
	ctnURL := util.URLPush(util.URLPush(isURL, "Images"), name)
	// ok, we had a change in running state
	v, err := is.api.QueryGetValueDsc(is.api.Self().String(), ctnURL)
	if err != nil {
		// image was deleted
		is.fireTriggers(name, ContainerState_DELETED)
		is.updateSetState()
		return
	}
	idc := v.(ia.Image)

	if idc.Container == nil || idc.Container.State == "" { // deleted
		is.fireTriggers(name, ContainerState_DELETED)
		is.updateSetState()
		return
	}

	if idc.Container.State == ContainerState_DEAD {
		if idc.State != ia.ImageState_ERROR && idc.State != ia.ImageState_FATAL {
			is.imageRaiseError(name, ia.Image_DIED)
		}
		return
	}

	if is.fireTriggers(name, idc.Container.State) {
		is.updateSetState()
	}
	// Action may have been included in a whole-image change
	if sub == "" && idc.Action != is.getAction(name) {
		is.activateImage(name)
	}
}

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
	is.updateSetState()
}

////////////////////////
// Utility functions //
//////////////////////

// get a configured API client
func (is ImageAPI) getAPIClient() *api.Imageapi {
	as := is.cfg.ApiServer
	t := transport.New(fmt.Sprintf("%s:%d", as.Server, as.Port), as.ApiBase, []string{"http"})
	return api.New(t, strfmt.Default)
}

// not super efficient, but should work
func (is *ImageAPI) protoToAPI(p *ia.Container) *models.Container {
	b, err := json.Marshal(p)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container proto -> API: %v", err)
		return nil
	}
	r := &models.Container{}
	err = json.Unmarshal(b, r)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container proto -> API: %v", err)
		return nil
	}
	return r
}

// not super efficient, but should work
func (is *ImageAPI) apiToProto(a *models.Container) *ia.Container {
	b, err := json.Marshal(a)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container API -> proto: %v", err)
		return nil
	}
	r := &ia.Container{}
	kjson.Unmarshal(b, r)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container API -> proto: %v", err)
		return nil
	}
	return r
}

func (is *ImageAPI) imageRaiseError(name string, e ia.Image_ErrorCode) {
	url := util.URLPush(util.URLPush(isURL, "Images"), name)
	is.setValues(map[string]interface{}{
		util.URLPush(url, "State"):     ia.ImageState_ERROR,
		util.URLPush(url, "LastError"): e,
	})
}

func (is *ImageAPI) createImage(name string, image *ia.Image) {
	is.api.Logf(types.LLINFO, "creating image %s", name)
	// make sure naming is uniform.  The container must be named the same as the map.
	image.Container.Name = name
	client := is.getAPIClient()
	apiContainer := is.protoToAPI(image.Container)
	params := containers.NewCreateContainerParams()
	params.Container = apiContainer
	_, err := client.Containers.CreateContainer(params)
	if err != nil {
		if cerr, ok := err.(*containers.CreateContainerDefault); ok {
			is.api.Logf(types.LLERROR, "container creation failed for image %s: Code: %d Message: %s", name, cerr.Payload.Code, *cerr.Payload.Message)
			// "discover" our error state
		} else {
			is.api.Logf(types.LLERROR, "failed to call imageapi: %v", err)
		}
		is.imageRaiseError(name, ia.Image_ATTACH)
	}
}

func (is *ImageAPI) deleteImage(name string, image *ia.Image) {
	url := isURL
	for _, u := range []string{"Images", name, "Container", "State"} {
		url = util.URLPush(url, u)
	}
	v, err := is.api.QueryGetValueDsc(is.api.Self().String(), url)
	if err != nil {
		// image must not exist, so our work is done
		return
	}
	client := is.getAPIClient()
	switch v.(string) {
	case ContainerState_RUNNING:
		// tell the node to stop
		is.api.Logf(types.LLINFO, "stopping image %s", name)
		params := containers.NewSetContainerStateBynameParams()
		params.Name = name
		params.State = string(models.ContainerStateExited)
		_, err := client.Containers.SetContainerStateByname(params)
		if err != nil {
			if cerr, ok := err.(*containers.SetContainerStateBynameDefault); ok {
				is.api.Logf(types.LLERROR, "failed to stop container %s: Code: %d Message %s", name, cerr.Payload.Code, *cerr.Payload.Message)
			} else {
				is.api.Logf(types.LLERROR, "failed to call imageapi: %v", err)
			}
			// "discover" our error state
			is.imageRaiseError(name, ia.Image_ATTACH)
		}
		fallthrough
	case ContainerState_STOPPING:
		// set a trigger and get rerun when it's done
		is.api.Logf(types.LLINFO, "waiting for image %s to exit", name)
		is.setTrigger(name, ContainerState_EXITED, is.tDelete)
		return
	}
	// ok, we're ready to delete
	is.api.Logf(types.LLINFO, "deleting image %s", name)
	params := containers.NewDeleteContainerBynameParams()
	params.Name = name
	_, err = client.Containers.DeleteContainerByname(params)
	if err != nil {
		if cerr, ok := err.(*containers.DeleteContainerBynameDefault); ok {
			is.api.Logf(types.LLERROR, "failed to delete container %s: Code: %d Message %s", name, cerr.Payload.Code, *cerr.Payload.Message)
			// "discover" our error state
		} else {
			is.api.Logf(types.LLERROR, "failed to call imageapi: %v", err)
		}
		is.imageRaiseError(name, ia.Image_ATTACH)
	}
}

func (is *ImageAPI) reloadImage(name string, image *ia.Image) {
	is.api.Logf(types.LLINFO, "reloading image %s", name)
	is.setTrigger(name, ContainerState_DELETED, is.tCreate)
	is.deleteImage(name, image)
}

///////////////////////
// trigger handling //
/////////////////////

// Triggers are hooks that get called when a certain ContainerState is reached
// This allows for chaining of async actions
// All triggers take an Image name as argument

// we define an extra container state to recognize deleted containers
const (
	ContainerState_DELETED  = "DELETED"
	ContainerState_CREATED  = "created"
	ContainerState_RUNNING  = "running"
	ContainerState_STOPPING = "stopping"
	ContainerState_EXITED   = "exited"
	ContainerState_DEAD     = "dead"
)

func (is *ImageAPI) clearAllTriggers() {
	is.api.Logf(types.LLDDDEBUG, "clearing all triggers")
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.triggers = make(map[string]map[string][]trigger)
}

func (is *ImageAPI) clearTriggers(name string) {
	is.api.Logf(types.LLDDDEBUG, "clearing triggers for %s", name)
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.triggers[name] = make(map[string][]trigger)
}

func (is *ImageAPI) setTrigger(name, state string, t trigger) {
	is.api.Logf(types.LLDDDEBUG, "setting trigger %s for %s", state, name)
	is.mutex.Lock()
	defer is.mutex.Unlock()
	var ok bool
	var ts map[string][]trigger
	if ts, ok = is.triggers[name]; !ok {
		is.triggers[name] = make(map[string][]trigger)
		ts = is.triggers[name]
	}
	ts[state] = append(ts[state], t)
}

func (is *ImageAPI) fireTriggers(name, state string) bool {
	is.mutex.Lock()
	if ts, ok := is.triggers[name][state]; ok {
		if len(ts) == 0 {
			is.mutex.Unlock()
			return false
		}
		is.api.Logf(types.LLDDEBUG, "firing image %d triggers %s for %s", len(ts), state, name)
		delete(is.triggers[name], state)
		is.mutex.Unlock()
		for _, t := range ts {
			t(name)
		}
		return true
	}
	is.mutex.Unlock()
	return false
}

func (is *ImageAPI) tDelete(name string) {
	// we use an empty image
	time.Sleep(5 * time.Second) // FIXME: this is a hack to give rbd/fs time to flush.  This should be fixed in imageapi-server
	is.deleteImage(name, &ia.Image{})
}

func (is *ImageAPI) tCreate(name string) {
	iurl := util.URLPush(util.URLPush(isURL, "Images"), name)
	v, err := is.api.QueryGetValue(is.api.Self().String(), iurl)
	image := v.(ia.Image)
	if err != nil {
		// apparently there's nothing to create
		is.api.Logf(types.LLDEBUG, "triggered to create image that is no longer defined: %s", name)
		return
	}
	is.createImage(name, &image)
}

func (is *ImageAPI) tSetACTIVE(name string) {
	is.api.Logf(types.LLINFO, "image %s becomes ACTIVE", name)
	iurl := util.URLPush(util.URLPush(isURL, "Images"), name)
	is.setValues(map[string]interface{}{
		util.URLPush(iurl, "State"):   ia.ImageState_ACTIVE,
		util.URLPush(iurl, "Action"):  ia.Image_NONE,
		util.URLPush(iurl, "Retries"): int32(0),
	})
}

func (is *ImageAPI) tSetIDLE(name string) {
	is.api.Logf(types.LLINFO, "image %s becomes IDLE", name)
	iurl := util.URLPush(util.URLPush(isURL, "Images"), name)
	is.setValues(map[string]interface{}{
		util.URLPush(iurl, "State"):   ia.ImageState_IDLE,
		util.URLPush(iurl, "Action"):  ia.Image_CREATE,
		util.URLPush(iurl, "Retries"): int32(0),
	})
}

// action handling

func (is *ImageAPI) setRunning() {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.running = true
}

func (is *ImageAPI) isRunning() bool {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	return is.running
}

func (is *ImageAPI) clearRunning() {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.running = false
}

func (is *ImageAPI) setAction(name string, action ia.Image_Action) {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.actions[name] = action
}

// note: also a trigger
func (is *ImageAPI) clearAction(name string) {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	if _, ok := is.actions[name]; ok {
		delete(is.actions, name)
	}
}

func (is *ImageAPI) clearAllActions() {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.actions = make(map[string]ia.Image_Action)
}

func (is *ImageAPI) getAction(name string) ia.Image_Action {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	if a, ok := is.actions[name]; ok {
		return a
	}
	return ia.Image_NONE
}
