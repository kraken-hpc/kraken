/* vboxmanage.go: mutations for VirtualBox using the vboxmanage-rest-api
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. vboxmanage.proto

/*
 * This module will manipulate the PhysState state field.
 * It will be restricted to Platform = vbox.
 */

package vboxmanage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

const (
	VBMBase        string = "/vboxmanage"
	VBMStat        string = VBMBase + "/showvminfo"
	VBMOn          string = VBMBase + "/startvm"
	VBMOff         string = VBMBase + "/controlvm"
	PlatformString string = "vbox"
)

// vbmResponse is the VBM response structure
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

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Platform": reflect.ValueOf(PlatformString),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

////////////////////
// VBM Object /
//////////////////

// VBM provides a power on/off interface to the vboxmanage-rest-api interface
type VBM struct {
	api        types.ModuleAPIClient
	cfg        *Config
	mchan      <-chan types.Event
	dchan      chan<- types.Event
	pollTicker *time.Ticker
}

/*
 *types.Module
 */
var _ types.Module = (*VBM)(nil)

// Name returns the FQDN of the module
func (*VBM) Name() string { return "github.com/hpc/kraken/modules/vboxmanage" }

/*
 * types.ModuleWithConfig
 */
var _ types.ModuleWithConfig = (*VBM)(nil)

// NewConfig returns a fully initialized default config
func (*VBM) NewConfig() proto.Message {
	r := &Config{
		ServerUrl: "type.googleapis.com/VBox.VirtualMachine/ApiServer",
		NameUrl:   "type.googleapis.com/VBox.VirtualMachine/VmName",
		UuidUrl:   "type.googleapis.com/VBox.VirtualMachine/Uuid",
		Servers: map[string]*Server{
			"vbm": {
				Name: "vbm",
				Ip:   "vboxmanage.local",
				Port: 8269,
			},
		},
		PollingInterval: "30s",
	}
	return r
}

// UpdateConfig updates the running config
func (pp *VBM) UpdateConfig(cfg proto.Message) (e error) {
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
func (*VBM) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * types.ModuleWithMutations & types.ModuleWithDiscovery
 */
var _ types.ModuleWithMutations = (*VBM)(nil)
var _ types.ModuleWithDiscovery = (*VBM)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (pp *VBM) SetMutationChan(c <-chan types.Event) { pp.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (pp *VBM) SetDiscoveryChan(c chan<- types.Event) { pp.dchan = c }

/*
 * types.ModuleSelfService
 */
var _ types.ModuleSelfService = (*VBM)(nil)

// Entry is the module's executable entrypoint
func (pp *VBM) Entry() {
	url := util.NodeURLJoin(pp.api.Self().String(),
		util.URLPush(util.URLPush("/Services", "vboxmanage"), "State"))
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
func (pp *VBM) Init(api types.ModuleAPIClient) {
	pp.api = api
	pp.cfg = pp.NewConfig().(*Config)
}

// Stop should perform a graceful exit
func (pp *VBM) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (pp *VBM) handleMutation(m types.Event) {
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
		pp.api.Logf(types.LLERROR, "could not get NID and/or VBM Server for node: %s", me.NodeCfg.ID().String())
		return
	}
	name := vs[pp.cfg.GetNameUrl()].String()
	srv := vs[pp.cfg.GetServerUrl()].String()
	// mutation switch
	switch me.Type {
	case core.MutationEvent_MUTATE:
		switch me.Mutation[1] {
		case "UKtoOFF": // this just forces discovery
			go pp.vmDiscover(srv, name, me.NodeCfg.ID())
		case "OFFtoON":
			go pp.vmOn(srv, name, me.NodeCfg.ID())
		case "ONtoOFF":
			go pp.vmOff(srv, name, me.NodeCfg.ID())
		case "HANGtoOFF":
			go pp.vmOff(srv, name, me.NodeCfg.ID())
			break
		case "UKtoHANG": // we don't actually do this
			fallthrough
		default:
			pp.api.Logf(types.LLDEBUG, "unexpected event: %s", me.Mutation[1])
		}
		break
	case core.MutationEvent_INTERRUPT:
		// nothing to do
		break
	}
}

func (pp *VBM) vmDiscover(srvName, name string, id types.NodeID) {
	srv, ok := pp.cfg.Servers[srvName]
	if !ok {
		pp.api.Logf(types.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))

	url := "http://" + addr + VBMStat + "/" + name
	resp, e := http.Get(url)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error dialing api: %v", e)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		pp.api.Logf(types.LLERROR, "error dialing api: HTTP %v", resp.StatusCode)
		return
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error reading api response body: %v", e)
		return
	}
	var rs struct {
		Name  string
		Uuid  string
		Ram   string
		Vram  string
		State string
	}
	e = json.Unmarshal(body, &rs)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error unmarshaling json: %v", e)
		return
	}
	var vid string
	switch rs.State {
	case "paused":
		fallthrough
	case "powered off":
		vid = "POWER_OFF"
	case "running":
		vid = "POWER_ON"
	default:
		vid = "PHYS_UNKNOWN"
	}

	url = util.NodeURLJoin(id.String(), "/PhysState")
	v := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: vid,
		},
	)
	pp.dchan <- v
}

