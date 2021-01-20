/* pipower.go: a power control module for the proprietary BitScope power control module
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. pipower.proto

/*
 * This module will manipulate the PhysState state field.
 * It will be restricted to Arch = aarch64 & Platform = rpi3.
 * This requires at least Chassis & Rank to be set in the RPi3 extension.
 * This nodename format is a key to knowing which PiPower server to talk to.
 */

package pipower

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

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	pipb "github.com/hpc/kraken/extensions/rpi3"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

const (
	ChassisURL string = "type.googleapis.com/RPi3.Pi/Chassis"
	RankURL    string = "type.googleapis.com/RPi3.Pi/Rank"
)

// ppNode is the PiPower node struct
type ppNode struct {
	ID    string `json:"id,omitempty"`
	State string `json:"state,omitempty"`
}

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
	"UKtoOFF": {
		f:       cpb.Node_PHYS_UNKNOWN,
		t:       cpb.Node_POWER_OFF,
		timeout: "5s",
	},
	"OFFtoON": {
		f:       cpb.Node_POWER_OFF,
		t:       cpb.Node_POWER_ON,
		timeout: "5s",
	},
	"ONtoOFF": {
		f:       cpb.Node_POWER_ON,
		t:       cpb.Node_POWER_OFF,
		timeout: "5s",
	},
	"HANGtoOFF": {
		f:       cpb.Node_PHYS_HANG,
		t:       cpb.Node_POWER_OFF,
		timeout: "10s", // we need a longer timeout, because we let it sit cold for a few seconds
	},
	"UKtoHANG": { // this one should never happen; just making sure HANG gets connected in our graph
		f:       cpb.Node_PHYS_UNKNOWN,
		t:       cpb.Node_PHYS_HANG,
		timeout: "0s",
	},
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Arch":     reflect.ValueOf("aarch64"),
	"/Platform": reflect.ValueOf("rpi3"),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

////////////////////
// PiPower Object /
//////////////////

// PiPower provides a power on/off interface to the proprietary BitScope power control plane
type PiPower struct {
	api    types.ModuleAPIClient
	mutex  *sync.Mutex
	queue  map[string][2]string // map[<nodename>][<mutation>, <nodeidstr>]
	cfg    *Config
	mchan  <-chan types.Event
	dchan  chan<- types.Event
	ticker *time.Ticker
}

/*
 *types.Module
 */
var _ types.Module = (*PiPower)(nil)

// Name returns the FQDN of the module
func (*PiPower) Name() string { return "github.com/hpc/kraken/modules/pipower" }

/*
 * types.ModuleWithConfig
 */
var _ types.ModuleWithConfig = (*PiPower)(nil)

// NewConfig returns a fully initialized default config
func (*PiPower) NewConfig() proto.Message {
	r := &Config{
		Servers: map[string]*Server{
			"c0": {
				Name: "c0",
				Ip:   "127.0.0.1",
				Port: 8000,
			},
		},
		Tick: "1s",
	}
	return r
}

