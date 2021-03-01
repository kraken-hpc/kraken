/* libvirt.go: mutations for libvirt using the go-libvirt via RPC interface of local socket
 *
 * Author: Benjamin S. Allen <bsallen@alcf.anl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. libvirt.proto

/*
 * This module will manipulate the PhysState state field.
 * It will be restricted to Platform = libvirt.
 */

package libvirt

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"time"

	lv "github.com/digitalocean/go-libvirt"
	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

// PlatformString denotes what node platform this modules supports
const PlatformString string = "libvirt"

// ppmut helps us succinctly define our mutations
type ppmut struct {
	from    cpb.Node_PhysState
	to      cpb.Node_PhysState
	timeout string // timeout
	// everything fails to PHYS_HANG
}

// Mutation definitions
// also we discover anything we can migrate to
var muts = map[string]ppmut{
	"UKtoOFF": {
		from:    cpb.Node_PHYS_UNKNOWN,
		to:      cpb.Node_POWER_OFF,
		timeout: "10s",
	},
	"OFFtoON": {
		from:    cpb.Node_POWER_OFF,
		to:      cpb.Node_POWER_ON,
		timeout: "10s",
	},
	"ONtoOFF": {
		from:    cpb.Node_POWER_ON,
		to:      cpb.Node_POWER_OFF,
		timeout: "10s",
	},
	"HANGtoOFF": {
		from:    cpb.Node_PHYS_HANG,
		to:      cpb.Node_POWER_OFF,
		timeout: "20s", // we need a longer timeout, because we let it sit cold for a few seconds
	},
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Platform": reflect.ValueOf(PlatformString),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

////////////////////
// Virt Object /
//////////////////

// Virt provides a power on/off interface to the LibVirt interface
type Virt struct {
	api        types.ModuleAPIClient
	cfg        *Config
	mchan      <-chan types.Event
	dchan      chan<- types.Event
	pollTicker *time.Ticker
	srvConns   map[string]*lv.Libvirt
}

/*
 *types.Module
 */
var _ types.Module = (*Virt)(nil)

// Name returns the FQDN of the module
func (*Virt) Name() string { return "github.com/hpc/kraken/modules/libvirt" }

/*
 * types.ModuleWithConfig
 */
var _ types.ModuleWithConfig = (*Virt)(nil)

// NewConfig returns a fully initialized default config
func (*Virt) NewConfig() proto.Message {
	defConf := &Config{
		ServerUrl: "type.googleapis.com/LibVirt.VirtualMachine/Server",
		NameUrl:   "type.googleapis.com/LibVirt.VirtualMachine/VmName",
		UuidUrl:   "type.googleapis.com/LibVirt.VirtualMachine/Uuid",
		Servers: map[string]*Server{
			"socket": {
				Name:       "socket",
				SocketPath: "/var/run/libvirt/libvirt-sock",
			},
		},
		PollingInterval: "30s",
	}
	return defConf
}

// UpdateConfig updates the running config
func (v *Virt) UpdateConfig(cfg proto.Message) error {
	if virtCfg, ok := cfg.(*Config); ok {
		v.cfg = virtCfg
		if v.pollTicker != nil {
			v.pollTicker.Stop()
			dur, _ := time.ParseDuration(v.cfg.GetPollingInterval())
			v.pollTicker = time.NewTicker(dur)
		}
		return nil
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*Virt) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * types.ModuleWithMutations & types.ModuleWithDiscovery
 */
var _ types.ModuleWithMutations = (*Virt)(nil)
var _ types.ModuleWithDiscovery = (*Virt)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (v *Virt) SetMutationChan(c <-chan types.Event) { v.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (v *Virt) SetDiscoveryChan(c chan<- types.Event) { v.dchan = c }

/*
 * types.ModuleSelfService
 */
var _ types.ModuleSelfService = (*Virt)(nil)

// Init is used to intialize an executable module prior to entrypoint
func (v *Virt) Init(api types.ModuleAPIClient) {
	v.api = api
	v.cfg = v.NewConfig().(*Config)
}

// Entry is the LibVirt module's executable entrypoint
func (v *Virt) Entry() {
	url := util.NodeURLJoin(v.api.Self().String(), util.URLPush(util.URLPush("/Services", "libvirt"), "State"))
	v.dchan <- core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)
	// setup a ticker for polling discovery
	dur, _ := time.ParseDuration(v.cfg.GetPollingInterval())
	v.pollTicker = time.NewTicker(dur)

	// main loop
	for {
		select {
		case <-v.pollTicker.C:
			go v.discoverAll()
			break
		case m := <-v.mchan: // mutation request
			go v.handleMutation(m)
			break
		}
	}
}

// Stop attempts to perform a graceful exit, closing all connections, before exiting.
func (v *Virt) Stop() {
	for _, conn := range v.srvConns {
		_ = conn.ConnectClose()
	}
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (v *Virt) handleMutation(m types.Event) {
	if m.Type() != types.Event_STATE_MUTATION {
		v.api.Log(types.LLINFO, "got an unexpected event type on mutation channel")
	}
	me := m.Data().(*core.MutationEvent)
	// extract the mutating node's name and server
	values, err := me.NodeCfg.GetValues([]string{v.cfg.GetNameUrl(), v.cfg.GetServerUrl()})
	if err != nil {
		v.api.Logf(types.LLERROR, "error getting values for node: %v", err)
	}
	if len(values) != 2 {
		v.api.Logf(types.LLERROR, "could not get Node ID and/or LibVirt server for node: %s", me.NodeCfg.ID().String())
		return
	}
	name := values[v.cfg.GetNameUrl()].String()
	srvName := values[v.cfg.GetServerUrl()].String()

	libVirtConn := v.connectToServer(srvName)
	if libVirtConn == nil {
		v.api.Logf(types.LLERROR, "could not fetch LibVirt connection for server: %s", srvName)
	}

	// mutation switch
	switch me.Type {
	case core.MutationEvent_MUTATE:
		switch me.Mutation[1] {
		case "UKtoOFF": // this just forces discovery
			go v.vmDiscover(name, me.NodeCfg.ID(), libVirtConn)
		case "OFFtoON":
			go v.vmOn(name, me.NodeCfg.ID(), libVirtConn)
		case "ONtoOFF":
			go v.vmOff(name, me.NodeCfg.ID(), libVirtConn)
		case "HANGtoOFF":
			go v.vmOff(name, me.NodeCfg.ID(), libVirtConn)
			break
		case "UKtoHANG": // we don't actually do this
			fallthrough
		default:
			v.api.Logf(types.LLDEBUG, "unexpected event: %s", me.Mutation[1])
		}
		break
	case core.MutationEvent_INTERRUPT:
		// nothing to do
		break
	}
}

// discoverAll is used to do polling based discovery of power state
func (v *Virt) discoverAll() {
	v.api.Log(types.LLDEBUG, "polling for node state")
	ns, e := v.api.QueryReadAll()
	if e != nil {
		v.api.Logf(types.LLERROR, "polling node query failed: %v", e)
		return
	}
	idmap := make(map[string]types.NodeID)
	bySrv := make(map[string][]string)

	// build lists
	for _, n := range ns {
		vs, e := n.GetValues([]string{"/Platform", v.cfg.GetNameUrl(), v.cfg.GetServerUrl()})
		if e != nil {
			v.api.Logf(types.LLERROR, "error getting values for node: %v", e)
		}
		if len(vs) != 3 {
			v.api.Logf(types.LLDEBUG, "skipping node %s, doesn't have complete LibVirt info", n.ID().String())
			continue
		}
		if vs["/Platform"].String() != PlatformString {
			continue
		}
		name := vs[v.cfg.GetNameUrl()].String()
		srv := vs[v.cfg.GetServerUrl()].String()
		idmap[name] = n.ID()
		bySrv[srv] = append(bySrv[srv], name)
	}

	for srvName, names := range bySrv {
		libVirtConn := v.connectToServer(srvName)
		if libVirtConn == nil {
			continue
		}

		for _, name := range names {
			v.vmDiscover(name, idmap[name], libVirtConn)
		}
	}
}

// connectToServer dials TCP connection and connects to the libvirt server
// returning a pointer to a libvirt.Libvirt with an active connection.
// On errors it will log and return nil.
func (v *Virt) connectToServer(srvName string) *lv.Libvirt {
	srv, ok := v.cfg.Servers[srvName]
	if !ok {
		v.api.Logf(types.LLERROR, "cannot control power via unknown LibVirt server: %s", srvName)
		return nil
	}

	// Check if there is an existing connection, if so just return it
	if libVirtConn, ok := v.srvConns[srvName]; ok {
		return libVirtConn
	}

	// Setup Connection to LibVirt, for now only supporting socket connections
	var c net.Conn
	if srv.SocketPath != "" {
		var err error
		if c, err = net.DialTimeout("unix", srv.SocketPath, 2*time.Second); err != nil {
			v.api.Logf(types.LLERROR, "failed to dial libvirt server: %v", err)
			return nil
		}
	} else {
		v.api.Logf(types.LLERROR, "failed to dial libvirt server, only socket connections supported")
		return nil
	}

	libVirtConn := lv.New(c)
	if err := libVirtConn.Connect(); err != nil {
		v.api.Logf(types.LLERROR, "failed to connect to libvirt server: %v", err)
		return nil
	}

	v.srvConns[srvName] = libVirtConn
	return libVirtConn
}

func (v *Virt) vmDiscover(name string, id types.NodeID, libVirtConn *lv.Libvirt) {
	dom, err := libVirtConn.DomainLookupByName(name)
	if err != nil {
		v.api.Logf(types.LLERROR, "failed to retrieve domain: %s, %v", dom.Name, err)
		return
	}

	state, _, err := libVirtConn.DomainGetState(dom, 0)
	if err != nil {
		v.api.Logf(types.LLERROR, "failed to get state of domain: %s, %v", dom.Name, err)
		state = int32(lv.DomainNostate)
	}

	url := util.NodeURLJoin(id.String(), "/PhysState")
	event := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: domainState(lv.DomainState(state)),
		},
	)
	v.dchan <- event
}

// domainState translate libvirt.DomainState int32 Enum into event values
func domainState(state lv.DomainState) string {
	switch state {
	case lv.DomainNostate:
		return "PHYS_UNKNOWN"
	case lv.DomainRunning:
		return "POWER_ON"
	case lv.DomainBlocked:
		return "PHYS_UNKNOWN"
	case lv.DomainPaused:
		return "POWER_OFF"
	case lv.DomainShutdown:
		return "POWER_OFF"
	case lv.DomainShutoff:
		return "POWER_OFF"
	case lv.DomainCrashed:
		return "PHYS_UNKNOWN"
	case lv.DomainPmsuspended:
		return "POWER_OFF"
	}
	return "PHYS_UNKNOWN"
}

func (v *Virt) vmOn(name string, id types.NodeID, libVirtConn *lv.Libvirt) {
	dom, err := libVirtConn.DomainLookupByName(name)
	if err != nil {
		v.api.Logf(types.LLERROR, "failed to retrieve domain: %s, %v", dom.Name, err)
		return
	}

	if err := libVirtConn.DomainCreate(dom); err != nil {
		v.api.Logf(types.LLERROR, "failed to create domain: %s, %v", dom.Name, err)
	}

	url := util.NodeURLJoin(id.String(), "/PhysState")
	event := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "POWER_ON",
		},
	)
	v.dchan <- event
}