func (pp *VBM) vmOn(srvName, name string, id types.NodeID) {
	srv, ok := pp.cfg.Servers[srvName]
	if !ok {
		pp.api.Logf(types.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))

	url := "http://" + addr + VBMOn + "/" + name + "?type=headless"
	resp, e := http.Get(url)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error dialing api: %v", e)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		pp.api.Logf(types.LLERROR, "error dialing api: HTTP %v", resp.StatusCode)
		return
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error reading api response body: %v", e)
		return
	}
	/* example response
		 * {
	     *	"shell": {
	     *		"command": "/usr/local/bin/vboxmanage startvm node2 --type headless",
	     *		"directory": "/usr/local/lib/node_modules/vboxmanage-rest-api",
	     *		"exitCode": 0,
	     *		"output": "Waiting for VM \"node2\" to power on...\nVM \"node2\" has been successfully started.\n"
	  	 *		}
		 * }
	*/
	var rs struct {
		Shell struct {
			Command   string
			Directory string
			ExitCode  int
			Output    string
		}
	}
	e = json.Unmarshal(body, &rs)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error unmarshaling json: %v", e)
		return
	}
	if rs.Shell.ExitCode != 0 {
		pp.api.Logf(types.LLERROR, "vboxmanage command failed, exit code: %d, cmd: %s, out: %s", rs.Shell.ExitCode, rs.Shell.Command, rs.Shell.Output)
		return
	}
	url = util.NodeURLJoin(id.String(), "/PhysState")
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

func (pp *VBM) vmOff(srvName, name string, id types.NodeID) {
	srv, ok := pp.cfg.Servers[srvName]
	if !ok {
		pp.api.Logf(types.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))

	url := "http://" + addr + VBMOff + "/" + name + "/poweroff"
	resp, e := http.Get(url)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error dialing api: %v", e)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		pp.api.Logf(types.LLERROR, "error dialing api: HTTP %v", resp.StatusCode)
		return
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error reading api response body: %v", e)
		return
	}
	/* example response
	 * {
	 *  "shell": {
	 *    "command": "/usr/local/bin/vboxmanage controlvm route2 poweroff",
	 *    "directory": "/usr/local/lib/node_modules/vboxmanage-rest-api",
	 *    "exitCode": 0,
	 *    "output": "0%...10%...20%...30%...40%...50%...60%...70%...80%...90%...100%\n"
	 *  }
	 * }
	 */
	var rs struct {
		Shell struct {
			Command   string
			Directory string
			ExitCode  int
			Output    string
		}
	}
	e = json.Unmarshal(body, &rs)
	if e != nil {
		pp.api.Logf(types.LLERROR, "error unmarshaling json: %v", e)
		return
	}
	if rs.Shell.ExitCode != 0 {
		pp.api.Logf(types.LLERROR, "vboxmanage command failed, exit code: %d, cmd: %s, out: %s", rs.Shell.ExitCode, rs.Shell.Command, rs.Shell.Output)
		return
	}
	url = util.NodeURLJoin(id.String(), "/PhysState")
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
func (pp *VBM) discoverAll() {
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
			pp.api.Logf(types.LLDEBUG, "skipping node %s, doesn't have complete VBM info", n.ID().String())
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

	// This is not very efficient, but we assume that this module won't be used for huge amounts of vms
	for s, ns := range bySrv {
		for _, n := range ns {
			pp.vmDiscover(s, n, idmap[n])
		}
	}
}

// initialization
func init() {
	module := &VBM{}
	mutations := make(map[string]types.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	drstate := make(map[string]reflect.Value)
	si := core.NewServiceInstance("vboxmanage", module.Name(), module.Entry)

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
	discovers["/Services/vboxmanage/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
