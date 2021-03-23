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
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	proto "github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/kraken-hpc/kraken/core"
	cpb "github.com/kraken-hpc/kraken/core/proto"
	ia "github.com/kraken-hpc/kraken/extensions/imageapi"
	"github.com/kraken-hpc/kraken/lib/types"
	"github.com/kraken-hpc/kraken/lib/util"
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
	"/Busy": { // we don't actually discover these, but we need them to be "Discoverable"
		cpb.Node_BUSY.String(): reflect.ValueOf(cpb.Node_BUSY),
		cpb.Node_FREE.String(): reflect.ValueOf(cpb.Node_FREE),
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
func (*ImageAPI) Name() string { return "github.com/kraken-hpc/kraken/modules/imageapi" }

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
	// clear any state discoverable info about images
	is.api.QuerySetValueDsc(is.api.Self().String(), isURL, ia.ImageSet{
		Images: map[string]*ia.Image{},
	})
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
