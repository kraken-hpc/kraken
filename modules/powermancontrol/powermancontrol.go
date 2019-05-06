/* powermancontrol.go: used to control power on nodes via powerman
 *
 * Author: R. Eli Snyder <resnyder@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/powermancontrol.proto

/*
 * This module will manipulate the PhysState state field.
 * It will be restricted to Platform = powerman
 */

package powermancontrol

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/powermancontrol/proto"
)

const (
	PMCBase        string = "/powermancontrol"
	PMCOn          string = PMCBase + "/poweron"
	PMCOff         string = PMCBase + "/poweroff"
	PMCStat        string = PMCBase + "/nodeStatus"
	PlatformString string = "powerman"
)

// ppmut helps us succinctly define our mutations
type ppmut struct {
	f       cpb.Node_PhysState // from
	t       cpb.Node_PhysState // to
	timeout string             // timeout
	// everything fails to PHYS_HANG
}

// our mutation definitions
// also we discover anything we can migrate to
var muts = map[string]ppmut{
	"UKtoOFF": ppmut{
		f:       cpb.Node_PHYS_UNKNOWN,
		t:       cpb.Node_POWER_OFF,
		timeout: "10s",
	},
	"OFFtoON": ppmut{
		f:       cpb.Node_POWER_OFF,
		t:       cpb.Node_POWER_ON,
		timeout: "10s",
	},
	"ONtoOFF": ppmut{
		f:       cpb.Node_POWER_ON,
		t:       cpb.Node_POWER_OFF,
		timeout: "10s",
	},
	"HANGtoOFF": ppmut{
		f:       cpb.Node_PHYS_HANG,
		t:       cpb.Node_POWER_OFF,
		timeout: "20s", // we need a longer timeout, because we let it sit cold for a few seconds
	},
	"UKtoHANG": ppmut{ // this one should never happen; just making sure HANG gets connected in our graph
		f:       cpb.Node_PHYS_UNKNOWN,
		t:       cpb.Node_PHYS_HANG,
		timeout: "0s",
	},
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Platform": reflect.ValueOf(PlatformString),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

////////////////////
// PMC Object /
//////////////////

// PMC provides a power on/off interface to powerman
type PMC struct {
	api        lib.APIClient
	cfg        *pb.PMCConfig
	mchan      <-chan lib.Event
	dchan      chan<- lib.Event
	pollTicker *time.Ticker
}

/*
 *lib.Module
 */
var _ lib.Module = (*PMC)(nil)

// Name returns the FQDN of the module
func (p *PMC) Name() string { return "github.com/hpc/kraken/modules/powermancontrol" }

/*
 * lib.ModuleWithConfig
 */
var _ lib.ModuleWithConfig = (*PMC)(nil)

// NewConfig returns a fully initialized default config
func (p *PMC) NewConfig() proto.Message {
	r := &pb.PMCConfig{
		ServerUrl: "type.googleapis.com/proto.Powerman/ApiServer",
		NameUrl:   "type.googleapis.com/proto.Powerman/Name",
		Servers: map[string]*pb.PMCServer{
			"pmc": {
				Name: "pmc",
				Ip:   "localhost",
				Port: 8269,
			},
		},
		PollingInterval: "30s",
	}
	return r
}

// UpdateConfig updates the running config
func (p *PMC) UpdateConfig(cfg proto.Message) (e error) {
	if pcfg, ok := cfg.(*pb.PMCConfig); ok {
		p.cfg = pcfg
		if p.pollTicker != nil {
			p.pollTicker.Stop()
			dur, _ := time.ParseDuration(p.cfg.GetPollingInterval())
			p.pollTicker = time.NewTicker(dur)
			return
		}
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*PMC) ConfigURL() string {
	cfg := &pb.PMCConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * lib.ModuleWithMutations & lib.ModuleWithDiscovery
 */
var _ lib.ModuleWithMutations = (*PMC)(nil)
var _ lib.ModuleWithDiscovery = (*PMC)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (p *PMC) SetMutationChan(c <-chan lib.Event) { p.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (p *PMC) SetDiscoveryChan(c chan<- lib.Event) { p.dchan = c }

/*
 * lib.ModuleSelfService
 */
var _ lib.ModuleSelfService = (*PMC)(nil)

// Entry is the module's executable entrypoint
func (p *PMC) Entry() {
	url := lib.NodeURLJoin(p.api.Self().String(),
		lib.URLPush(lib.URLPush("/Services", "powermancontrol"), "State"))
	p.dchan <- core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  p.Name(),
			URL:     url,
			ValueID: "RUN",
		},
	)

	dur, _ := time.ParseDuration(p.cfg.GetPollingInterval())
	p.pollTicker = time.NewTicker(dur)

	for {
		select {
		case <-p.pollTicker.C:
			go p.discoverAll()
			break
		case m := <-p.mchan:
			if m.Type() != lib.Event_STATE_MUTATION {
				p.api.Log(lib.LLERROR, "got unexpected non-mutation event")
				break
			}
			go p.handleMutation(m)
			break
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (p *PMC) Init(api lib.APIClient) {
	p.api = api
	p.cfg = p.NewConfig().(*pb.PMCConfig)
}

// Stop should perform a graceful exit
func (p *PMC) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (p *PMC) handleMutation(m lib.Event) {
	if m.Type() != lib.Event_STATE_MUTATION {
		p.api.Log(lib.LLINFO, "got an unexpected event type on mutation channel")
	}

	me := m.Data().(*core.MutationEvent)
	// extract the mutating node's name and server
	vs := me.NodeCfg.GetValues([]string{p.cfg.GetNameUrl(), p.cfg.GetServerUrl()})
	if len(vs) != 2 {
		p.api.Logf(lib.LLERROR, "could not get NID and/or PMC Server for node: %s", me.NodeCfg.ID().String())
		return
	}
	name := vs[p.cfg.GetNameUrl()].String()
	srv := vs[p.cfg.GetServerUrl()].String()

	// mutation switch
	switch me.Type {
	case core.MutationEvent_MUTATE:
		switch me.Mutation[1] {
		case "UKtoOFF": // this just forces discover
			go p.nodeDiscover(srv, name, me.NodeCfg.ID())
			break
		case "OFFtoON":
			go p.powerOn(srv, name, me.NodeCfg.ID())
			break
		case "ONtoOFF":
			go p.powerOff(srv, name, me.NodeCfg.ID())
			break
		case "HANGtoOFF":
			go p.powerOff(srv, name, me.NodeCfg.ID())
			break
		case "UKtoHANG": // we don't actually do this
			fallthrough
		default:
			p.api.Logf(lib.LLDEBUG, "unexpected event: %s", me.Mutation[1])
		}
		break
	case core.MutationEvent_INTERRUPT:
		// nothing to do
		break
	}
}

func (p *PMC) nodeDiscover(srvName, name string, id lib.NodeID) {
	srv, ok := p.cfg.Servers[srvName]
	if !ok {
		p.api.Logf(lib.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))

	url := "http://" + addr + PMCStat + "/" + name
	resp, e := http.Get(url)
	if e != nil {
		p.api.Logf(lib.LLERROR, "error dialing api: %v", e)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		p.api.Logf(lib.LLERROR, "error dialing api: HTTP %v", resp.StatusCode)
		return
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		p.api.Logf(lib.LLERROR, "error reading api response body: %v", e)
		return
	}

	var rs struct {
		State string
	}

	e = json.Unmarshal(body, &rs)
	if e != nil {
		p.api.Logf(lib.LLERROR, "error unmarshaling json: %v", e)
		return
	}
	url = lib.NodeURLJoin(id.String(), "/PhysState")
	v := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  p.Name(),
			URL:     url,
			ValueID: rs.State,
		},
	)
	p.dchan <- v
}

func (p *PMC) powerOn(srvName, name string, id lib.NodeID) {
	srv, ok := p.cfg.Servers[srvName]
	if !ok {
		p.api.Logf(lib.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))

	url := "http://" + addr + PMCOn + "/" + name
	resp, e := http.Get(url)
	if e != nil {
		p.api.Logf(lib.LLERROR, "error dialing api: %v", e)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		p.api.Logf(lib.LLERROR, "error dialing api: HTTP %v", resp.StatusCode)
		return
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		p.api.Logf(lib.LLERROR, "error reading api response body: %v", e)
		return
	}

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
		p.api.Logf(lib.LLERROR, "error unmarshaling json: %v", e)
		return
	}
	if rs.Shell.ExitCode != 0 {
		p.api.Logf(lib.LLERROR, "powermancontrol command failed, exit code: %d, cmd: %s, out: %s", rs.Shell.ExitCode, rs.Shell.Command, rs.Shell.Output)
		return
	}
	url = lib.NodeURLJoin(id.String(), "/PhysState")
	v := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  p.Name(),
			URL:     url,
			ValueID: "POWER_ON",
		},
	)
	p.dchan <- v
}

