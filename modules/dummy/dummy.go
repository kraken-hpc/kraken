/* dummy.go: this module is intended for testing purposes, and to be used as a template
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. dummy.proto

package dummy

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/kraken-hpc/kraken/core"
	corepb "github.com/kraken-hpc/kraken/core/proto"
	"github.com/kraken-hpc/kraken/lib/types"
	"github.com/kraken-hpc/kraken/lib/util"
)

// This is a simple module example

var _ types.Module = (*Dummy)(nil)
var _ types.ModuleSelfService = (*Dummy)(nil)
var _ types.ModuleWithConfig = (*Dummy)(nil)
var _ types.ModuleWithDiscovery = (*Dummy)(nil)
var _ types.ModuleWithMutations = (*Dummy)(nil)

var muts = map[string][2]string{
	"dumFail":   {"", "fail"},     // we need some in arrow to our failure mode or it won't get built into the graph
	"dumFto0":   {"fail", "dum0"}, // On fail we reset to dum0
	"dumDiscUK": {"", "dum0"},
	"dum0to1":   {"dum0", "dum1"},
	"dum1to0":   {"dum1", "dum0"},
	"dum1to1a":  {"dum1", "dum1a"},
	"dum1to1b":  {"dum1", "dum1b"},
	"dum1ato1":  {"dum1a", "dum1"},
	"dum1ato1b": {"dum1a", "dum1b"},
	"dum1bto0":  {"dum1b", "dum0"},
}

var timeouts = map[string]int{
	"dumFail":   0,
	"dumFto0":   5,
	"dumDiskUK": 5,
	"dum0to1":   5,
	"dum1to0":   5,
	"dum1to10a": 5,
	"dum1ato1b": 2, // this one will fail!... but we should recover to get to the right state
	"dum1bto0":  3,
}

// A Dummy says strings from its cfg.say every cfg.duration
type Dummy struct {
	//api *core.ModuleAPIClient
	cfg    *Config
	i      int
	api    types.ModuleAPIClient
	dchan  chan<- types.Event
	mchan  <-chan types.Event
	timers map[string]*time.Timer
}

/*
 * Required methods
 */

// Entry is the module entry point.  Every module must have an Entry()
// It should already have an initialized API
// and a config in place before this is called.
func (d *Dummy) Entry() {
	d.timers = make(map[string]*time.Timer)
	if d.cfg == nil {
		d.cfg = d.NewConfig().(*Config)
	}
	go func() {
		dur, e := time.ParseDuration(d.cfg.Wait)
		if e != nil {
			fmt.Printf("bad duration, %s, falling back to default\n", d.cfg.Wait)
		}
		if len(d.cfg.Say) == 0 {
			time.Sleep(dur)
			return
		}
		if d.i >= len(d.cfg.Say) {
			d.i = 0
		}
		fmt.Println("Dummy says " + d.cfg.Say[d.i])
		time.Sleep(dur)
		d.i++
	}()

	dtimer := make(chan types.Event)
	/*
		discover := func(v types.Event) {
			time.Sleep(3 * time.Second)
			dtimer <- v
		}
	*/
	url := util.NodeURLJoin(d.api.Self().String(),
		util.URLPush(util.URLPush("/Services", "dummy"), "State"))
	d.dchan <- core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "Run",
		},
	)

	for {
		select {
		case dv := <-dtimer:
			// time to discover something!
			fmt.Printf("discovered: %s\n", dv.Data().(*core.DiscoveryEvent).ValueID)
			d.dchan <- dv
			break
		case mv := <-d.mchan:
			// got a mutation
			data := mv.Data().(*core.MutationEvent)
			if data.Type == corepb.MutationControl_MUTATE {
				if data.Mutation[0] != d.Name() {
					fmt.Printf("got a mutation intended for someone else: %s\n", data.Mutation[0])
					continue
				}
				var val [2]string
				var ok bool
				if val, ok = muts[data.Mutation[1]]; !ok {
					fmt.Printf("got an unknown requested mutation: %s\n", data.Mutation[1])
					continue
				}
				url := util.NodeURLJoin(data.NodeCfg.ID().String(), "/Platform")
				fmt.Printf("initiating mutation: %s : %s -> %s\n", url, val[0], val[1])
				v := core.NewEvent(
					types.Event_DISCOVERY,
					url,
					&core.DiscoveryEvent{
						URL:     url,
						ValueID: val[1],
					},
				)
				d.timers[data.NodeCfg.ID().String()] = time.AfterFunc(3*time.Second, func() { dtimer <- v })
			} else {
				fmt.Printf("got an interrupt for %s, stopping discovery timer\n", data.NodeCfg.ID().String())
				d.timers[data.NodeCfg.ID().String()].Stop()
			}
			break
		}
	}
}

