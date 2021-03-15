/* powerapi.go: mutations for PowerAPI
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. powerapi-config.proto

/*
 * This module will manipulate the PhysState state field.
 * It will be restricted to Platform = powerapi.
 */

package powerapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"time"

	proto "github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
	api "github.com/hpc/powerapi/pkg/powerapi-client"
)

const (
	PlatformString string = "powerapi"
)

// vbmResponse is the PowerAPI response structure
type vbmResponse struct {
	Err    int32    `json:"e,omitempty"`
	ErrMsg string   `json:"err_msg,omitempty"`
	Off    []uint32 `json:"off,omitempty"`
	On     []uint32 `json:"on,omitempty"`
}

// ppmut helps us succinctly define our mutations
type ppmut struct {
	f       cpb.Node_PhysState // from
	t       cpb.Node_PhysState // to
	timeout string             // timeout
	reqs    map[string]reflect.Value
	excs    map[string]reflect.Value
	// everything fails to PHYS_HANG
}

// our mutation definitions
// also we discover anything we can migrate to
var muts = map[string]ppmut{
	"UKtoOFF": {
		f:       cpb.Node_PHYS_UNKNOWN,
		t:       cpb.Node_POWER_OFF,
		timeout: "10s",
	},
	"OFFtoON": {
		f:       cpb.Node_POWER_OFF,
		t:       cpb.Node_POWER_ON,
		timeout: "10s",
	},
	"ONtoOFF": {
		f:       cpb.Node_POWER_ON,
		t:       cpb.Node_POWER_OFF,
		timeout: "10s",
		excs: map[string]reflect.Value{
			"/RunState": reflect.ValueOf(cpb.Node_SYNC),
			"/Busy":     reflect.ValueOf(cpb.Node_BUSY),
		},
	},
	"HANGtoOFF": {
		f:       cpb.Node_PHYS_HANG,
		t:       cpb.Node_POWER_OFF,
		timeout: "20s", // we need a longer timeout, because we let it sit cold for a few seconds
	},
}

// httpProto maps flag.https to the index of http/https
var httpProto = map[bool]int{
	false: 1,
	true:  0,
}

