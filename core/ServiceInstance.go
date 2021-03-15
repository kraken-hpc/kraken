/* ServiceInstance.go: provides the interface for controlling service instance processes
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I proto/src -I proto --gogo_out=Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,plugins=grpc:proto proto/src/ServiceInstance.proto

package core

import (
	"fmt"
	"os"
	"sync"

	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"

	"github.com/hpc/kraken/lib/types"
	fork "github.com/jlowellwofford/go-fork"
)

////////////////////////////
// ServiceInstance Object /
//////////////////////////

var _ types.ServiceInstance = (*ServiceInstance)(nil)

// A ServiceInstance describes a service that will be built-in to the binary and exec'ed by forking
// note: state information is stored in the node proto object, this object manages a running context
type ServiceInstance struct {
	id     string // ID must be unique
	module string // name doesn't need to be unique; we can run multiple configs of the same service
	exe    string // gets set automatically
	entry  func() // needs to run as a goroutine
	sock   string
	fork   *fork.Function
	ctl    chan<- types.ServiceControl
	wchan  chan<- types.ServiceInstanceUpdate
	state  types.ServiceState // note: these states mean a slightly different: RUN means process is running, INIT means nothing
	m      types.ModuleSelfService
	mutex  *sync.Mutex
}

// NewServiceInstance provides a new, initialized ServiceInstance object
func NewServiceInstance(id, module string, entry func()) *ServiceInstance {
	si := &ServiceInstance{
		id:     id,
		module: module,
		entry:  entry,
		fork:   nil,
		mutex:  &sync.Mutex{},
	}
	si.setState((types.Service_STOP)) // we're obviously stopped right now
	si.exe, _ = os.Executable()
	return si
}

// ID gets the ID string for the service
func (si *ServiceInstance) ID() string { return si.id }

// Module returns the name of the module this is an instance of
func (si *ServiceInstance) Module() string { return si.module }

// GetState returns the current run state of the service
func (si *ServiceInstance) GetState() types.ServiceState {
	si.mutex.Lock()
	defer si.mutex.Unlock()
	return si.state
}

// UpdateConfig will send a signal to the running si to check for a config update
func (si *ServiceInstance) UpdateConfig() {
	if si.ctl != nil {
		si.ctl <- types.ServiceControl{Command: types.ServiceControl_UPDATE}
	}
}

// Start will execute the process
func (si *ServiceInstance) Start() {
	e := si.start()
	if e != nil {
		si.setState(types.Service_ERROR)
		return
	}
	si.setState(types.Service_RUN)
	go si.watcher()
}

// Stop sends a signal to the running si to stop
func (si *ServiceInstance) Stop() {
	if si.ctl != nil {
		si.ctl <- types.ServiceControl{Command: types.ServiceControl_STOP}
	}
}

// Watch provides a channel where process state changes will be reported
func (si *ServiceInstance) Watch(wchan chan<- types.ServiceInstanceUpdate) {
	si.wchan = wchan
}

// SetCtl sets the channel to send control message to (to pass through the API)
func (si *ServiceInstance) SetCtl(ctl chan<- types.ServiceControl) {
	si.ctl = ctl
}

// SetSock sets the path to the API socket
func (si *ServiceInstance) SetSock(sock string) {
	si.sock = sock
}

// setState sets the state, but should only be done internally.  This makes sure we notify any watcher
func (si *ServiceInstance) setState(state types.ServiceState) {
	si.mutex.Lock()
	defer si.mutex.Unlock()
	si.state = state
	if si.wchan != nil {
		si.wchan <- types.ServiceInstanceUpdate{
			ID:    si.id,
			State: si.state,
		}
	}
}

func (si *ServiceInstance) watcher() {
	e := si.fork.Wait()
	if e != nil {
		si.setState(types.Service_ERROR)
		return
	}
	si.setState(types.Service_STOP)
}

func (si *ServiceInstance) start() (e error) {
	si.mutex.Lock()
	defer si.mutex.Unlock()
	if si.state == types.Service_RUN {
		return fmt.Errorf("cannot start service instance not in stop state")
	}
	if _, e = os.Stat(si.exe); os.IsNotExist(e) {
		return e
	}
	si.fork = fork.NewFork("ModuleExecute", ModuleExecute, os.Args[0], "["+si.id+"]")
	si.fork.Stdin = os.Stdin
	si.fork.Stdout = os.Stdout
	si.fork.Stderr = os.Stderr
	return si.fork.Fork(si.ID(), si.module, si.sock)
}

// moduleExecute does all of the necessary steps to start the service instance
// this is the actual entry point for a new module process
func ModuleExecute(id, module, sock string) {
	m, ok := Registry.Modules[module]
	if !ok {
		fmt.Printf("trying to launch non-existent module: %s", module)
		return
	}
	mss, ok := m.(types.ModuleSelfService)
	if !ok {
		fmt.Printf("module is not executable: %s", module)
		return
	}
	config := false
	mc, ok := m.(types.ModuleWithConfig)
	if ok {
		config = true
	}

	fmt.Printf("I am: %s\n", id)

	api := NewModuleAPIClient(sock)
	mss.Init(api)
	// call in, and get a control chan
	cc, e := api.ServiceInit(id, module)
	if e != nil {
		fmt.Printf("sock: %v\nid: %v\nmodule: %v\nerror: %v\n", os.Getenv("KRAKEN_SOCK"), os.Getenv("KRAKEN_ID"), os.Getenv("KRAKEN_MODULE"), e)
		return
	}

	// Setup logger stream
	if e = api.LoggerInit(id); e != nil {
		fmt.Printf("failed to create logger stream: %v\n", e)
		return
	}

	// Setup mutation stream if we need it
	mm, ok := m.(types.ModuleWithMutations)
	if ok {
		cc, e := api.MutationInit(id, module)
		if e != nil {
			api.Logf(ERROR, "failed to create mutation stream: %v\n", e)
			return
		}
		mm.SetMutationChan(cc)
	}

	// Setup event stream if we need it
	me, ok := m.(types.ModuleWithAllEvents)
	if ok {
		cc, e := api.EventInit(id, module)
		if e != nil {
			api.Logf(ERROR, "failed to create event stream: %v\n", e)
			return
		}
		me.SetEventsChan(cc)
	}

	// Setup discovery stream if we need it
	md, ok := m.(types.ModuleWithDiscovery)
	if ok {
		cc, e := api.DiscoveryInit(id)
		if e != nil {
			api.Logf(ERROR, "failed to create discovery stream: %v\n", e)
			return
		}
		md.SetDiscoveryChan(cc)
	}

	updateConfig := func() {
		if !config {
			api.Logf(ERROR, "tried to update config on module with no config")
			return
		}
		// Get a copy of the config
		n, _ := api.QueryRead(api.Self().String())
		srv := n.GetService(id)
		p, e := Registry.Resolve(srv.GetConfig().GetTypeUrl())
		if e != nil {
			api.Logf(ERROR, "resolve config error (%s): %v\n", srv.GetConfig().GetTypeUrl(), e)
			return
		}
		e = ptypes.UnmarshalAny(srv.GetConfig(), p)
		if e != nil {
			api.Logf(ERROR, "unmarshal config failure: %v\n", e)
			return
		}
		defaults := mc.NewConfig()
		proto.Merge(defaults, p)
		mc.UpdateConfig(defaults)
	}

	if config {
		updateConfig()
	}
	go mss.Entry()

	for {
		select {
		case cmd := <-cc:
			switch cmd.Command {
			case types.ServiceControl_STOP:
				api.Logf(NOTICE, "stopping")
				mss.Stop()
				break
			case types.ServiceControl_UPDATE:
				updateConfig()
				break
			default:
			}
		}
	}
}

func init() {
	fork.RegisterFunc("ModuleExecute", ModuleExecute)
}