// UpdateConfig updates the running config
func (pp *PiPower) UpdateConfig(cfg proto.Message) (e error) {
	if ppcfg, ok := cfg.(*Config); ok {
		pp.cfg = ppcfg
		if pp.ticker != nil {
			pp.ticker.Stop()
			dur, _ := time.ParseDuration(pp.cfg.Tick)
			pp.ticker = time.NewTicker(dur)
		}
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*PiPower) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * types.ModuleWithMutations & types.ModuleWithDiscovery
 */
var _ types.ModuleWithMutations = (*PiPower)(nil)
var _ types.ModuleWithDiscovery = (*PiPower)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (pp *PiPower) SetMutationChan(c <-chan types.Event) { pp.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (pp *PiPower) SetDiscoveryChan(c chan<- types.Event) { pp.dchan = c }

/*
 * types.ModuleSelfService
 */
var _ types.ModuleSelfService = (*PiPower)(nil)

// Entry is the module's executable entrypoint
func (pp *PiPower) Entry() {
	url := util.NodeURLJoin(pp.api.Self().String(),
		util.URLPush(util.URLPush("/Services", "pipower"), "State"))
	pp.dchan <- core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)
	// main loop
	for {
		// fire a timer that will do our next work
		dur, _ := time.ParseDuration(pp.cfg.GetTick())
		pp.ticker = time.NewTicker(dur)
		select {
		case <-pp.ticker.C: // time to do work
			go pp.fireChanges()
			break
		case m := <-pp.mchan: // mutation request
			go pp.handleMutation(m)
			break
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (pp *PiPower) Init(api types.ModuleAPIClient) {
	pp.api = api
	pp.mutex = &sync.Mutex{}
	pp.queue = make(map[string][2]string)
	pp.cfg = pp.NewConfig().(*Config)
}

// Stop should perform a graceful exit
func (pp *PiPower) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

// this might get some strange results if you don't stick to the c<num>n<num> scheme
func (*PiPower) parseNodeName(nn string) (chassis, node string) {
	// c0n12 -> c0, 12
	s := strings.Split(nn, "n")
	return s[0], s[1]
}

func (pp *PiPower) fireChanges() {
	on := map[string][]string{}
	off := map[string][]string{}
	stat := map[string][]string{}

	idmap := map[string]string{}

	pp.mutex.Lock()
	for m := range pp.queue {
		c, n := pp.parseNodeName(m)
		idmap[m] = pp.queue[m][1]
		switch pp.queue[m][0] {
		case "UKtoOFF": // this actually just forces discovery
			stat[c] = append(stat[c], n)
			break
		case "OFFtoON":
			on[c] = append(on[c], n)
			break
		case "ONtoOFF":
			fallthrough
		case "HANGtoOFF":
			off[c] = append(off[c], n)
			break
		}
	}
	pp.queue = make(map[string][2]string)
	pp.mutex.Unlock()
	for c := range on {
		pp.fire(c, on[c], "/state/on", idmap)
	}
	for c := range off {
		pp.fire(c, off[c], "/state/off", idmap)
	}
	for c := range stat {
		pp.fire(c, stat[c], "/state", idmap)
	}
}

func (pp *PiPower) fire(c string, ns []string, cmd string, idmap map[string]string) {
	srv, ok := pp.cfg.Servers[c]
	if !ok {
		pp.api.Logf(types.LLERROR, "cannot control power for unknown chassis: %s", c)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))
	nlist := strings.Join(ns, ",")
	url := "http://" + addr + "/nodes/" + nlist + cmd
	resp, e := http.Get(url)
	if e != nil {
		pp.api.Logf(types.LLERROR, "http GET to API failed: %v", e)
		return
	}
	defer resp.Body.Close()
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		pp.api.Logf(types.LLERROR, "http GET failed to read body: %v", e)
		return
	}
	rs := []ppNode{}
	e = json.Unmarshal(body, &rs)
	if e != nil {
		pp.api.Logf(types.LLERROR, "got invalid JSON response: %v", e)
		return
	}
	for _, r := range rs {
		url := util.NodeURLJoin(idmap[c+"n"+r.ID], "/PhysState")
		vid := "POWER_OFF"
		if r.State == "on" {
			vid = "POWER_ON"
		}
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
}

func (pp *PiPower) handleMutation(m types.Event) {
	if m.Type() != types.Event_STATE_MUTATION {
		pp.api.Log(types.LLINFO, "got an unexpected event type on mutation channel")
	}
	me := m.Data().(*core.MutationEvent)
	//nodename := me.NodeCfg.Message().(*cpb.Node).Nodename
	vs, e := me.NodeCfg.GetValues([]string{ChassisURL, RankURL})
	if e != nil {
		pp.api.Logf(types.LLERROR, "error getting values for node: %v", e)
	}
	// we make a speciall "nodename" consisting of <chassis>n<rank> to key by
	// mostly for historical convenience
	if len(vs) != 2 {
		pp.api.Logf(types.LLERROR, "incomplete RPi3 data for power control: %v", vs)
		return
	}
	nodename := vs[ChassisURL].String() + "n" + strconv.FormatUint(vs[RankURL].Uint(), 10)
	switch me.Type {
	case core.MutationEvent_MUTATE:
		switch me.Mutation[1] {
		case "UKtoOFF": // this actually just forces discovery
			fallthrough
		case "OFFtoON":
			fallthrough
		case "ONtoOFF":
			fallthrough
		case "HANGtoOFF":
			pp.mutex.Lock()
			pp.queue[nodename] = [2]string{me.Mutation[1], me.NodeCfg.ID().String()}
			pp.mutex.Unlock()
			/*
					url := util.NodeURLJoin(me.NodeCfg.ID().String(), "/RunState")
					ev := core.NewEvent(
						types.Event_DISCOVERY,
						url,
						&core.DiscoveryEvent{
							URL:     url,
							ValueID: "RUN_UK",
						},
					)
					pp.dchan <- ev
				url := util.NodeURLJoin(me.NodeCfg.ID().String(), "type.googleapis.com/RPi3.Pi/Pxe")
				ev := core.NewEvent(
					types.Event_DISCOVERY,
					url,
					&core.DiscoveryEvent{
						URL:     url,
						ValueID: "PXE_NONE",
					},
				)
				pp.dchan <- ev
			*/
			break
		case "UKtoHANG": // we don't actually do this
			fallthrough
		default:
			pp.api.Logf(types.LLDEBUG, "unexpected event: %s", me.Mutation[1])
		}
		break
	case core.MutationEvent_INTERRUPT:
		pp.mutex.Lock()
		delete(pp.queue, nodename)
		pp.mutex.Unlock()
		break
	}
}

// initialization
func init() {
	module := &PiPower{}
	mutations := make(map[string]types.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	drstate := make(map[string]reflect.Value)
	si := core.NewServiceInstance("pipower", module.Name(), module.Entry)

	for m := range muts {
		dur, _ := time.ParseDuration(muts[m].timeout)
		mutations[m] = core.NewStateMutation(
			map[string][2]reflect.Value{
				"/PhysState": {
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
	discovers["type.googleapis.com/RPi3.Pi/Pxe"] = map[string]reflect.Value{
		"PXE_NONE": reflect.ValueOf(pipb.Pi_NONE),
	}
	discovers["/Services/pipower/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
