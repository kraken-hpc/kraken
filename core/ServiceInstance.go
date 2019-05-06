/* ServiceInstance.go: provides the ServiceInstance object
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
	"os"
	"os/exec"

	"github.com/golang/protobuf/ptypes"

	"github.com/golang/protobuf/ptypes/any"
	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
)

////////////////////////////
// ServiceInstance Object /
//////////////////////////

var _ lib.ServiceInstance = (*ServiceInstance)(nil)

// A ServiceInstance describes a service that will be built-in to the binary and exec'ed by forking
type ServiceInstance struct {
	id     string // ID must be unique
	module string // name doesn't need to be unique; we can run multiple configs of the same service
	exe    string // gets set automatically
	sock   string
	cmd    *exec.Cmd
	cfg    *any.Any
	ctl    chan<- lib.ServiceControl
	state  lib.ServiceState
	m      lib.ModuleSelfService
}

// NewServiceInstance provides a new, initialized ServiceInstance object
func NewServiceInstance(id, module string, cfg *any.Any) *ServiceInstance {
	ss := &ServiceInstance{
		id:     id,
		module: module,
		cmd:    nil,
		cfg:    cfg,
		state:  lib.Service_UNKNOWN,
	}
	ss.exe, _ = os.Executable()
	return ss
}

func NewServiceInstanceFromMessage(m *pb.ServiceInstance) *ServiceInstance {
	var f func()
	if mod, ok := Registry.Modules[m.Module]; ok {
		if modss, ok := mod.(lib.ModuleSelfService); ok {
			f = modss.Entry
		}
	}
	si := NewServiceInstance(m.Id, m.Module, f, m.Config)
	si.SetState(lib.ServiceState(m.State))
	return si
}

// ID gets the ID string for the service
func (ss *ServiceInstance) ID() string { return ss.id }

func (ss *ServiceInstance) State() lib.ServiceState {
	return ss.state
}

func (ss *ServiceInstance) SetState(state lib.ServiceState) {
	ss.state = state
}

// GetState returns the current run state of the service
// This is only really relevant on a local node
func (ss *ServiceInstance) GetState() lib.ServiceState {
	if ss.cmd == nil {
		return lib.Service_STOP
	}
	if ss.cmd.Process != nil {
		//fmt.Printf("cmd.Process.Pid: %d\n", ss.cmd.Process.Pid)
		// is it running?
		p, _ := os.FindProcess(ss.cmd.Process.Pid)
		if p.Pid < 1 { // we get a negative pid for non-existent
			return lib.Service_STOP
		}
	}
	return lib.Service_RUN
}

// Name returns the service name (i.e. Args[0])
func (ss *ServiceInstance) Module() string { return ss.module }

// Exe returns the path to the executable for this service (usually self)
func (ss *ServiceInstance) Exe() string { return ss.exe }

// Cmd returns the current command context, if any
func (ss *ServiceInstance) Cmd() *exec.Cmd { return ss.cmd }

// SetCmd registers the current command execution context
func (ss *ServiceInstance) SetCmd(c *exec.Cmd) { ss.cmd = c }

func (ss *ServiceInstance) SetCtl(c chan<- lib.ServiceControl) { ss.ctl = c }

// Config returns the current config
func (ss *ServiceInstance) Config() *any.Any { return ss.cfg }

// UpdateConfig will update the module config, and send an update to it if it's running
func (ss *ServiceInstance) UpdateConfig(cfg *any.Any) {
	ss.cfg = cfg
	if ss.ctl != nil {
		ss.ctl <- lib.ServiceControl{Command: lib.ServiceControl_UPDATE, Config: ss.cfg}
	}
}

func (ss *ServiceInstance) Message() *pb.ServiceInstance {
	si := &pb.ServiceInstance{
		Id:     ss.ID(),
		Module: ss.Module(),
		State:  pb.ServiceInstance_ServiceState(ss.State()),
		Config: ss.Config(),
	}
	return si
}

func (ss *ServiceInstance) Stop() {
	ss.SetState(lib.Service_STOP)
	if ss.ctl != nil {
		ss.ctl <- lib.ServiceControl{Command: lib.ServiceControl_STOP}
		ss.cmd.Process.Wait()
	}
}

// ModuleExecute does all of the necessary steps to start the service instance
// This includes getting the API ready
// It assumes certain things are in the OS environment
// This may belong somewhere else.
func ModuleExecute(id, module, sock string) {
	m, ok := Registry.Modules[module]
	if !ok {
		fmt.Printf("trying to launch non-existent module: %s", module)
		return
	}
	mss, ok := m.(lib.ModuleSelfService)
	if !ok {
		fmt.Printf("module is not executable: %s", module)
		return
	}
	config := false
	mc, ok := m.(lib.ModuleWithConfig)
	if ok {
		config = true
	}

	api := NewAPIClient(sock)
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
	mm, ok := m.(lib.ModuleWithMutations)
	if ok {
		cc, e := api.MutationInit(id, module)
		if e != nil {
			api.Logf(ERROR, "failed to create mutation stream: %v\n", e)
			return
		}
		mm.SetMutationChan(cc)
	}

	// Setup discovery stream if we need it
	md, ok := m.(lib.ModuleWithDiscovery)
	if ok {
		cc, e := api.DiscoveryInit()
		if e != nil {
			api.Logf(ERROR, "failed to create discovery stream: %v\n", e)
			return
		}
		md.SetDiscoveryChan(cc)
	}

	go func() {
		for {
			cmd := <-cc
			if e != nil {
				os.Exit(0)
			}
			switch cmd.Command {
			case lib.ServiceControl_STOP:
				os.Exit(0)
				break
			case lib.ServiceControl_UPDATE:
				if !config {
					api.Logf(ERROR, "tried to update config on module with no config")
					break
				}
				p, e := Registry.Resolve(cmd.Config.GetTypeUrl())
				if e != nil {
					api.Logf(ERROR, "resolve config error: %v\n", e)
					break
				}
				e = ptypes.UnmarshalAny(cmd.Config, p)
				if e != nil {
					api.Logf(ERROR, "unmarshal config failure: %v\n", e)
					break
				}
				mc.UpdateConfig(p)
				break
			default:
			}
		}
	}()
	mss.Entry()
}
