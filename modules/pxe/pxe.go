/* pxe.go: provides generic PXE/iPXE-boot capabilities
 *           this manages both DHCP and TFTP/HTTP services.
 *			 If <file> doesn't exist, but <file>.tpl does, tftp will fill it as as template.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. pxemodule.proto

package pxe

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/mdlayher/raw"

	"github.com/golang/protobuf/ptypes"

	"github.com/gogo/protobuf/proto"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/ipv4"
	pxepb "github.com/hpc/kraken/extensions/pxe"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

const (
	PXEStateURL = "type.googleapis.com/PXE.Client/State"
	SrvStateURL = "/Services/pxe/State"
)

type pxmut struct {
	f       pxepb.Client_State
	t       pxepb.Client_State
	reqs    map[string]reflect.Value
	timeout string
}

var muts = map[string]pxmut{
	"NONEtoWAIT": {
		f:       pxepb.Client_NONE,
		t:       pxepb.Client_WAIT,
		reqs:    reqs,
		timeout: "10s",
	},
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/PhysState": reflect.ValueOf(cpb.Node_POWER_ON),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

/* we use channels and a node manager rather than locking
   to make our node store safe.  This is a simpple query
   language for that service */

type nodeQueryBy string

const (
	queryByIP  nodeQueryBy = "IP"
	queryByMAC nodeQueryBy = "MAC"
)

//////////////////
// PXE Object /
////////////////

// PXE provides PXE-boot capabilities
type PXE struct {
	api   types.ModuleAPIClient
	cfg   *Config
	mchan <-chan types.Event
	dchan chan<- types.Event

	selfIP  net.IP
	selfNet net.IP

	options   layers.DHCPOptions
	leaseTime time.Duration

	iface     *net.Interface
	rawHandle *raw.Conn

	// for maintaining our list of currently booting nodes

	mutex  sync.RWMutex
	nodeBy map[nodeQueryBy]map[string]types.Node
}

/*
 * concurrency safe accessors for nodeBy
 */

// NodeGet gets a node that we know about -- concurrency safe
func (px *PXE) NodeGet(qb nodeQueryBy, q string) (n types.Node) { // returns nil for not found
	var ok bool
	px.mutex.RLock()
	if n, ok = px.nodeBy[qb][q]; !ok {
		px.api.Logf(types.LLERROR, "tried to acquire node that doesn't exist: %s %s", qb, q)
		px.mutex.RUnlock()
		return
	}
	px.mutex.RUnlock()
	return
}

// NodeDelete deletes a node that we know about -- cuncurrency safe
func (px *PXE) NodeDelete(qb nodeQueryBy, q string) { // silently ignores non-existent nodes
	var n types.Node
	var ok bool
	px.mutex.Lock()
	if n, ok = px.nodeBy[qb][q]; !ok {
		px.mutex.Unlock()
		return
	}
	v, e := n.GetValues([]string{px.cfg.IpUrl, px.cfg.MacUrl})
	if e != nil {
		px.api.Logf(types.LLERROR, "error getting values for node: %v", e)
	}
	ip := v[px.cfg.IpUrl].Interface().(*ipv4.IP).IP
	mac := v[px.cfg.MacUrl].Interface().(*ipv4.IP).IP
	delete(px.nodeBy[queryByIP], ip.String())
	delete(px.nodeBy[queryByMAC], mac.String())
	px.mutex.Unlock()
}

// NodeCreate creates a new node in our node pool -- concurrency safe
func (px *PXE) NodeCreate(n types.Node) (e error) {
	v, e := n.GetValues([]string{px.cfg.IpUrl, px.cfg.MacUrl})
	if e != nil {
		px.api.Logf(types.LLERROR, "error getting values for node: %v", e)
	}
	if len(v) != 2 {
		return fmt.Errorf("missing ip or mac for node, aborting")
	}
	ip := v[px.cfg.IpUrl].Interface().(*ipv4.IP).IP
	mac := v[px.cfg.MacUrl].Interface().(*ipv4.MAC).HardwareAddr
	if ip == nil || mac == nil { // incomplete node
		return fmt.Errorf("won't add incomplete node: ip: %v, mac: %v", ip, mac)
	}
	px.mutex.Lock()
	px.nodeBy[queryByIP][ip.String()] = n
	px.nodeBy[queryByMAC][mac.String()] = n
	px.mutex.Unlock()
	return
}

/*
 * types.Module
 */

var _ types.Module = (*PXE)(nil)

// Name returns the FQDN of the module
func (*PXE) Name() string { return "github.com/hpc/kraken/modules/pxe" }

/*
 * types.ModuleWithConfig
 */

var _ types.Module = (*PXE)(nil)

// NewConfig returns a fully initialized default config
func (*PXE) NewConfig() proto.Message {
	r := &Config{
		SrvIfaceUrl: "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Eth/Iface",
		SrvIpUrl:    "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Ip/Ip",
		IpUrl:       "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Ip/Ip",
		NmUrl:       "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Ip/Subnet",
		SubnetUrl:   "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Ip/Subnet",
		MacUrl:      "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Eth/Mac",
		TftpDir:     "tftp",
	}
	return r
}

