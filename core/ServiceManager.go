/* ServiceManagement.go: provides management of external service modules
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
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/hpc/kraken/lib"
)

///////////////////////////
// ServiceManager Object /
/////////////////////////

var _ lib.ServiceManager = (*ServiceManager)(nil)

type ServiceManager struct {
	srv  map[string]lib.ServiceInstance
	sock string
}

func NewServiceManager(sock string) *ServiceManager {
	sm := &ServiceManager{
		srv:  make(map[string]lib.ServiceInstance),
		sock: sock,
	}
	return sm
}

func (sm *ServiceManager) AddService(s lib.ServiceInstance) (e error) {
	if _, ok := sm.srv[s.ID()]; ok {
		return fmt.Errorf("service by this ID already exists: %s", s.ID())
	}
	sm.srv[s.ID()] = s
	return
}

// AddServiceByURL adds a service that corresponds to a know module URL
func (sm *ServiceManager) AddServiceByModule(id, module string, cfg *any.Any) (e error) {
	if m, ok := Registry.Modules[module]; ok {
		if s, ok := m.(lib.ModuleSelfService); ok {
			// TODO: it would be easy to expand this so there could be multiple instances of the same service
			return sm.AddService(NewServiceInstance(id, s.Name(), cfg))
		}
		return fmt.Errorf("module is not runnable: %s", module)
	}
	return fmt.Errorf("module not found by url: %s", module)
}

func (sm *ServiceManager) DelService(id string) (e error) {
	if s, ok := sm.srv[id]; ok {
		if s.State() == lib.Service_RUN {
			return fmt.Errorf("service is running, stop before deleting: %s", id)
		}
		delete(sm.srv, id)
	}
	return fmt.Errorf("cannot delete non-existent service: %s", id)
}

func (sm *ServiceManager) Service(id string) (si lib.ServiceInstance) {
	var ok bool
	if si, ok = sm.srv[id]; ok {
		return
	}
	return nil
}

func (sm *ServiceManager) RunService(id string) (e error) {
	if s, ok := sm.srv[id]; ok {
		if s.Entry == nil { // handle non-executable (core) services
			return
		}
		if s.State() != lib.Service_RUN {
			if e = sm.start(s); e == nil {
				s.SetState(lib.Service_RUN)
			}
			return
		}
		return fmt.Errorf("service is in state, %d, cannot start", s.State())
	}
	return fmt.Errorf("cannot run non-existent service: %s", id)
}

func (sm *ServiceManager) StopService(id string) (e error) {
	if s, ok := sm.srv[id]; ok {
		if s.Entry == nil { // handle non-executable (core) services
			return
		}
		if s.State() == lib.Service_RUN {
			s.Stop()
			s.SetCmd(nil)
			return
		}
		return fmt.Errorf("service is in state, %d, cannot stop", s.State())
	}
	return fmt.Errorf("cannot stop non-existent service: %s", id)
}

func (sm *ServiceManager) GetServiceIDs() []string {
	r := []string{}
	for k := range sm.srv {
		r = append(r, k)
	}
	return r
}

func (sm *ServiceManager) syncState(s lib.ServiceInstance, state lib.ServiceState) lib.ServiceState {
	r := lib.Service_UNKNOWN
	if s.GetState() != state {
		switch state {
		case lib.Service_RUN:
			e := sm.RunService(s.ID())
			if e != nil {
			}
			r = lib.Service_INIT
			break
		case lib.Service_STOP:
			s.Stop() // we'll have 3 seconds to gracefully stop
			go func() {
				time.Sleep(3 * time.Second)
				sm.kill(s)
			}()
			r = lib.Service_STOP
			break
		default: // don't do anything
		}
	}
	return r
}

// SyncNode synchronizes service info between the state store of self and the service manager
// It reports the current discoverable state of all services
// NOTE: This is probablly very inefficient
// the returned map is what we have "discovered"
func (sm *ServiceManager) SyncNode(n lib.Node) map[string]lib.ServiceState {
	r := make(map[string]lib.ServiceState)
	sids := sm.GetServiceIDs()
	for _, srv := range n.GetServices() {
		for i, s := range sids {
			if s == srv.ID() {
				sids = append(sids[:i], sids[i+1:]...)
			}
		}
		if s, ok := sm.srv[srv.ID()]; ok {
			if !proto.Equal(s.Config(), srv.Config()) {
				s.UpdateConfig(srv.Config())
			}
			if ss := sm.syncState(s, srv.State()); ss != lib.Service_UNKNOWN {
				r[srv.ID()] = ss
			}
		} else {
			e := sm.AddServiceByModule(srv.ID(), srv.Module(), srv.Config())
			if e != nil {
				return r
			}
			if ss := sm.syncState(sm.srv[srv.ID()], srv.State()); ss != lib.Service_UNKNOWN {
				r[srv.ID()] = ss
			}
		}
	}
	// sids is now a list of services we're not supposed to have anymore
	for _, id := range sids {
		sm.srv[id].Stop()
		go func() {
			time.Sleep(3 * time.Second)
			sm.kill(sm.srv[id])
		}()
		sm.DelService(id)
	}
	return r
}

func (sm *ServiceManager) start(s lib.ServiceInstance) (e error) {
	if s.State() == lib.Service_RUN {
		return fmt.Errorf("service is already running")
	}
	if _, e = os.Stat(s.Exe()); os.IsNotExist(e) {
		return e
	}
	// TODO: we should probably do more sanity checks here...
	cmd := exec.Command(s.Exe())
	cmd.Args = []string{"[kraken:" + s.ID() + "]"}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"KRAKEN_SOCK="+sm.sock,
		"KRAKEN_MODULE="+s.Module(),
		"KRAKEN_ID="+s.ID())
	e = cmd.Start()
	s.SetCmd(cmd)
	s.SetState(lib.Service_RUN)
	return
}

func (sm *ServiceManager) kill(s lib.ServiceInstance) (e error) {
	cmd := s.Cmd()
	if cmd == nil || cmd.Process == nil {
		return
	}
	/*
		if s.State() != lib.Service_RUN {
			return fmt.Errorf("cannot kill a process that isn't running")
		}
	*/
	s.SetState(lib.Service_STOP)
	cmd.Process.Kill()
	cmd.Process.Wait()
	return cmd.Process.Release()
}
