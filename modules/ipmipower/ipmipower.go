/* ipmipower.go: this module will allow the capability of turning on and off nodes
 *
 * Author: R. "Eli" Snyder <resnyder@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/ipmipower.proto

package ipmipower

import (
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	corepb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
	"github.com/hpc/kraken/lib/ipmi"
	pb "github.com/hpc/kraken/modules/ipmipower/proto"
)

var _ lib.Module = (*Ipmipower)(nil)
var _ lib.ModuleSelfService = (*Ipmipower)(nil)
var _ lib.ModuleWithConfig = (*Ipmipower)(nil)
var _ lib.ModuleWithDiscovery = (*Ipmipower)(nil)
var _ lib.ModuleWithMutations = (*Ipmipower)(nil)

var muts = map[string][2]string{
	"fail":      [2]string{"", "fail"},    //connecting fail to the graph
	"failToOff": [2]string{"fail", "off"}, //on fail reset to off
	"powOn":     [2]string{"off", "on"},   //power off to power on
	"powOff":    [2]string{"on", "off"},   //power on to power off
}

//len(timeouts) must equal len(muts)
var timeouts = map[string]int{
	"fail":       2,
	"failToOff:": 72,
	"powOn":      0,
	"powOff":     5,
}

// An Ipmipower can power on and off nodes
type Ipmipower struct {
	//api *core.APIClient
	cfg      *pb.IpmipowerConfig //
	api      lib.APIClient
	dchan    chan<- lib.Event
	mchan    <-chan lib.Event
	bmcIP    string
	bmcPort  uint
	username string
	pass     string
	oper     string
}

/*
 * Required methods
 */

// Entry is the module entry point.  Every module must have an Entry()
// It should already have an initialized API
// and a config in place before this is called.
func (p *Ipmipower) Entry() {
	if p.cfg == nil {
		p.cfg = p.NewConfig().(*pb.IpmipowerConfig)
	}

	url := lib.NodeURLJoin(p.api.Self().String(),
		lib.URLPush(lib.URLPush("/Services", "ipmipower"), "State"))
	p.dchan <- core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  p.Name(),
			URL:     url,
			ValueID: "Run",
		},
	)

	for {
		select {
		case d := p.dchan:

			break
		case m := p.mchan:
			if m.Type() != lib.Event_STATE_MUTATION {
				p.api.Log(lib.LLERROR, "got unexpected non-mutation event")
				break
			}
			break
	}
}

// Stop is responsible for making sure the module exits cleanly.Stop
// Every module must have a Stop()
// If this does not exit, it may get killed.
func (p *Ipmipower) Stop() {
	os.Exit(0)
}

// Name needs to return a uniquely identifying string.
// Every module must have a name.
// The URL to the module is a good choice.
func (p *Ipmipower) Name() string {
	return "github.com/hpc/kraken/modules/ipmipower"
}

/*
 * Optional methods
 */

// UpdateConfig takes a new module configuration, and potentially adjusts to changes it contains.
// Any module that needs configuration (most) should have this defined.
// If it is defined, updates will occur when they are changed in the state.
func (p *Ipmipower) UpdateConfig(cfg proto.Message) (e error) {
	fmt.Printf("(ipmipower) updating config to: %v\n", cfg)
	if dc, ok := cfg.(*pb.IpmipowerConfig); ok {
		p.cfg = dc
		p.Init(p.api)
		return
	}
	return fmt.Errorf("wrong config type")
}

// SetDiscoveryChan Sets the discover channel
func (p *Ipmipower) SetDiscoveryChan(c chan<- lib.Event) {
	p.dchan = c
}

// SetMutationChan Sets the mutation channel
func (p *Ipmipower) SetMutationChan(c <-chan lib.Event) {
	p.mchan = c
}

// Init initializes the module.
// If this exists, it will be called when the module is instantiated.
// It will be called before the API is initialized or the config is in place.
func (p *Ipmipower) Init(api lib.APIClient) {
	p.api = api
	if p.cfg == nil {
		p.cfg = p.NewConfig().(*pb.IpmipowerConfig)
	}
}

// NewConfig spits out a default config
func (*Ipmipower) NewConfig() proto.Message {
	return &pb.IpmipowerConfig{
		BmcIp:   "127.0.0.1",
		BmcPort: 623,
		User:    "",
		Pass:    "",
		Oper:    "status",
	}
}

// ConfigURL returns the configurl of the module
func (p *Ipmipower) ConfigURL() string {
	a, _ := ptypes.MarshalAny(p.NewConfig())
	return a.GetTypeUrl()
}

var mutations = make(map[string]lib.StateMutation)
var discovers = make(map[string]map[string]reflect.Value)

////////////////////////
// Unexported methods /
///////////////////////

// Handles mutations
func (p *Ipmipower) handleMutation(m *core.MutationEvent) {
	switch M.Type {
	case core.MutationEvent_MUTATE:
		switch m.Mutation[1] {
		case "powOn":
			ipAddr, _, e := net.ParseCIDR(p.cfg.BmcIp)
			if e != nil {
				log.Println(e)
			}
			ipmiAddr := net.UDPAddr{IP: ipAddr, Port: int(p.cfg.BmcPort)}
			ipmiSes := ipmi.NewIPMISession(&ipmiAddr)
			e = ipmiSes.Start(p.cfg.User, p.cfg.Pass)
			if e != nil {
				log.Println(e)
			}
			
			break
		case "powOff":
			ipAddr, _, e := net.ParseCIDR(p.cfg.BmcIp)
			if e != nil {
				log.Println(e)
			}
			ipmiAddr := net.UDPAddr{IP: ipAddr, Port: int(p.cfg.BmcPort)}
			ipmiSes := ipmi.NewIPMISession(&ipmiAddr)
			e = ipmiSes.Start(p.cfg.User, p.cfg.Pass)
			if e != nil {
				log.Println(e)
			}

			break
		}
	}
}

// Module needs to get registered her
func init() {
	module := &Ipmipower{}
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
				"/Arch":                reflect.ValueOf("ipmi"),
				"/Services/ipmi/State": reflect.ValueOf(corepb.ServiceInstance_RUN),
			},
			map[string]reflect.Value{},
			lib.StateMutationContext_SELF,
			time.Second*time.Duration(timeouts[k]),
			[3]string{module.Name(), "/Platform", "fail"},
		)
	}
	discovers["/Platform"] = pdsc
	discovers["/Services/ipmi/State"] = map[string]reflect.Value{
		"Run": reflect.ValueOf(corepb.ServiceInstance_RUN)}
	si := core.NewServiceInstance("ipmi", module.Name(), module.Entry, nil)
	si.SetState(lib.Service_STOP)
	core.Registry.RegisterDiscoverable(module, discovers)
	core.Registry.RegisterMutations(module, mutations)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
}
