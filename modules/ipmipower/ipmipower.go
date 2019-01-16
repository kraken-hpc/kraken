/* ipmipower.go: this module will allow the capability of turning on and off nodes
 *
 * Author: R. Eli Snyder <resnyder@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/ipmipower.proto

package ipmipower

import (
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/IPv4"
	"github.com/hpc/kraken/lib"
	"github.com/hpc/kraken/lib/ipmi"
	pb "github.com/hpc/kraken/modules/ipmipower/proto"
)

var _ lib.Module = (*Ipmipower)(nil)
var _ lib.ModuleSelfService = (*Ipmipower)(nil)
var _ lib.ModuleWithConfig = (*Ipmipower)(nil)
var _ lib.ModuleWithDiscovery = (*Ipmipower)(nil)
var _ lib.ModuleWithMutations = (*Ipmipower)(nil)

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
		timeout: "10s",
	},
	"UKtoHANG": ppmut{ // this one should never happen; just making sure HANG gets connected in our graph
		f:       cpb.Node_PHYS_UNKNOWN,
		t:       cpb.Node_PHYS_HANG,
		timeout: "0s",
	},
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Platform": reflect.ValueOf("ipmi"),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

// An Ipmipower can power on and off nodes
type Ipmipower struct {
	cfg   *pb.IpmipowerConfig
	api   lib.APIClient
	dchan chan<- lib.Event
	mchan <-chan lib.Event
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
			ValueID: "RUN",
		},
	)

	for {
		select {
		case m := <-p.mchan:
			if m.Type() != lib.Event_STATE_MUTATION {
				p.api.Log(lib.LLERROR, "got unexpected non-mutation event")
				break
			}
			me := m.Data().(*core.MutationEvent)
			p.handleMutation(me)
			break
		}
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
		BmcIpUrl: "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/1/Ip/Ip",
		BmcPort:  623,
		User:     "",
		Pass:     "",
		Oper:     "status",
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

func (p *Ipmipower) bmcURLToIP(n *lib.Node) ([]byte, error) {
	ipURL := p.cfg.BmcIpUrl
	ipv, e := (*n).GetValue(ipURL)
	if e != nil {
		return []byte{}, e
	}
	if ipv.Type() != reflect.TypeOf([]byte{}) {
		e := errors.New("The IP provided is not 4 bytes long")
		return []byte{}, e
	}
	ip := IPv4.BytesToIP(ipv.Bytes())
	return ip, error(nil)
}

// Handles mutations
func (p *Ipmipower) handleMutation(m *core.MutationEvent) {
	subCmds := map[string]uint8{
		"off": ipmi.IPMIChassisCtlDown,
		"on":  ipmi.IPMIChassisCtlUp,
	}

	switch m.Type {
	case core.MutationEvent_MUTATE:
		switch m.Mutation[1] {
		case "UKtoOFF":
			var cc uint8
			var d []byte
			ip, e := p.bmcURLToIP(&m.NodeCfg)
			if e != nil {
				p.api.Logf(lib.LLERROR, "Error parsing IP: %s", e)
				return
			}
			ipmiAddr := net.UDPAddr{IP: ip, Port: int(p.cfg.BmcPort)}
			ipmiSes := ipmi.NewIPMISession(&ipmiAddr)
			e = ipmiSes.Start(p.cfg.User, p.cfg.Pass)
			if e != nil {
				p.api.Logf(lib.LLERROR, "error starting IPMI session with %s: %s", e)
				return
			}
			cc, d, e = ipmiSes.Send(ipmi.IPMIFnChassisReq, ipmi.IPMICmdChassisCtl, []byte{subCmds[p.cfg.Oper]})
			url := lib.NodeURLJoin(m.NodeCfg.ID().String(), "/PhysState")
			vid := ""
			if e != nil {
				p.api.Logf(lib.LLERROR, "error sending IPMI command to %s: %s", IPv4.BytesToIP(ip).String(), e)
				return
			}
			if cc != 0x00 {
				p.api.Logf(lib.LLERROR, "bad completion code to %s: %x", IPv4.BytesToIP(ip).String(), cc)
				return
			}
			if len(d) != 4 {
				p.api.Logf(lib.LLERROR, "got unexecuted status data length to %s", IPv4.BytesToIP(ip).String())
				return
			}
			if d[0]&0x01 != 0 {
				vid = "POWER_ON"
			} else {
				vid = "POWER_OFF"
			}
			if d[0]&0x02 != 0 {
				p.api.Logf(lib.LLERROR, "power overload")
				return
			}
			if d[0]&0x04 != 0 {
				p.api.Logf(lib.LLERROR, "interlock")
				return
			}
			if d[0]&0x08 != 0 {
				p.api.Logf(lib.LLERROR, "power fault")
				return
			}
			if d[0]&0x10 != 0 {
				p.api.Logf(lib.LLERROR, "power control fault")
				return
			}
			v := core.NewEvent(
				lib.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{
					Module:  p.Name(),
					URL:     url,
					ValueID: vid,
				},
			)
			p.dchan <- v
			break
		case "HANGtoOFF":
			fallthrough
		case "OFFtoON":
			fallthrough
		case "ONtoOFF":
			var cc uint8
			ip, e := p.bmcURLToIP(&m.NodeCfg)
			url := lib.NodeURLJoin(m.NodeCfg.ID().String(), "/PhysState")
			if e != nil {
				p.api.Logf(lib.LLERROR, "Error parsing IP: %s", e)
				return
			}
			ipmiAddr := net.UDPAddr{IP: ip, Port: int(p.cfg.BmcPort)}
			ipmiSes := ipmi.NewIPMISession(&ipmiAddr)
			e = ipmiSes.Start(p.cfg.User, p.cfg.Pass)
			if e != nil {
				p.api.Logf(lib.LLERROR, "error starting IPMI session with %s: %s", IPv4.BytesToIP(ip).String(), e)
				return
			}
			cc, _, e = ipmiSes.Send(ipmi.IPMIFnChassisReq, ipmi.IPMICmdChassisCtl, []byte{subCmds[p.cfg.Oper]})
			if e != nil {
				p.api.Logf(lib.LLERROR, "error sending IPMI command with %s: %s", IPv4.BytesToIP(ip).String(), e)
				return
			}
			if cc != 0x00 {
				p.api.Logf(lib.LLERROR, "bad completion code: %x", cc)
				return
			}
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
	}
	return
}

// Module needs to get registered here
func init() {
	module := &Ipmipower{}
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
			lib.StateMutationContext_CHILD, //child because performing action on somebody else, self if perm action on self
			dur,
			[3]string{module.Name(), "/PhysState", "PHYS_HANG"},
		)
		drstate[cpb.Node_PhysState_name[int32(muts[m].t)]] = reflect.ValueOf(muts[m].t)
	}
	discovers["/PhysState"] = drstate
	discovers["/RunState"] = map[string]reflect.Value{
		"RUN_UK": reflect.ValueOf(cpb.Node_UNKNOWN),
	}
	discovers["/Services/ipmipower/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}
	si := core.NewServiceInstance("ipmipower", module.Name(), module.Entry, nil)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(module, discovers)
	core.Registry.RegisterMutations(module, mutations)
}
