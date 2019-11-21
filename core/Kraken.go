/* Kraken.go: the Kraken object orchestrates Kraken
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
)

////////////////////////
// Auxilliary objects /
//////////////////////

// Context contains information about the current running context
// such as who we are, and to whom we belong.
type Context struct {
	Services *ServiceManager
	Logger   ServiceLogger
	Query    QueryEngine
	SubChan  chan<- lib.EventListener
	Self     lib.NodeID
	Parents  []string
	SSE      ContextSSE
	SME      ContextSME
	RPC      ContextRPC
	sdqChan  chan lib.Query
	smqChan  chan lib.Query
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
	RootSpec lib.StateSpec
}

type ContextRPC struct {
	Network      string
	Addr         string
	Port         int
	Path         string // path for UNIX socket
	NetListner   net.Listener
	UNIXListener net.Listener
}

///////////////////
// Kraken Object /
/////////////////

var _ lib.Module = (*Kraken)(nil)
var _ lib.ServiceInstance = (*Kraken)(nil)

// A Kraken is a mythical giant squid-beast.
type Kraken struct {
	Ctx Context
	Ede *EventDispatchEngine
	Sde *StateDifferenceEngine
	Sse *StateSyncEngine
	Sme *StateMutationEngine
	Api *APIServer

	// Un-exported
	em  *EventEmitter
	log *WriterLogger
}

// NewKraken creates a new Kracken object with proper intialization
func NewKraken(id string, ip string, parents []string, llevel lib.LoggerLevel) *Kraken {
	k := &Kraken{
		Ctx: Context{
			Self:    NewNodeID(id),
			Parents: parents,
			Logger:  ServiceLogger{},
		},
		em:  NewEventEmitter(lib.Event_CONTROL),
		log: &WriterLogger{},
	}
	// defaults
	k.Ctx.SSE = ContextSSE{
		Network:   "udp4",
		Addr:      ip,
		Port:      31415,
		AddrURL:   "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		HelloTime: 10 * time.Second,
		DeadTime:  40 * time.Second,
	}
	k.Ctx.SME = ContextSME{
		RootSpec: DefaultRootSpec(),
	}
	k.Ctx.RPC = ContextRPC{
		Network: "tcp",
		Addr:    ip,
		Port:    31415,
		Path:    "/tmp/kraken.sock",
	}
	k.log.SetLoggerLevel(llevel)
	return k
}

// implement lib.ServiceInstance
// this is a little artificial, but it's a special case
// many of these would never becaused because it's not actually managed
// by ServiceManager
func (sse *Kraken) ID() string                   { return sse.Name() }
func (*Kraken) State() lib.ServiceState          { return lib.Service_RUN }
func (*Kraken) SetState(lib.ServiceState)        {}
func (*Kraken) GetState() lib.ServiceState       { return lib.Service_RUN }
func (sse *Kraken) Module() string               { return sse.Name() }
func (*Kraken) Exe() string                      { return "" }
func (*Kraken) Cmd() *exec.Cmd                   { return nil }
func (*Kraken) SetCmd(*exec.Cmd)                 {}
func (*Kraken) Stop()                            {}
func (*Kraken) SetCtl(chan<- lib.ServiceControl) {}
func (*Kraken) Config() *any.Any                 { return nil }
func (*Kraken) UpdateConfig(*any.Any)            {}
func (*Kraken) Message() *pb.ServiceInstance     { return nil }

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
	// Setup logger
	k.log.RegisterWriter(os.Stderr)
	k.SetModule("kraken")
	//k.SetLoggerLevel(lib.LLDEBUG)

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

	k.Ctx.sdqChan = make(chan lib.Query)
	k.Ctx.smqChan = make(chan lib.Query)

	k.Ede = NewEventDispatchEngine(k.Ctx)
	k.Ctx.SubChan = k.Ede.SubscriptionChan()
	k.Sde = NewStateDifferenceEngine(k.Ctx, k.Ctx.sdqChan)
	k.Ctx.Services = NewServiceManager("unix:" + k.Ctx.RPC.Path)
	k.Ctx.Query = *NewQueryEngine(k.Ctx.sdqChan, k.Ctx.smqChan)

	k.Sse = NewStateSyncEngine(k.Ctx)
	k.Sme = NewStateMutationEngine(k.Ctx, k.Ctx.smqChan)
	k.Api = NewAPIServer(k.Ctx)

	k.Sde.Subscribe("SDE", k.Ede.EventChan())
	k.Sme.Subscribe("SME", k.Ede.EventChan())
	k.Sse.Subscribe("SSE", k.Ede.EventChan())
	k.Api.Subscribe("API", k.Ede.EventChan())
}

// Run starts all services as goroutines
func (k *Kraken) Run() {
	k.Log(INFO, "starting core services")

	go k.Ede.Run()
	go k.Sde.Run()
	go k.Sse.Run()
	go k.Sme.Run()
	go k.Api.Run()

	if len(k.Ctx.Parents) < 1 {
		// set our own runstate and phystate, should we discover these instead?
		n, _ := k.Ctx.Query.ReadDsc(k.Ctx.Self)
		dn, _ := k.Ctx.Query.ReadDsc(k.Ctx.Self)
		n.SetValue("/PhysState", reflect.ValueOf(pb.Node_POWER_ON))
		dn.SetValue("/PhysState", reflect.ValueOf(pb.Node_POWER_ON))
		n.SetValue("/RunState", reflect.ValueOf(pb.Node_SYNC))
		dn.SetValue("/RunState", reflect.ValueOf(pb.Node_SYNC))
		k.Ctx.Query.UpdateDsc(dn)
		k.Ctx.Query.Update(n)
	}
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
var _ lib.Logger = (*Kraken)(nil)

func (k *Kraken) Log(level lib.LoggerLevel, m string) { k.log.Log(level, m) }
func (k *Kraken) Logf(level lib.LoggerLevel, fmt string, v ...interface{}) {
	k.log.Logf(level, fmt, v...)
}
func (k *Kraken) SetModule(name string)                   { k.log.SetModule(name) }
func (k *Kraken) GetModule() string                       { return k.log.GetModule() }
func (k *Kraken) SetLoggerLevel(level lib.LoggerLevel)    { k.log.SetLoggerLevel(level) }
func (k *Kraken) GetLoggerLevel() lib.LoggerLevel         { return k.log.GetLoggerLevel() }
func (k *Kraken) IsEnabledFor(level lib.LoggerLevel) bool { return k.log.IsEnabledFor(level) }

/*
 * Consume EventEmitter
 */
var _ lib.EventEmitter = (*Kraken)(nil)

func (k *Kraken) Subscribe(id string, c chan<- []lib.Event) error {
	return k.em.Subscribe(id, c)
}
func (k *Kraken) Unsubscribe(id string) error { return k.em.Unsubscribe(id) }
func (k *Kraken) Emit(v []lib.Event)          { k.em.Emit(v) }
func (k *Kraken) EmitOne(v lib.Event)         { k.em.EmitOne(v) }
func (k *Kraken) EventType() lib.EventType    { return k.em.EventType() }