// powerToPhys maps the power state to the phys state
var powerToPhys = map[api.PowerState]string{
	api.POWERSTATE_ON:  cpb.Node_POWER_ON.String(),
	api.POWERSTATE_OFF: cpb.Node_POWER_OFF.String(),
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Platform": reflect.ValueOf(PlatformString),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

/////////////////////
// PowerAPI Object /
///////////////////

// PowerAPI provides a power on/off interface
type PowerAPI struct {
	api        types.ModuleAPIClient
	cfg        *Config
	mchan      <-chan types.Event
	dchan      chan<- types.Event
	pollTicker *time.Ticker
}

/*
 *types.Module
 */
var _ types.Module = (*PowerAPI)(nil)

// Name returns the FQDN of the module
func (*PowerAPI) Name() string { return "github.com/hpc/kraken/modules/powerapi" }

/*
 * types.ModuleWithConfig
 */
var _ types.ModuleWithConfig = (*PowerAPI)(nil)

// NewConfig returns a fully initialized default config
func (*PowerAPI) NewConfig() proto.Message {
	r := &Config{
		ServerUrl: "type.googleapis.com/PowerAPI.Node/ApiServer",
		NameUrl:   "type.googleapis.com/PowerAPI.Node/NodeName",
		UriUrl:    "type.googleapis.com/PowerAPI.Node/NodeUri",
		Servers: map[string]*Server{
			"powerapi": {
				Server:  "localhost",
				Port:    8269,
				Https:   false,
				ApiBase: "/power/v1",
			},
		},
		PollingInterval: "30s",
	}
	return r
}

// UpdateConfig updates the running config
func (pp *PowerAPI) UpdateConfig(cfg proto.Message) (e error) {
	if ppcfg, ok := cfg.(*Config); ok {
		pp.cfg = ppcfg
		if pp.pollTicker != nil {
			pp.pollTicker.Stop()
			dur, _ := time.ParseDuration(pp.cfg.GetPollingInterval())
			pp.pollTicker = time.NewTicker(dur)
		}
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*PowerAPI) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * types.ModuleWithMutations & types.ModuleWithDiscovery
 */
var _ types.ModuleWithMutations = (*PowerAPI)(nil)
var _ types.ModuleWithDiscovery = (*PowerAPI)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (pp *PowerAPI) SetMutationChan(c <-chan types.Event) { pp.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (pp *PowerAPI) SetDiscoveryChan(c chan<- types.Event) { pp.dchan = c }

/*
 * types.ModuleSelfService
 */
var _ types.ModuleSelfService = (*PowerAPI)(nil)

// Entry is the module's executable entrypoint
func (pp *PowerAPI) Entry() {
	url := util.NodeURLJoin(pp.api.Self().String(),
		util.URLPush(util.URLPush("/Services", "powerapi"), "State"))
	pp.dchan <- core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)
	// setup a ticker for polling discovery
	dur, _ := time.ParseDuration(pp.cfg.GetPollingInterval())
	pp.pollTicker = time.NewTicker(dur)

	// main loop
	for {

		select {
		case <-pp.pollTicker.C:
			go pp.discoverAll()
			break
		case m := <-pp.mchan: // mutation request
			go pp.handleMutation(m)
			break
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (pp *PowerAPI) Init(api types.ModuleAPIClient) {
	pp.api = api
	pp.cfg = pp.NewConfig().(*Config)
}

// Stop should perform a graceful exit
func (pp *PowerAPI) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (pp *PowerAPI) handleMutation(m types.Event) {
	if m.Type() != types.Event_STATE_MUTATION {
		pp.api.Log(types.LLINFO, "got an unexpected event type on mutation channel")
	}
	me := m.Data().(*core.MutationEvent)
	// extract the mutating node's name and server
	vs, e := me.NodeCfg.GetValues([]string{pp.cfg.GetNameUrl(), pp.cfg.GetServerUrl()})
	if e != nil {
		pp.api.Logf(types.LLERROR, "error getting values for node: %v", e)
	}
	if len(vs) != 2 {
		pp.api.Logf(types.LLERROR, "could not get NID and/or PowerAPI Server for node: %s", me.NodeCfg.ID().String())
		return
	}
	name := vs[pp.cfg.GetNameUrl()].String()
	srv := vs[pp.cfg.GetServerUrl()].String()
	// mutation switch
	switch me.Type {
	case core.MutationEvent_MUTATE:
		switch me.Mutation[1] {
		case "UKtoOFF": // this just forces discovery
			go pp.nodeDiscover(srv, name, me.NodeCfg.ID())
		case "OFFtoON":
			go pp.nodeOn(srv, name, me.NodeCfg.ID())
		case "ONtoOFF":
			go pp.nodeOff(srv, name, me.NodeCfg.ID())
		case "HANGtoOFF":
			go pp.nodeOff(srv, name, me.NodeCfg.ID())
			break
		default:
			pp.api.Logf(types.LLDEBUG, "unexpected event: %s", me.Mutation[1])
		}
		break
	case core.MutationEvent_INTERRUPT:
		// nothing to do
		break
	}
}

func clientContext(srv *Server) (*api.APIClient, context.Context) {
	client := api.NewAPIClient(api.NewConfiguration())
	ctx := context.Background()
	ctx = context.WithValue(ctx, api.ContextServerIndex, httpProto[srv.Https])
	server := "localhost"
	apiBase := "/power/v1"
	port := int32(8269)
	if srv.Server != "" {
		server = srv.Server
	}
	if srv.ApiBase != "" {
		apiBase = srv.ApiBase
	}
	if srv.Port != 0 {
		port = srv.Port
	}
	ctx = context.WithValue(ctx, api.ContextServerVariables, map[string]string{
		"server":  fmt.Sprintf("%s:%d", server, port),
		"apiBase": apiBase,
	})
	return client, ctx
}

func (pp *PowerAPI) apiError(r *http.Response, e error) bool {
	if e == nil {
		return false
	}
	pp.api.Logf(types.LLERROR, "api call failed: err(%v) response(%v)", e, r)
	return true
}

func (pp *PowerAPI) nodeDiscover(srvName, name string, id types.NodeID) {
	srv, ok := pp.cfg.Servers[srvName]
	if !ok {
		pp.api.Logf(types.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	client, ctx := clientContext(srv)

	cs, r, e := client.DefaultApi.ComputerSystemsNameGet(ctx, name).Execute()
	if pp.apiError(r, e) {
		return
	}

	url := util.NodeURLJoin(id.String(), "/PhysState")
	v := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: powerToPhys[*cs.PowerState],
		},
	)
	pp.dchan <- v
}

func (pp *PowerAPI) nodeOn(srvName, name string, id types.NodeID) {
	srv, ok := pp.cfg.Servers[srvName]
	if !ok {
		pp.api.Logf(types.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	client, ctx := clientContext(srv)

	body := api.NewResetRequestBody(api.RESETTYPE_FORCE_ON)
	_, r, e := client.DefaultApi.ComputerSystemsNameActionsComputerSystemResetPost(ctx, name).ResetRequestBody(*body).Execute()
	if pp.apiError(r, e) {
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
	pp.dchan <- v
}

func (pp *PowerAPI) nodeOff(srvName, name string, id types.NodeID) {
	srv, ok := pp.cfg.Servers[srvName]
	if !ok {
		pp.api.Logf(types.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	client, ctx := clientContext(srv)

	body := api.NewResetRequestBody(api.RESETTYPE_FORCE_OFF)
	_, r, e := client.DefaultApi.ComputerSystemsNameActionsComputerSystemResetPost(ctx, name).ResetRequestBody(*body).Execute()
	if pp.apiError(r, e) {
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
	pp.dchan <- v
}

// discoverAll is used to do polling discovery of power state
// Note: this is probably not extremely efficient for large systems
func (pp *PowerAPI) discoverAll() {
	pp.api.Log(types.LLDEBUG, "polling for node state")
	ns, e := pp.api.QueryReadAll()
	if e != nil {
		pp.api.Logf(types.LLERROR, "polling node query failed: %v", e)
		return
	}
	idmap := make(map[string]types.NodeID)
	bySrv := make(map[string][]string)

	// build lists
	for _, n := range ns {
		vs, e := n.GetValues([]string{"/Platform", pp.cfg.GetNameUrl(), pp.cfg.GetServerUrl()})
		if e != nil {
			pp.api.Logf(types.LLERROR, "error getting values for node: %v", e)
		}
		if len(vs) != 3 {
			pp.api.Logf(types.LLDEBUG, "skipping node %s, doesn't have complete PowerAPI info", n.ID().String())
			continue
		}
		if vs["/Platform"].String() != PlatformString { // Note: this may need to be more flexible in the future
			continue
		}
		name := vs[pp.cfg.GetNameUrl()].String()
		srv := vs[pp.cfg.GetServerUrl()].String()
		idmap[name] = n.ID()
		bySrv[srv] = append(bySrv[srv], name)
	}

	// This is not very efficient
	for s, ns := range bySrv {
		for _, n := range ns {
			pp.nodeDiscover(s, n, idmap[n])
		}
	}
}

// initialization
func init() {
	module := &PowerAPI{}
	mutations := make(map[string]types.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	drstate := make(map[string]reflect.Value)
	si := core.NewServiceInstance("powerapi", module.Name(), module.Entry)

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
				"/PhysState": {
					reflect.ValueOf(muts[m].f),
					reflect.ValueOf(muts[m].t),
				},
			},
			mut.reqs,
			mut.excs,
			types.StateMutationContext_CHILD,
			dur,
			[3]string{si.ID(), "/PhysState", "PHYS_HANG"},
		)
		drstate[cpb.Node_PhysState_name[int32(muts[m].t)]] = reflect.ValueOf(muts[m].t)
	}
	discovers["/PhysState"] = drstate
	discovers["/PhysState"]["PHYS_UNKNOWN"] = reflect.ValueOf(cpb.Node_PHYS_UNKNOWN)
	discovers["/PhysState"]["PHYS_HANG"] = reflect.ValueOf(cpb.Node_PHYS_HANG)
	discovers["/RunState"] = map[string]reflect.Value{
		"RUN_UK": reflect.ValueOf(cpb.Node_UNKNOWN),
	}
	discovers["/Services/powerapi/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