func (p *PMC) powerOff(srvName, name string, id lib.NodeID) {
	srv, ok := p.cfg.Servers[srvName]
	if !ok {
		p.api.Logf(lib.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))

	url := "http://" + addr + PMCOff + "/" + name
	resp, e := http.Get(url)
	if e != nil {
		p.api.Logf(lib.LLERROR, "error dialing api: %v", e)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		p.api.Logf(lib.LLERROR, "error dialing api: HTTP %v", resp.StatusCode)
		return
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		p.api.Logf(lib.LLERROR, "error reading api response body: %v", e)
		return
	}

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
		p.api.Logf(lib.LLERROR, "error unmarshaling json: %v", e)
		return
	}
	if rs.Shell.ExitCode != 0 {
		p.api.Logf(lib.LLERROR, "powermancontrol command failed, exit code: %d, cmd: %s, out: %s", rs.Shell.ExitCode, rs.Shell.Command, rs.Shell.Output)
		return
	}
	url = lib.NodeURLJoin(id.String(), "/PhysState")
	v := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  p.Name(),
			URL:     url,
			ValueID: "POWER_OFF",
		},
	)
	p.dchan <- v
}

func (p *PMC) discoverAll() {
	p.api.Log(lib.LLDEBUG, "polling for node state")
	ns, e := p.api.QueryReadAll()
	if e != nil {
		p.api.Logf(lib.LLERROR, "polling node query failed: %v", e)
		return
	}
	idmap := make(map[string]lib.NodeID)
	bySrv := make(map[string][]string)

	// build lists
	for _, n := range ns {
		vs := n.GetValues([]string{"/Platform", p.cfg.GetNameUrl(), p.cfg.GetServerUrl()})
		if len(vs) != 3 {
			p.api.Logf(lib.LLDEBUG, "skipping node %s, doesn't have complete PMC info", n.ID().String())
			continue
		}
		if vs["/Platform"].String() != PlatformString { // Note: this may need to be more flexible in the future
			continue
		}
		name := vs[p.cfg.GetNameUrl()].String()
		srv := vs[p.cfg.GetServerUrl()].String()
		idmap[name] = n.ID()
		bySrv[srv] = append(bySrv[srv], name)
	}

	// This is not very efficient, but we assume that this module won't be used for huge amounts of nodes
	for s, ns := range bySrv {
		for _, n := range ns {
			p.nodeDiscover(s, n, idmap[n])
		}
	}
}

// initialization
func init() {
	module := &PMC{}
	mutations := make(map[string]lib.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	drstate := make(map[string]reflect.Value)

	for m := range muts {
		dur, _ := time.ParseDuration(muts[m].timeout)
		mutations[m] = core.NewStateMutation(
			map[string][2]reflect.Value{
				"/PhysState": [2]reflect.Value{
					reflect.ValueOf(muts[m].f),
					reflect.ValueOf(muts[m].t),
				},
			},
			reqs,
			excs,
			lib.StateMutationContext_CHILD,
			dur,
			[3]string{module.Name(), "/PhysState", "PHYS_HANG"},
		)
		drstate[cpb.Node_PhysState_name[int32(muts[m].t)]] = reflect.ValueOf(muts[m].t)
	}
	discovers["/PhysState"] = drstate
	discovers["/RunState"] = map[string]reflect.Value{
		"RUN_UK": reflect.ValueOf(cpb.Node_UNKNOWN),
	}
	discovers["/Services/powermancontrol/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}
	si := core.NewServiceInstance("powermancontrol", module.Name(), nil)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
