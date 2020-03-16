/* pipxe.go: provides PXE-boot capabilities for Raspberry Pis
 *           this manages both DHCP and TFTP services.
 *           It incorperates some hacks to get the Rpi3B to boot consistently.
 *			 If <file> doesn't exist, but <file>.tpl does, tftp will fill it as as template.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/pipxe.proto

package pipxe

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

	"github.com/golang/protobuf/proto"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/IPv4"
	rpipb "github.com/hpc/kraken/extensions/RPi3/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/pipxe/proto"
)

const (
	PxeURL      = "type.googleapis.com/proto.RPi3/Pxe"
	SrvStateURL = "/Services/pipxe/State"
	MACVendor   = "b8:27:eb"
)

type pxmut struct {
	f       rpipb.RPi3_PXE
	t       rpipb.RPi3_PXE
	reqs    map[string]reflect.Value
	timeout string
}

var muts = map[string]pxmut{
	"NONEtoWAIT": {
		f:       rpipb.RPi3_NONE,
		t:       rpipb.RPi3_WAIT,
		reqs:    reqs,
		timeout: "10s",
	},
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Arch":      reflect.ValueOf("aarch64"),
	"/Platform":  reflect.ValueOf("rpi3"),
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
// PiPXE Object /
////////////////

// PiPXE provides PXE-boot capabilities for Raspberry Pis
type PiPXE struct {
	api   lib.APIClient
	cfg   *pb.PiPXEConfig
	mchan <-chan lib.Event
	dchan chan<- lib.Event

	selfIP  net.IP
	selfNet net.IP

	options   layers.DHCPOptions
	leaseTime time.Duration

	iface     *net.Interface
	rawHandle *raw.Conn

	// for maintaining our list of currently booting nodes

	mutex     sync.RWMutex
	nodeBy    map[nodeQueryBy]map[string]lib.Node
	wakeMutex sync.Mutex
	nodeWake  map[string]chan<- bool //[nodeId]doneChannel
}

/*
 * concurrency safe accessors for nodeBy
 */

// NodeGet gets a node that we know about -- concurrency safe
func (px *PiPXE) NodeGet(qb nodeQueryBy, q string) (n lib.Node) { // returns nil for not found
	var ok bool
	px.mutex.RLock()
	if n, ok = px.nodeBy[qb][q]; !ok {
		px.api.Logf(lib.LLERROR, "tried to acquire node that doesn't exist: %s %s", qb, q)
		px.mutex.RUnlock()
		return
	}
	px.mutex.RUnlock()
	return
}

// NodeDelete deletes a node that we know about -- cuncurrency safe
func (px *PiPXE) NodeDelete(qb nodeQueryBy, q string) { // silently ignores non-existent nodes
	var n lib.Node
	var ok bool
	px.mutex.Lock()
	if n, ok = px.nodeBy[qb][q]; !ok {
		px.mutex.Unlock()
		return
	}
	v, e := n.GetValues([]string{px.cfg.IpUrl, px.cfg.MacUrl})
	if e != nil {
		px.api.Logf(lib.LLERROR, "error getting values: %v", e)
	}
	ip := IPv4.BytesToIP(v[px.cfg.IpUrl].Bytes())
	mac := IPv4.BytesToMAC(v[px.cfg.MacUrl].Bytes())
	delete(px.nodeBy[queryByIP], ip.String())
	delete(px.nodeBy[queryByMAC], mac.String())
	px.mutex.Unlock()
}

