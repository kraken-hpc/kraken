/* powermancontrol.go: used to control power on nodes via powerman
 *
 * Author: R. Eli Snyder <resnyder@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto -I . --go_out=plugins=grpc:. powermancontrol.proto

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
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

const (
	PMCBase        string = "/powermancontrol"
	PMCSet         string = PMCBase + "/setstate"
	PMCStat        string = PMCBase + "/nodestatus"
	PlatformString string = "powerman"
)

// pmcNode is the response from powermanapi
type pmcNode struct {
	Name  string `json:"name,omitempty"`
	State string `json:"state,omitempty"`
}

// ppmut helps us succinctly define our mutations
type pmcmut struct {
	f       cpb.Node_PhysState // from
	t       cpb.Node_PhysState // to
	timeout string             // timeout
	// everything fails to PHYS_HANG
}

// our mutation definitions
// also we discover anything we can migrate to
// Timeouts are a minute because powerman really can take that long to execute a command
var muts = map[string]pmcmut{
	"UKtoOFF": pmcmut{
		f:       cpb.Node_PHYS_UNKNOWN,
		t:       cpb.Node_POWER_OFF,
		timeout: "1m",
	},
	"OFFtoON": pmcmut{
		f:       cpb.Node_POWER_OFF,
		t:       cpb.Node_POWER_ON,
		timeout: "1m",
	},
	"ONtoOFF": pmcmut{
		f:       cpb.Node_POWER_ON,
		t:       cpb.Node_POWER_OFF,
		timeout: "1m",
	},
	"HANGtoOFF": pmcmut{
		f:       cpb.Node_PHYS_HANG,
		t:       cpb.Node_POWER_OFF,
		timeout: "1m", // we need a longer timeout, because we let it sit cold for a few seconds
	},
	"UKtoHANG": pmcmut{ // this one should never happen; just making sure HANG gets connected in our graph
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
	api        types.ModuleAPIClient
	cfg        *Config
	mchan      <-chan types.Event
	dchan      chan<- types.Event
	pollTicker *time.Ticker
	fireTicker *time.Ticker
	queueMutex *sync.Mutex
	queue      map[string][3]string // map[<nodename>][<srvName>, <mutation>, <nodeidstr>]
}

/*
 *types.Module
 */
var _ types.Module = (*PMC)(nil)

// Name returns the FQDN of the module
func (p *PMC) Name() string { return "github.com/hpc/kraken/modules/powermancontrol" }

/*
 * types.ModuleWithConfig
 */
var _ types.ModuleWithConfig = (*PMC)(nil)

// NewConfig returns a fully initialized default config
func (p *PMC) NewConfig() proto.Message {
	r := &Config{
		ServerUrl: "type.googleapis.com/proto.Powerman/ApiServer",
		NameUrl:   "type.googleapis.com/proto.Powerman/Name",
		Servers: map[string]*Server{
			"pmc": {
				Name: "pmc",
				Ip:   "localhost",
				Port: 8269,
			},
		},
		PollingInterval: "30s",
		FireInterval:    "2s",
	}
	return r
}

