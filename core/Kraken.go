/* Kraken.go: the Kraken object orchestrates Kraken
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/types"
)

////////////////////////
// Auxilliary objects /
//////////////////////

const AddrURL = "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/0/Ip/Ip"

// Context contains information about the current running context
// such as who we are, and to whom we belong.
type Context struct {
	Logger  ServiceLogger
	Query   QueryEngine
	SubChan chan<- types.EventListener
	Self    types.NodeID
	Parents []string
	SDE     ContextSDE
	SSE     ContextSSE
	SME     ContextSME
	RPC     ContextRPC
	Sm      types.ServiceManager // API needs this
	sdqChan chan types.Query
	smqChan chan types.Query
}

type ContextSSE struct {
	Network   string
	Addr      string
	Port      int
	AddrURL   string
	HelloTime time.Duration
	DeadTime  time.Duration
}

type ContextSME struct {
	RootSpec types.StateSpec
}

type ContextRPC struct {
	Network      string
	Addr         string
	Port         int
	Path         string // path for UNIX socket
	NetListner   net.Listener
	UNIXListener net.Listener
}

type ContextSDE struct {
	InitialCfg []types.Node
	InitialDsc []types.Node
}

///////////////////
// Kraken Object /
/////////////////

var _ types.Module = (*Kraken)(nil)

//var _ types.ServiceInstance = (*Kraken)(nil)

// A Kraken is a mythical giant squid-beast.
type Kraken struct {
	Ctx Context
	Ede *EventDispatchEngine
	Sde *StateDifferenceEngine
	Sse *StateSyncEngine
	Sme *StateMutationEngine
	Api *ModuleAPIServer
	Sm  *ServiceManager

	// Un-exported
	em   *EventEmitter
	log  types.Logger
	self types.Node
}

// NewKraken creates a new Kraken object with proper intialization
func NewKraken(self types.Node, parents []string, logger types.Logger) *Kraken {
	// FIXME: we probably shouldn't rely on this
	ipv, _ := self.GetValue(AddrURL)
	ip := net.IP(ipv.Bytes())

	k := &Kraken{
		Ctx: Context{
			Self:    self.ID(),
			Parents: parents,
			Logger:  ServiceLogger{},
		},
		em:   NewEventEmitter(types.Event_CONTROL),
		log:  logger,
		self: self,
	}
	// defaults
	k.Ctx.SSE = ContextSSE{
		Network:   "udp4",
		Addr:      ip.String(),
		Port:      31415,
		AddrURL:   "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		HelloTime: 10 * time.Second,
		DeadTime:  40 * time.Second,
	}
	k.Ctx.SME = ContextSME{
		RootSpec: DefaultRootSpec(),
	}
	k.Ctx.RPC = ContextRPC{
		Network: "tcp",
		Addr:    ip.String(),
		Port:    31415,
		Path:    "/tmp/kraken.sock",
	}
	k.Ctx.SDE = ContextSDE{
		InitialCfg: []types.Node{self},
		InitialDsc: []types.Node{},
	}
	k.SetModule("kraken")
	return k
}

// implement types.ServiceInstance
// this is a little artificial, but it's a special case
// many of these would never becaused because it's not actually managed
// by ServiceManager
func (sse *Kraken) ID() string                     { return sse.Name() }
func (*Kraken) State() types.ServiceState          { return types.Service_RUN }
func (*Kraken) SetState(types.ServiceState)        {}
func (*Kraken) GetState() types.ServiceState       { return types.Service_RUN }
func (sse *Kraken) Module() string                 { return sse.Name() }
func (*Kraken) Exe() string                        { return "" }
func (*Kraken) Cmd() *exec.Cmd                     { return nil }
func (*Kraken) SetCmd(*exec.Cmd)                   {}
func (*Kraken) Stop()                              {}
func (*Kraken) SetCtl(chan<- types.ServiceControl) {}
func (*Kraken) Config() *any.Any                   { return nil }
func (*Kraken) UpdateConfig(*any.Any)              {}
func (*Kraken) Message() *pb.ServiceInstance       { return nil }

func (k *Kraken) Name() string { return "kraken" }

// Release the Kraken...
// the Kraken process itself has the core task of managing services
func (k *Kraken) Release() {
	// go get Kraken
	k.Bootstrap()
	k.Run()
}

// Bootstrap creates all service instances in the correct order
// with all of the correct plumbing
func (k *Kraken) Bootstrap() {
	k.Log(NOTICE, "releasing the Kraken...")
	k.Logf(INFO, "my ID is: %s", k.Ctx.Self.String())
	if len(k.Ctx.Parents) < 1 {
		k.Logf(INFO, "starting with no parents, I will be a full-state node")
	} else {
		for _, p := range k.Ctx.Parents {
			k.Logf(INFO, "initial parent: %s", p)
		}
	}

	extString := ""
	for e := range Registry.Extensions {
		extString += fmt.Sprintf("\n\t%s", e)
	}
	k.Logf(INFO, "this kraken is built with extensions: %s", extString)

	modString := ""
	for m := range Registry.Modules {
		modString += fmt.Sprintf("\n\t%s", m)
	}
	k.Logf(INFO, "this kraken is built with modules: %s", modString)

	// Create service instances
	k.Log(INFO, "bootstrapping core services")

	// Setup service logger
	slog := make(chan LoggerEvent)
	k.Ctx.Logger.RegisterChannel(slog)
	k.Ctx.Logger.SetLoggerLevel(k.GetLoggerLevel())
	go ServiceLoggerListener(k.log, slog)

	// setup the RPC listener, to be shared
	if e := setupRPCListener(&k.Ctx.RPC); e != nil {
		k.Logf(FATAL, "%v", e)
		os.Exit(1)
		return
	}
	k.Logf(INFO, "RPC is listening on %s:%s:%d", k.Ctx.RPC.Network, k.Ctx.RPC.Addr, k.Ctx.RPC.Port)
	k.Logf(INFO, "RPC is listening on socket %s", k.Ctx.RPC.Path)

	k.Ctx.sdqChan = make(chan types.Query)
	k.Ctx.smqChan = make(chan types.Query)

	k.Ede = NewEventDispatchEngine(k.Ctx)
	k.Ctx.SubChan = k.Ede.SubscriptionChan()
	k.Sde = NewStateDifferenceEngine(k.Ctx, k.Ctx.sdqChan)
	k.Ctx.Query = *NewQueryEngine(k.Ctx.sdqChan, k.Ctx.smqChan)
	k.Sm = NewServiceManager(k.Ctx, "unix:"+k.Ctx.RPC.Path)
	k.Ctx.Sm = k.Sm // API needs this

	k.Sse = NewStateSyncEngine(k.Ctx)
	k.Sme = NewStateMutationEngine(k.Ctx, k.Ctx.smqChan)
	k.Api = NewModuleAPIServer(k.Ctx)

	k.Sde.Subscribe("SDE", k.Ede.EventChan())
	k.Sme.Subscribe("SME", k.Ede.EventChan())
	k.Sse.Subscribe("SSE", k.Ede.EventChan())
	k.Api.Subscribe("API", k.Ede.EventChan())
}

// Run starts all services as goroutines
func (k *Kraken) Run() {
	k.Log(INFO, "starting core services")

	// each service needs to notify when it is ready
	ready := make(chan interface{})

	go k.Ede.Run(ready)
	<-ready
	k.Log(types.LLINFO, "EventDispatchEngine reported ready")
	go k.Sme.Run(ready)
	<-ready
	k.Log(types.LLINFO, "StateMutationEngine reported ready")
	go k.Sde.Run(ready)
	<-ready
	k.Log(types.LLINFO, "StateDifferenceEngine reported ready")
	go k.Sse.Run(ready)
	<-ready
	k.Log(types.LLINFO, "StateSyncEngine reported ready")
	go k.Api.Run(ready)
	<-ready
	k.Log(types.LLINFO, "API reported ready")
	go k.Sm.Run(ready)
	<-ready
	k.Log(types.LLINFO, "ServiceManager reported ready")
}

////////////////////////
// Unexported methods /
//////////////////////

func setupRPCListener(cfg *ContextRPC) (e error) {
	// Setup gRPC
	cfg.NetListner, e = net.Listen(cfg.Network, cfg.Addr+":"+strconv.Itoa(cfg.Port))
	if e != nil {
		return fmt.Errorf("listen for RPC failed: %v", e)
	}
	os.Remove(cfg.Path)
	cfg.UNIXListener, e = net.Listen("unix", cfg.Path)
	if e != nil {
		return fmt.Errorf("listen for RPC failed: %v", e)
	}
	return
}

////////////////////////////
// Passthrough Interfaces /
//////////////////////////

/*
 * Consume Logger
 */