// Stop is responsible for making sure the module exits cleanly.Stop
// Every module must have a Stop()
// If this does not exit, it may get killed.
func (d *Dummy) Stop() {
	os.Exit(0)
}

// Name needs to return a uniquely identifying string.
// Every module must have a name.
// The URL to the module is a good choice.
func (d *Dummy) Name() string {
	return "github.com/kraken-hpc/kraken/modules/dummy"
}

/*
 * Optional methods
 */

// UpdateConfig takes a new module configuration, and potentially adjusts to changes it contains.
// Any module that needs configuration (most) should have this defined.
// If it is defined, updates will occur when they are changed in the state.
func (d *Dummy) UpdateConfig(cfg proto.Message) (e error) {
	fmt.Printf("(dummy) updating config to: %v\n", cfg)
	if dc, ok := cfg.(*Config); ok {
		d.cfg = dc
		d.Init(d.api)
		return
	}
	return fmt.Errorf("wrong config type")
}

func (d *Dummy) SetDiscoveryChan(c chan<- types.Event) {
	d.dchan = c
}

func (d *Dummy) SetMutationChan(c <-chan types.Event) {
	d.mchan = c
}

// Init initializes the module.
// If this exists, it will be called when the module is instantiated.
// It will be called before the API is initialized or the config is in place.
func (d *Dummy) Init(api types.ModuleAPIClient) {
	d.api = api
	d.i = 0
}

func (d *Dummy) NewConfig() proto.Message {
	return &Config{
		Say:  []string{"what"},
		Wait: "3s",
	}
}

func (d *Dummy) ConfigURL() string {
	a, _ := ptypes.MarshalAny(d.NewConfig())
	return a.GetTypeUrl()
}

var mutations = make(map[string]types.StateMutation)
var discovers = make(map[string]map[string]reflect.Value)

// Module needs to get registered here
func init() {
	module := &Dummy{}
	core.Registry.RegisterModule(module)
	pdsc := make(map[string]reflect.Value)
	si := core.NewServiceInstance("dummy", module.Name(), module.Entry)

	for k, v := range muts {
		pdsc[v[1]] = reflect.ValueOf(v[1])
		mutations[k] = core.NewStateMutation(
			map[string][2]reflect.Value{
				"/Platform": {
					reflect.ValueOf(v[0]),
					reflect.ValueOf(v[1]),
				},
			},
			map[string]reflect.Value{
				"/Arch":                 reflect.ValueOf("dummy"),
				"/Services/dummy/State": reflect.ValueOf(corepb.ServiceInstance_RUN),
			},
			map[string]reflect.Value{},
			types.StateMutationContext_SELF,
			time.Second*time.Duration(timeouts[k]),
			[3]string{si.ID(), "/Platform", "fail"},
		)
	}
	discovers["/Platform"] = pdsc
	discovers["/Services/dummy/State"] = map[string]reflect.Value{
		"Run": reflect.ValueOf(corepb.ServiceInstance_RUN)}
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
}
