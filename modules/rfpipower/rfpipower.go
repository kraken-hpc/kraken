/* rfpipower.go: a Redfish API based power control module
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>, Ghazanfar Ali <ghazanfar.ali@ttu.edu>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. rfpipower.proto

/*
 * This module will manipulate the PhysState state field.
 * It will be restricted to Arch = aarch64 & Platform = rpi3.
 * This requires at least Chassis & Rank to be set in the RPi3 extension.
 * This nodename format is a key to knowing which PiPower server to talk to.
 */

package rfpipower

import (
	"bytes"
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
	ptypes "github.com/gogo/protobuf/types"
	"github.com/kraken-hpc/kraken/core"
	cpb "github.com/kraken-hpc/kraken/core/proto"
	pipb "github.com/kraken-hpc/kraken/extensions/rpi3"
	"github.com/kraken-hpc/kraken/lib/types"
	"github.com/kraken-hpc/kraken/lib/util"
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

// payload struct for collection of nodes
type nodesInfo struct {
	CMD   string
	Nodes []string
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
// RFPiPower Object /
//////////////////

// RFPiPower provides a power on/off interface to the proprietary BitScope power control plane
type RFPiPower struct {
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
var _ types.Module = (*RFPiPower)(nil)

// Name returns the FQDN of the module
func (*RFPiPower) Name() string { return "github.com/kraken-hpc/kraken/modules/rfpipower" }

/*
 * types.ModuleWithConfig
 */
var _ types.ModuleWithConfig = (*RFPiPower)(nil)

// NewConfig returns a fully initialized default config
func (*RFPiPower) NewConfig() proto.Message {
	r := &Config{
		Servers: map[string]*Server{
			"C0": {
				Name: "C0",
				Ip:   "127.0.0.1",
				Port: 8000,
			},
		},
		Tick: "1s",
	}
	return r
}

// UpdateConfig updates the running config
func (pp *RFPiPower) UpdateConfig(cfg proto.Message) (e error) {
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
func (*RFPiPower) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * types.ModuleWithMutations & types.ModuleWithDiscovery
 */
var _ types.ModuleWithMutations = (*RFPiPower)(nil)
var _ types.ModuleWithDiscovery = (*RFPiPower)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (pp *RFPiPower) SetMutationChan(c <-chan types.Event) { pp.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (pp *RFPiPower) SetDiscoveryChan(c chan<- types.Event) { pp.dchan = c }

/*
 * types.ModuleSelfService
 */

var _ types.ModuleSelfService = (*RFPiPower)(nil)

// Entry is the module's executable entrypoint
func (pp *RFPiPower) Entry() {

	url := util.NodeURLJoin(pp.api.Self().String(),
		util.URLPush(util.URLPush("/Services", "rfpipower"), "State"))
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
func (pp *RFPiPower) Init(api types.ModuleAPIClient) {
	pp.api = api
	pp.mutex = &sync.Mutex{}
	pp.queue = make(map[string][2]string)
	pp.cfg = pp.NewConfig().(*Config)
}

// Stop should perform a graceful exit
func (pp *RFPiPower) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

// this might get some strange results if you don't stick to the c<num>n<num> scheme
func (*RFPiPower) parseNodeName(nn string) (chassis, node string) {
	// c0n12 -> c0, 12
	s := strings.Split(nn, "n")
	return s[0], s[1]
}

func (pp *RFPiPower) fireChanges() {
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
		pp.fire(c, on[c], "on", idmap)
	}
	for c := range off {
		pp.fire(c, off[c], "off", idmap)
	}
	for c := range stat {
		pp.fire(c, stat[c], "state", idmap)
	}

}

func (pp *RFPiPower) fire(c string, ns []string, cmd string, idmap map[string]string) {

	srv, ok := pp.cfg.Servers[c]
	if !ok {
		pp.api.Logf(types.LLERROR, "cannot control power for unknown chassis: %s", c)
		return
	}

	payLoad, _ := json.Marshal(nodesInfo{
		CMD:   cmd,
		Nodes: ns,
	})

	// URL construction: chassis ip, port, identity
	// change hard coded "ip" with "srv.Ip" and "port" with strconv.Itoa(int(srv.Port))
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))
	url := "http://" + addr + "/redfish/v1/Systems/" + c + "/Actions/ComputerSystem.Reset"

	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(payLoad))
	if err != nil {
		pp.api.Logf(types.LLERROR, "http PUT API request failed: %v", err)
		return
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		pp.api.Logf(types.LLERROR, "http PUT API call failed: %v", err)
		return
	}

	defer resp.Body.Close()
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		pp.api.Logf(types.LLERROR, "http PUT response failed to read body: %v", e)
		return
	}
	rs := []ppNode{}
	e = json.Unmarshal(body, &rs)
	if e != nil {
		pp.api.Logf(types.LLERROR, "got invalid JSON response: %v", e)
		fmt.Println(e)
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

func (pp *RFPiPower) handleMutation(m types.Event) {
	if m.Type() != types.Event_STATE_MUTATION {
		pp.api.Log(types.LLINFO, "got an unexpected event type on mutation channel")
	}
	me := m.Data().(*core.MutationEvent)
	vs, e := me.NodeCfg.GetValues([]string{ChassisURL, RankURL})
	// we make a speciall "nodename" consisting of <chassis>n<rank> to key by
	// mostly for historical convenience
	if e != nil {
		pp.api.Logf(types.LLERROR, "error getting values: %v", e)
	}
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
			break
		case "UKtoHANG": // we don't actually do this
			fallthrough
		default:
			//REVERSE pp.api.Logf(types.LLDEBUG, "unexpected event: %s", me.Mutation[1])
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
	module := &RFPiPower{}
	mutations := make(map[string]types.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	drstate := make(map[string]reflect.Value)
	si := core.NewServiceInstance("rfpipower", module.Name(), module.Entry)

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
	discovers["/Services/rfpipower/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