var _ types.Logger = (*Kraken)(nil)

func (k *Kraken) Log(level types.LoggerLevel, m string) { k.log.Log(level, m) }
func (k *Kraken) Logf(level types.LoggerLevel, fmt string, v ...interface{}) {
	k.log.Logf(level, fmt, v...)
}
func (k *Kraken) SetModule(name string)                     { k.log.SetModule(name) }
func (k *Kraken) GetModule() string                         { return k.log.GetModule() }
func (k *Kraken) SetLoggerLevel(level types.LoggerLevel)    { k.log.SetLoggerLevel(level) }
func (k *Kraken) GetLoggerLevel() types.LoggerLevel         { return k.log.GetLoggerLevel() }
func (k *Kraken) IsEnabledFor(level types.LoggerLevel) bool { return k.log.IsEnabledFor(level) }

/*
 * Consume EventEmitter
 */
var _ types.EventEmitter = (*Kraken)(nil)

func (k *Kraken) Subscribe(id string, c chan<- []types.Event) error {
	return k.em.Subscribe(id, c)
}
func (k *Kraken) Unsubscribe(id string) error { return k.em.Unsubscribe(id) }
func (k *Kraken) Emit(v []types.Event)        { k.em.Emit(v) }
func (k *Kraken) EmitOne(v types.Event)       { k.em.EmitOne(v) }
func (k *Kraken) EventType() types.EventType  { return k.em.EventType() }
