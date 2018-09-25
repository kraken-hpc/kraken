/* dummy.go: this module is intended for testing purposes, and to be used as a template
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/dummy.proto

package dummy

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	corepb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/dummy/proto"
)

// This is a simple module example

var _ lib.Module = (*Dummy)(nil)
var _ lib.ModuleSelfService = (*Dummy)(nil)
var _ lib.ModuleWithConfig = (*Dummy)(nil)
var _ lib.ModuleWithDiscovery = (*Dummy)(nil)
var _ lib.ModuleWithMutations = (*Dummy)(nil)

var muts = map[string][2]string{
	"dumFail":   [2]string{"", "fail"},     // we need some in arrow to our failure mode or it won't get built into the graph
	"dumFto0":   [2]string{"fail", "dum0"}, // On fail we reset to dum0
	"dumDiscUK": [2]string{"", "dum0"},
	"dum0to1":   [2]string{"dum0", "dum1"},
	"dum1to0":   [2]string{"dum1", "dum0"},
	"dum1to1a":  [2]string{"dum1", "dum1a"},
	"dum1to1b":  [2]string{"dum1", "dum1b"},
	"dum1ato1":  [2]string{"dum1a", "dum1"},
	"dum1ato1b": [2]string{"dum1a", "dum1b"},
	"dum1bto0":  [2]string{"dum1b", "dum0"},
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
	//api *core.APIClient
	cfg    *pb.DummyConfig
	i      int
	api    lib.APIClient
	dchan  chan<- lib.Event
	mchan  <-chan lib.Event
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
		d.cfg = d.NewConfig().(*pb.DummyConfig)
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

	dtimer := make(chan lib.Event)
	/*
		discover := func(v lib.Event) {
			time.Sleep(3 * time.Second)
			dtimer <- v
		}
	*/
	url := lib.NodeURLJoin(d.api.Self().String(),
		lib.URLPush(lib.URLPush("/Services", "dummy"), "State"))
	d.dchan <- core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  d.Name(),
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
				url := lib.NodeURLJoin(data.NodeCfg.ID().String(), "/Platform")
				fmt.Printf("initiating mutation: %s : %s -> %s\n", url, val[0], val[1])
				v := core.NewEvent(
					lib.Event_DISCOVERY,
					url,
					&core.DiscoveryEvent{
						Module:  d.Name(),
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
	return "github.com/hpc/kraken/modules/dummy"
}

/*
 * Optional methods
 */

// UpdateConfig takes a new module configuration, and potentially adjusts to changes it contains.
// Any module that needs configuration (most) should have this defined.
// If it is defined, updates will occur when they are changed in the state.
func (d *Dummy) UpdateConfig(cfg proto.Message) (e error) {
	fmt.Printf("(dummy) updating config to: %v\n", cfg)
	if dc, ok := cfg.(*pb.DummyConfig); ok {
		d.cfg = dc
		d.Init(d.api)
		return
	}
	return fmt.Errorf("wrong config type")
}

func (d *Dummy) SetDiscoveryChan(c chan<- lib.Event) {
	d.dchan = c
}

func (d *Dummy) SetMutationChan(c <-chan lib.Event) {
	d.mchan = c
}

// Init initializes the module.
// If this exists, it will be called when the module is instantiated.
// It will be called before the API is initialized or the config is in place.
func (d *Dummy) Init(api lib.APIClient) {
	d.api = api
	d.i = 0
}

func (d *Dummy) NewConfig() proto.Message {
	return &pb.DummyConfig{
		Say:  []string{"what"},
		Wait: "3s",
	}
}

func (d *Dummy) ConfigURL() string {
	a, _ := ptypes.MarshalAny(d.NewConfig())
	return a.GetTypeUrl()
}

var mutations = make(map[string]lib.StateMutation)
var discovers = make(map[string]map[string]reflect.Value)

// Module needs to get registered here
func init() {
	module := &Dummy{}
	core.Registry.RegisterModule(module)
	pdsc := make(map[string]reflect.Value)
	for k, v := range muts {
		pdsc[v[1]] = reflect.ValueOf(v[1])
		mutations[k] = core.NewStateMutation(
			map[string][2]reflect.Value{
				"/Platform": [2]reflect.Value{
					reflect.ValueOf(v[0]),
					reflect.ValueOf(v[1]),
				},
			},
			map[string]reflect.Value{
				"/Arch":                 reflect.ValueOf("dummy"),
				"/Services/dummy/State": reflect.ValueOf(corepb.ServiceInstance_RUN),
			},
			map[string]reflect.Value{},
			lib.StateMutationContext_SELF,
			time.Second*time.Duration(timeouts[k]),
			[3]string{module.Name(), "/Platform", "fail"},
		)
	}
	discovers["/Platform"] = pdsc
	discovers["/Services/dummy/State"] = map[string]reflect.Value{
		"Run": reflect.ValueOf(corepb.ServiceInstance_RUN)}
	si := core.NewServiceInstance("dummy", module.Name(), module.Entry, nil)
	si.SetState(lib.Service_STOP)
	core.Registry.RegisterDiscoverable(module, discovers)
	core.Registry.RegisterMutations(module, mutations)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
}