// UpdateConfig updates the running config
func (px *PXE) UpdateConfig(cfg proto.Message) (e error) {
	if pxcfg, ok := cfg.(*Config); ok {
		px.cfg = pxcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*PXE) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * types.ModuleWithMutations & types.ModuleWithDiscovery
 */
var _ types.ModuleWithMutations = (*PXE)(nil)
var _ types.ModuleWithDiscovery = (*PXE)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (px *PXE) SetMutationChan(c <-chan types.Event) { px.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (px *PXE) SetDiscoveryChan(c chan<- types.Event) { px.dchan = c }

/*
 * types.ModuleSelfService
 */
var _ types.ModuleSelfService = (*PXE)(nil)

// Entry is the module's executable entrypoint
func (px *PXE) Entry() {
	nself, _ := px.api.QueryRead(px.api.Self().String())
	v, _ := nself.GetValue(px.cfg.SrvIpUrl)
	px.selfIP = v.Interface().(*ipv4.IP).IP
	v, _ = nself.GetValue(px.cfg.SubnetUrl)
	px.selfNet = v.Interface().(*ipv4.IP).IP
	v, _ = nself.GetValue(px.cfg.SrvIfaceUrl)
	go px.StartDHCP(v.String(), px.selfIP)
	go px.StartTFTP(px.selfIP)
	url := util.NodeURLJoin(px.api.Self().String(), SrvStateURL)
	ev := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)
	px.dchan <- ev
	for {
		select {
		case v := <-px.mchan:
			if v.Type() != types.Event_STATE_MUTATION {
				px.api.Log(types.LLERROR, "got unexpected non-mutation event")
				break
			}
			m := v.Data().(*core.MutationEvent)
			go px.handleMutation(m)
			break
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (px *PXE) Init(api types.ModuleAPIClient) {
	px.api = api
	px.mutex = sync.RWMutex{}
	px.nodeBy = make(map[nodeQueryBy]map[string]types.Node)
	px.nodeBy[queryByIP] = make(map[string]types.Node)
	px.nodeBy[queryByMAC] = make(map[string]types.Node)
	px.cfg = px.NewConfig().(*Config)
}

// Stop should perform a graceful exit
func (px *PXE) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (px *PXE) handleMutation(m *core.MutationEvent) {
	switch m.Type {
	case core.MutationEvent_MUTATE:
		switch m.Mutation[1] {
		case "NONEtoWAIT": // starting a new mutation, register the node
			if e := px.NodeCreate(m.NodeCfg); e != nil {
				px.api.Logf(types.LLERROR, "%v", e)
				break
			}
			url := util.NodeURLJoin(m.NodeCfg.ID().String(), PXEStateURL)
			ev := core.NewEvent(
				types.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{
					URL:     url,
					ValueID: "WAIT",
				},
			)
			px.dchan <- ev
		case "WAITtoINIT": // we're initializing, but don't do anything (more for discovery/timeout)
		}
	case core.MutationEvent_INTERRUPT: // on any interrupt, we remove the node
		v, e := m.NodeCfg.GetValue(px.cfg.IpUrl)
		if e != nil || !v.IsValid() {
			break
		}
		ip := v.Interface().(*ipv4.IP)
		px.NodeDelete(queryByIP, ip.String())
	}
}

func init() {
	module := &PXE{}
	mutations := make(map[string]types.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	dpxe := make(map[string]reflect.Value)
	si := core.NewServiceInstance("pxe", module.Name(), module.Entry)

	for m := range muts {
		dur, _ := time.ParseDuration(muts[m].timeout)
		mutations[m] = core.NewStateMutation(
			map[string][2]reflect.Value{
				PXEStateURL: {
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
		dpxe[pxepb.Client_State_name[int32(muts[m].t)]] = reflect.ValueOf(muts[m].t)
	}

	mutations["WAITtoINIT"] = core.NewStateMutation(
		map[string][2]reflect.Value{
			PXEStateURL: {
				reflect.ValueOf(pxepb.Client_WAIT),
				reflect.ValueOf(pxepb.Client_INIT),
			},
			"/RunState": {
				reflect.ValueOf(cpb.Node_UNKNOWN),
				reflect.ValueOf(cpb.Node_INIT),
			},
		},
		reqs,
		excs,
		types.StateMutationContext_CHILD,
		time.Second*90,
		[3]string{si.ID(), "/PhysState", "PHYS_HANG"},
	)
	dpxe["INIT"] = reflect.ValueOf(pxepb.Client_INIT)

	discovers[PXEStateURL] = dpxe
	discovers["/RunState"] = map[string]reflect.Value{
		"NODE_INIT": reflect.ValueOf(cpb.Node_INIT),
	}
	discovers["/PhysState"] = map[string]reflect.Value{
		"PHYS_HANG": reflect.ValueOf(cpb.Node_PHYS_HANG),
	}
	discovers[SrvStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
