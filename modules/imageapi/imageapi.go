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
	"os"
	"reflect"
	"regexp"
	"time"

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
		ia.ImageState_ERROR.String(): reflect.ValueOf(ia.ImageState_ERROR),
		ia.ImageState_FATAL.String(): reflect.ValueOf(ia.ImageState_FATAL),
	},
	siURL: {
		cpb.ServiceInstance_RUN.String(): reflect.ValueOf(cpb.ServiceInstance_RUN),
	},
}

// ismut helps us succinctly define our mutations
type ismut struct {
	f       ia.ImageState // from
	t       ia.ImageState // to
	timeout string        // timeout
	failto  [3]string
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
	// we can mutate out of error (but not fatal)
	"ERRORtoRUN": {
		f:       ia.ImageState_ERROR,
		t:       ia.ImageState_UPDATE,
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

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/RunState": reflect.ValueOf(cpb.Node_SYNC),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

////////////////////////////
// Module Initialization //
//////////////////////////

func init() {
	mutations := make(map[string]types.StateMutation)
	// create mutation/discovers from our muts
	for m := range muts {
		dur, _ := time.ParseDuration(muts[m].timeout)
		mutations[m] = core.NewStateMutation(
			map[string][2]reflect.Value{
				issURL: {
					reflect.ValueOf(muts[m].f),
					reflect.ValueOf(muts[m].t),
				},
			},
			reqs,
			excs,
			types.StateMutationContext_SELF,
			dur,
			muts[m].failto,
		)
		discovers[issURL][muts[m].t.String()] = reflect.ValueOf(muts[m].t)
	}

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

// ImageAPI provides layer1 image loading capabilities
type ImageAPI struct {
	api        types.ModuleAPIClient
	cfg        *Config
	mchan      <-chan types.Event
	dchan      chan<- types.Event
	echan      <-chan types.Event
	pollTicker *time.Ticker
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
}

// Stop should perform a graceful exit
func (is *ImageAPI) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (is *ImageAPI) handleMutation(m types.Event) {
	if m.Type() != types.Event_STATE_MUTATION {
		is.api.Log(types.LLINFO, "got an unexpected event type on mutation channel")
	}
	me := m.Data().(*core.MutationEvent)

	// mutation switch
	switch me.Type {
	case core.MutationEvent_MUTATE:
		if mut, ok := muts[me.Mutation[1]]; ok {
			// this is a mutation we known, call its handler
			mut.h(me)
		} else {
			is.api.Logf(types.LLERROR, "asked to perform unknown mutation: %s", me.Mutation[1])
		}
		break
	case core.MutationEvent_INTERRUPT:
		// nothing to do
		break
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
	cfg, _ := is.api.QueryRead(is.api.Self().String())
	dsc, _ := is.api.QueryReadDsc(is.api.Self().String())
	switch sce.Type {
	case cpb.StateChangeControl_CFG_UPDATE:
		// container configuration changed
		is.cfgChange(name, sub, cfg, dsc)
	case cpb.StateChangeControl_UPDATE:
		// container state changed
		is.dscChange(name, sub, cfg, dsc, sce.Value)
	}
}

// cfg changes general mark discover nodes as needing some kind of update
func (is *ImageAPI) cfgChange(name, sub string, cfg, dsc types.Node) {
	ctnURL := util.URLPush(util.URLPush(isURL, "Images"), name)
	if _, err := cfg.GetValue(ctnURL); err != nil {
		// this Image is no longer configured.  It should be marked for deletion
		if _, err := dsc.GetValue(ctnURL); err != nil {
			// it seems to have already been deleted.  We're done.
			return
		}
		// mark node for update/delete
		dsc.SetValues(map[string]reflect.Value{
			util.URLPush(ctnURL, "Action"): reflect.ValueOf(ia.Image_DELETE),
			util.URLPush(ctnURL, "State"):  reflect.ValueOf(ia.ImageState_UPDATE),
		})
	} else {
		// this Image needs to be updated/reloaded
		dsc.SetValues(map[string]reflect.Value{
			util.URLPush(ctnURL, "Action"): reflect.ValueOf(ia.Image_DELETE),
			util.URLPush(ctnURL, "State"):  reflect.ValueOf(ia.ImageState_UPDATE),
		})
	}
	is.api.QueryUpdateDsc(dsc)
	// discover the imagestate update
	is.dchan <- core.NewEvent(
		types.Event_DISCOVERY,
		util.NodeURLJoin(is.api.Self().String(), issURL),
		&core.DiscoveryEvent{
			URL:     util.NodeURLJoin(is.api.Self().String(), issURL),
			ValueID: ia.ImageState_UPDATE.String(),
		},
	)
}

// dsc changes mean something about the container(s) changed
func (is *ImageAPI) dscChange(name, sub string, cfg, dsc types.Node, value reflect.Value) {
	checkCompletion := false
	ctnURL := util.URLPush(util.URLPush(isURL, "Images"), name)
	switch sub {
	case "": // might be a deletion
		if _, err := dsc.GetValue(ctnURL); err != nil {
			checkCompletion = true
		}
	case "/Container/State":
		// ok, we had a change in running state
		v, err := dsc.GetValue(ctnURL)
		if err != nil {
			// shouldn't really happen, but be safe
			return
		}
		switch state {
		case ia.ContainerState_CREATED:
		case ia.ContainerState_RUNNING:
			// this probably means we achieved our update/start/whatever

		case ia.ContainerState_STOPPING:
		case ia.ContainerState_EXITED:
		case ia.ContainerState_DEAD:
		}
	}
	if checkCompletion {

	}
}

// get a configured API client
func (is ImageAPI) getAPIClient(as ApiServer) *api.Imageapi {
	t := transport.New(fmt.Sprintf("%s:%s", as.Server, as.Port), "", nil)
	t.BasePath = as.ApiBase
	return api.New(t, strfmt.Default)
}

// not super efficient, but should work
func (is *ImageAPI) protoToAPI(p *ia.Container) *models.Container {
	b, err := kjson.Marshal(p)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container proto -> API: %v", err)
		return nil
	}
	r := &models.Container{}
	err := json.Unmarshal(b)
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

// discover's job is to periodically poll the imageapi for current data and update discoverable state
// discover does not make decisions on ImageState
// ironically, discover uses SetValue + Update instead of discover, since it updates a complex data structure
func (is *ImageAPI) discover() {
	cfg, _ := is.api.QueryRead(is.api.Self().String())
	dsc, _ := is.api.QueryReadDsc(is.api.Self().String())
	iscv, _ := cfg.GetValue(isURL)
	isc := iscv.Interface().(ia.ImageSet)
	isdv, _ := dsc.GetValue(isURL)
	isd := isdv.Interface().(ia.ImageSet)
	client := is.getAPIClient()
	resp, err := client.Containers.ListContainers(&containers.ListContainersParams{})
	if err != nil {
		// set error stuff
	}
	ctnsList := resp.GetPayload()

	// dset is where we will build the ImageSet that we will discover for the node
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
			delete(isd.Images, cp.Name) // delete it so we know what we *didn't* find later
		} else {
			// this is a new one
			dset.Images[cp.Name] = &ia.Image{
				Container: cp,
			}
		}
	}
	// we now have a list of everything imageapi knows about...
	// we also want at least a stub for anything in the cfg set
	for _, cp := range isc.Images {
		if _, ok := dset.Images[cp.Container.Name]; !ok {
			// we don't have an entry, does dsc?
			if ds, ok := isd.Images[cp.Container.Name]; ok {
				dset.Images[cp.Container.Name] = ds
			} else {
				// nope, make a stub
				dset.Images[cp.Container.Name] = &ia.Image{
					Container: &ia.Container{
						Name: cp.Container.Name,
					},
				}
			}
		}
	}

	// ok, we're ready to ship it
	// we do a query instead of a discover because we're setting various values
	dsc.SetValue(isURL, reflect.ValueOf(*dset))
	is.api.QueryUpdateDsc(dsc)
}

/*
func (is *ImageAPI) nodeOn(srvName, name string, id types.NodeID) {
	srv, ok := is.cfg.Servers[srvName]
	if !ok {
		is.api.Logf(types.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	client, ctx := clientContext(srv)

	body := api.NewResetRequestBody(api.RESETTYPE_FORCE_ON)
	_, r, e := client.DefaultApi.ComputerSystemsNameActionsComputerSystemResetPost(ctx, name).ResetRequestBody(*body).Execute()
	if is.apiError(r, e) {
		return
	}

	url := util.NodeURLJoin(id.String(), "/PhysState")
	v := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "POWER_ON",
		},
	)
	is.dchan <- v
}

func (is *ImageAPI) nodeOff(srvName, name string, id types.NodeID) {
	srv, ok := is.cfg.Servers[srvName]
	if !ok {
		is.api.Logf(types.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	client, ctx := clientContext(srv)

	body := api.NewResetRequestBody(api.RESETTYPE_FORCE_OFF)
	_, r, e := client.DefaultApi.ComputerSystemsNameActionsComputerSystemResetPost(ctx, name).ResetRequestBody(*body).Execute()
	if is.apiError(r, e) {
		return
	}

	url := util.NodeURLJoin(id.String(), "/PhysState")
	v := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "POWER_OFF",
		},
	)
	is.dchan <- v
}

// discoverAll is used to do polling discovery of power state
// Note: this is probably not extremely efficient for large systems
func (is *ImageAPI) discoverAll() {
	is.api.Log(types.LLDEBUG, "polling for node state")
	ns, e := is.api.QueryReadAll()
	if e != nil {
		is.api.Logf(types.LLERROR, "polling node query failed: %v", e)
		return
	}
	idmap := make(map[string]types.NodeID)
	bySrv := make(map[string][]string)

	// build lists
	for _, n := range ns {
		vs, e := n.GetValues([]string{"/Platform", is.cfg.GetNameUrl(), is.cfg.GetServerUrl()})
		if e != nil {
			is.api.Logf(types.LLERROR, "error getting values for node: %v", e)
		}
		if len(vs) != 3 {
			is.api.Logf(types.LLDEBUG, "skiising node %s, doesn't have complete ImageAPI info", n.ID().String())
			continue
		}
		if vs["/Platform"].String() != PlatformString { // Note: this may need to be more flexible in the future
			continue
		}
		name := vs[is.cfg.GetNameUrl()].String()
		srv := vs[is.cfg.GetServerUrl()].String()
		idmap[name] = n.ID()
		bySrv[srv] = aisend(bySrv[srv], name)
	}

	// This is not very efficient
	for s, ns := range bySrv {
		for _, n := range ns {
			is.nodeDiscover(s, n, idmap[n])
		}
	}
}
*/