// NodeCreate creates a new node in our node pool -- concurrency safe
func (px *PiPXE) NodeCreate(n lib.Node) (e error) {
	v, e := n.GetValues([]string{px.cfg.IpUrl, px.cfg.MacUrl})
	if e != nil {
		px.api.Logf(lib.LLERROR, "error getting values: %v", e)
	}
	if len(v) != 2 {
		return fmt.Errorf("missing ip or mac for node, aborting")
	}
	ip := IPv4.BytesToIP(v[px.cfg.IpUrl].Bytes())
	mac := IPv4.BytesToMAC(v[px.cfg.MacUrl].Bytes())
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
 * lib.Module
 */

var _ lib.Module = (*PiPXE)(nil)

// Name returns the FQDN of the module
func (*PiPXE) Name() string { return "github.com/hpc/kraken/modules/pipxe" }

/*
 * lib.ModuleWithConfig
 */

var _ lib.Module = (*PiPXE)(nil)

// NewConfig returns a fully initialized default config
func (*PiPXE) NewConfig() proto.Message {
	r := &pb.PiPXEConfig{
		SrvIfaceUrl: "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Eth/Iface",
		SrvIpUrl:    "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		IpUrl:       "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		NmUrl:       "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Subnet",
		SubnetUrl:   "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Subnet",
		MacUrl:      "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Eth/Mac",
		TftpDir:     "tftp",
		ArpDeadline: "500ms",
		DhcpRetry:   3,
	}
	return r
}

// UpdateConfig updates the running config
func (px *PiPXE) UpdateConfig(cfg proto.Message) (e error) {
	if pxcfg, ok := cfg.(*pb.PiPXEConfig); ok {
		px.cfg = pxcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*PiPXE) ConfigURL() string {
	cfg := &pb.PiPXEConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * lib.ModuleWithMutations & lib.ModuleWithDiscovery
 */
var _ lib.ModuleWithMutations = (*PiPXE)(nil)
var _ lib.ModuleWithDiscovery = (*PiPXE)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (px *PiPXE) SetMutationChan(c <-chan lib.Event) { px.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (px *PiPXE) SetDiscoveryChan(c chan<- lib.Event) { px.dchan = c }

/*
 * lib.ModuleSelfService
 */
var _ lib.ModuleSelfService = (*PiPXE)(nil)

// Entry is the module's executable entrypoint
func (px *PiPXE) Entry() {
	nself, _ := px.api.QueryRead(px.api.Self().String())
	v, _ := nself.GetValue(px.cfg.SrvIpUrl)
	px.selfIP = IPv4.BytesToIP(v.Bytes())
	v, _ = nself.GetValue(px.cfg.SubnetUrl)
	px.selfNet = IPv4.BytesToIP(v.Bytes())
	v, _ = nself.GetValue(px.cfg.SrvIfaceUrl)
	go px.StartDHCP(v.String(), px.selfIP)
	go px.StartTFTP(px.selfIP)
	url := lib.NodeURLJoin(px.api.Self().String(), SrvStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
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
			if v.Type() != lib.Event_STATE_MUTATION {
				px.api.Log(lib.LLERROR, "got unexpected non-mutation event")
				break
			}
			m := v.Data().(*core.MutationEvent)
			go px.handleMutation(m)
			break
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (px *PiPXE) Init(api lib.APIClient) {
	px.api = api
	px.mutex = sync.RWMutex{}
	px.nodeBy = make(map[nodeQueryBy]map[string]lib.Node)
	px.nodeBy[queryByIP] = make(map[string]lib.Node)
	px.nodeBy[queryByMAC] = make(map[string]lib.Node)
	px.wakeMutex = sync.Mutex{}
	px.nodeWake = make(map[string]chan<- bool)
	px.cfg = px.NewConfig().(*pb.PiPXEConfig)
}

// Stop should perform a graceful exit
func (px *PiPXE) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (px *PiPXE) handleMutation(m *core.MutationEvent) {
	switch m.Type {
	case core.MutationEvent_MUTATE:
		switch m.Mutation[1] {
		case "NONEtoWAIT": // starting a new mutation, register the node
			if e := px.NodeCreate(m.NodeCfg); e != nil {
				px.api.Logf(lib.LLERROR, "%v", e)
				break
			}
			url := lib.NodeURLJoin(m.NodeCfg.ID().String(), PxeURL)
			ev := core.NewEvent(
				lib.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{
					URL:     url,
					ValueID: "WAIT",
				},
			)

			var stop = make(chan bool)
			px.wakeMutex.Lock()
			if stop, ok := px.nodeWake[m.NodeCfg.ID().String()]; ok {
				// if there is already a wakeNode running for this node, stop it
				stop <- true
			}
			px.nodeWake[m.NodeCfg.ID().String()] = stop
			px.wakeMutex.Unlock()
			go px.wakeNode(m.NodeCfg, stop)

			px.dchan <- ev
		case "WAITtoINIT": // we're initializing, but don't do anything (more for discovery/timeout)
		}
	case core.MutationEvent_INTERRUPT: // on any interrupt, we remove the node
		v, e := m.NodeCfg.GetValue(px.cfg.IpUrl)
		if e != nil || !v.IsValid() {
			break
		}
		ip := IPv4.BytesToIP(v.Bytes())
		px.NodeDelete(queryByIP, ip.String())
	}
}

func init() {
	module := &PiPXE{}
	mutations := make(map[string]lib.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	dpxe := make(map[string]reflect.Value)
	si := core.NewServiceInstance("pipxe", module.Name(), module.Entry)

	for m := range muts {
		dur, _ := time.ParseDuration(muts[m].timeout)
		mutations[m] = core.NewStateMutation(
			map[string][2]reflect.Value{
				PxeURL: {
					reflect.ValueOf(muts[m].f),
					reflect.ValueOf(muts[m].t),
				},
			},
			reqs,
			excs,
			lib.StateMutationContext_CHILD,
			dur,
			[3]string{si.ID(), "/PhysState", "PHYS_HANG"},
		)
		dpxe[rpipb.RPi3_PXE_name[int32(muts[m].t)]] = reflect.ValueOf(muts[m].t)
	}

	mutations["WAITtoINIT"] = core.NewStateMutation(
		map[string][2]reflect.Value{
			PxeURL: {
				reflect.ValueOf(rpipb.RPi3_WAIT),
				reflect.ValueOf(rpipb.RPi3_INIT),
			},
			"/RunState": {
				reflect.ValueOf(cpb.Node_UNKNOWN),
				reflect.ValueOf(cpb.Node_INIT),
			},
		},
		reqs,
		excs,
		lib.StateMutationContext_CHILD,
		time.Second*30,
		[3]string{si.ID(), "/PhysState", "PHYS_HANG"},
	)
	dpxe["INIT"] = reflect.ValueOf(rpipb.RPi3_INIT)

	discovers[PxeURL] = dpxe
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
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