// UpdateConfig updates the running config
func (p *PMC) UpdateConfig(cfg proto.Message) (e error) {
	if pcfg, ok := cfg.(*Config); ok {
		p.cfg = pcfg
		if p.pollTicker != nil {
			p.pollTicker.Stop()
			dur, _ := time.ParseDuration(p.cfg.GetPollingInterval())
			p.pollTicker = time.NewTicker(dur)
			return
		}
		if p.fireTicker != nil {
			p.fireTicker.Stop()
			dur, _ := time.ParseDuration(p.cfg.GetFireInterval())
			p.fireTicker = time.NewTicker(dur)
			return
		}
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*PMC) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * types.ModuleWithMutations & types.ModuleWithDiscovery
 */
var _ types.ModuleWithMutations = (*PMC)(nil)
var _ types.ModuleWithDiscovery = (*PMC)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (p *PMC) SetMutationChan(c <-chan types.Event) { p.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (p *PMC) SetDiscoveryChan(c chan<- types.Event) { p.dchan = c }

/*
 * types.ModuleSelfService
 */
var _ types.ModuleSelfService = (*PMC)(nil)

// Entry is the module's executable entrypoint
func (p *PMC) Entry() {
	url := util.NodeURLJoin(p.api.Self().String(),
		util.URLPush(util.URLPush("/Services", "powermancontrol"), "State"))
	p.dchan <- core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)

	pDur, _ := time.ParseDuration(p.cfg.GetPollingInterval())
	p.pollTicker = time.NewTicker(pDur)

	fDur, _ := time.ParseDuration(p.cfg.GetFireInterval())
	p.fireTicker = time.NewTicker(fDur)

	for {
		select {
		case <-p.pollTicker.C:
			go p.discoverAll()
			break
		case <-p.fireTicker.C: // send any commands that are waiting in queue
			go p.fireChanges()
			break
		case m := <-p.mchan:
			if m.Type() != types.Event_STATE_MUTATION {
				p.api.Log(types.LLERROR, "got unexpected non-mutation event")
				break
			}
			go p.handleMutation(m)
			break
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (p *PMC) Init(api types.ModuleAPIClient) {
	p.api = api
	p.cfg = p.NewConfig().(*Config)
	p.queueMutex = &sync.Mutex{}
	p.queue = make(map[string][3]string)
}

// Stop should perform a graceful exit
func (p *PMC) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (p *PMC) handleMutation(m types.Event) {
	if m.Type() != types.Event_STATE_MUTATION {
		p.api.Log(types.LLINFO, "got an unexpected event type on mutation channel")
	}

	me := m.Data().(*core.MutationEvent)
	// extract the mutating node's name and server
	vs, e := me.NodeCfg.GetValues([]string{p.cfg.GetNameUrl(), p.cfg.GetServerUrl()})
	if e != nil {
		p.api.Logf(types.LLERROR, "error getting values for node: %v", e)
	}
	if len(vs) != 2 {
		p.api.Logf(types.LLERROR, "could not get NID and/or PMC Server for node: %s", me.NodeCfg.ID().String())
		return
	}
	name := vs[p.cfg.GetNameUrl()].String()
	srv := vs[p.cfg.GetServerUrl()].String()

	// mutation switch
	switch me.Type {
	case core.MutationEvent_MUTATE:
		switch me.Mutation[1] {
		case "UKtoOFF": // this just forces discover
			fallthrough
		case "OFFtoON":
			fallthrough
		case "ONtoOFF":
			fallthrough
		case "HANGtoOFF":
			p.queueMutex.Lock()
			p.queue[name] = [3]string{srv, me.Mutation[1], me.NodeCfg.ID().String()}
			p.queueMutex.Unlock()
			break
		case "UKtoHANG": // we don't actually do this
			fallthrough
		default:
			p.api.Logf(types.LLDEBUG, "unexpected event: %s", me.Mutation[1])
		}
		break
	case core.MutationEvent_INTERRUPT:
		p.queueMutex.Lock()
		delete(p.queue, name)
		p.queueMutex.Unlock()
		break
	}
}

func (p *PMC) fireChanges() {
	on := map[string][]string{}
	off := map[string][]string{}
	stat := map[string][]string{}

	idmap := map[string]string{}

	p.queueMutex.Lock()
	for k, v := range p.queue {
		nodeName := k
		srvName := v[0]
		mutation := v[1]
		nodeID := v[2]
		idmap[nodeName] = nodeID
		switch mutation {
		case "UKtoOFF": // this actually just forces discovery
			stat[srvName] = append(stat[srvName], nodeName)
			break
		case "OFFtoON":
			on[srvName] = append(on[srvName], nodeName)
			break
		case "ONtoOFF":
			fallthrough
		case "HANGtoOFF":
			off[srvName] = append(off[srvName], nodeName)
			break
		}
	}
	p.queue = make(map[string][3]string)
	p.queueMutex.Unlock()
	for srv, nodes := range on {
		p.fire(srv, nodes, PMCSet+"/poweron", idmap)
	}
	for srv, nodes := range off {
		p.fire(srv, nodes, PMCSet+"/poweroff", idmap)
	}
	for srv, nodes := range stat {
		p.fire(srv, nodes, PMCStat, idmap)
	}
}

func (p *PMC) fire(pSrv string, nodes []string, cmd string, idmap map[string]string) {
	srv, ok := p.cfg.Servers[pSrv]
	if !ok {
		p.api.Logf(types.LLERROR, "cannot control power for unknown server: %s", pSrv)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))
	nlist := strings.Join(nodes, ",")
	url := "http://" + addr + cmd + "/" + nlist
	resp, e := http.Get(url)
	if e != nil {
		p.api.Logf(types.LLERROR, "error dialing api: %v", e)
		return
	}
	defer resp.Body.Close()
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		p.api.Logf(types.LLERROR, "error reading api response body: %v", e)
		return
	}
	rs := []pmcNode{}
	e = json.Unmarshal(body, &rs)
	if e != nil {
		p.api.Logf(types.LLERROR, "error unmarshaling json: %v", e)
		return
	}
	for _, r := range rs {
		url := util.NodeURLJoin(idmap[r.Name], "/PhysState")
		v := core.NewEvent(
			types.Event_DISCOVERY,
			url,
			&core.DiscoveryEvent{
				URL:     url,
				ValueID: r.State,
			},
		)
		p.dchan <- v
	}
}

func (p *PMC) discoverAll() {
	p.api.Log(types.LLDEBUG, "polling for node state")
	ns, e := p.api.QueryReadAll()
	if e != nil {
		p.api.Logf(types.LLERROR, "polling node query failed: %v", e)
		return
	}

	// build lists
	for _, n := range ns {
		vs, e := n.GetValues([]string{"/Platform", p.cfg.GetNameUrl(), p.cfg.GetServerUrl()})
		if e != nil {
			p.api.Logf(types.LLERROR, "error getting values for node: %v", e)
		}
		if len(vs) != 3 {
			p.api.Logf(types.LLDEBUG, "skipping node %s, doesn't have complete PMC info", n.ID().String())
			continue
		}
		if vs["/Platform"].String() != PlatformString { // Note: this may need to be more flexible in the future
			continue
		}
		name := vs[p.cfg.GetNameUrl()].String()
		srv := vs[p.cfg.GetServerUrl()].String()
		// This will force a discover
		p.queue[name] = [3]string{srv, "UKtoOFF", n.ID().String()}
	}

}

// initialization
func init() {
	module := &PMC{}
	mutations := make(map[string]types.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	drstate := make(map[string]reflect.Value)
	si := core.NewServiceInstance("powermancontrol", module.Name(), module.Entry)

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
			types.StateMutationContext_CHILD,
			dur,
			[3]string{si.ID(), "/PhysState", "PHYS_HANG"},
		)
		drstate[cpb.Node_PhysState_name[int32(muts[m].t)]] = reflect.ValueOf(muts[m].t)
	}
	discovers["/PhysState"] = drstate
	discovers["/RunState"] = map[string]reflect.Value{
		"RUN_UK": reflect.ValueOf(cpb.Node_UNKNOWN),
	}
	discovers["/Services/powermancontrol/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