func (v *Virt) vmOff(name string, id types.NodeID, libVirtConn *lv.Libvirt) {
	dom, err := libVirtConn.DomainLookupByName(name)
	if err != nil {
		v.api.Logf(types.LLERROR, "failed to retrieve domain: %s, %v", dom.Name, err)
		return
	}

	if err := libVirtConn.DomainShutdown(dom); err != nil {
		v.api.Logf(types.LLERROR, "failed to create domain: %s, %v", dom.Name, err)
	}

	url := util.NodeURLJoin(id.String(), "/PhysState")
	event := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "POWER_OFF",
		},
	)
	v.dchan <- event
}

// initialization
func init() {
	module := &Virt{}
	mutations := make(map[string]types.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	drstate := make(map[string]reflect.Value)
	si := core.NewServiceInstance("libvirt", module.Name(), module.Entry)

	for m := range muts {
		dur, _ := time.ParseDuration(muts[m].timeout)
		mutations[m] = core.NewStateMutation(
			map[string][2]reflect.Value{
				"/PhysState": {
					reflect.ValueOf(muts[m].from),
					reflect.ValueOf(muts[m].to),
				},
			},
			reqs,
			excs,
			types.StateMutationContext_CHILD,
			dur,
			[3]string{si.ID(), "/PhysState", "PHYS_HANG"},
		)
		drstate[cpb.Node_PhysState_name[int32(muts[m].to)]] = reflect.ValueOf(muts[m].to)
	}
	discovers["/PhysState"] = drstate
	discovers["/PhysState"]["PHYS_UNKNOWN"] = reflect.ValueOf(cpb.Node_PHYS_UNKNOWN)
	discovers["/PhysState"]["PHYS_HANG"] = reflect.ValueOf(cpb.Node_PHYS_HANG)
	discovers["/RunState"] = map[string]reflect.Value{
		"RUN_UK": reflect.ValueOf(cpb.Node_UNKNOWN),
	}
	discovers["/Services/libvirt/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
	core.Registry.RegisterMutations(si, mutations)
}
