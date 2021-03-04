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
	"reflect"
	"regexp"
	"sync"

	pb "github.com/hpc/kraken/core/proto"
	ct "github.com/hpc/kraken/core/proto/customtypes"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

///////////////////////////
// ServiceManager Object /
/////////////////////////

var _ types.ServiceManager = (*ServiceManager)(nil)

type ServiceManager struct {
	srv    map[string]types.ServiceInstance // map of si IDs to ServiceInstances
	mutex  *sync.Mutex
	sock   string // socket that we use for API comms
	sclist types.EventListener
	echan  chan types.Event
	wchan  chan types.ServiceInstanceUpdate
	ctx    Context
	query  *QueryEngine
	log    types.Logger
}

func NewServiceManager(ctx Context, sock string) *ServiceManager {
	sm := &ServiceManager{
		srv:   make(map[string]types.ServiceInstance),
		mutex: &sync.Mutex{},
		sock:  sock,
		echan: make(chan types.Event),
		wchan: make(chan types.ServiceInstanceUpdate),
		ctx:   ctx,
		log:   &ctx.Logger,
		query: &ctx.Query,
	}
	sm.log.SetModule("ServiceManager")
	return sm
}

func (sm *ServiceManager) Run(ready chan<- interface{}) {
	// subscribe to STATE_CHANGE events for "/Services"
	smurl := regexp.MustCompile(`^\/?Services\/`)
	sm.sclist = NewEventListener(
		"ServiceManager",
		types.Event_STATE_CHANGE,
		func(v types.Event) bool {
			node, url := util.NodeURLSplit(v.URL())
			if !ct.NewNodeID(node).EqualTo(sm.ctx.Self) {
				return false
			}
			if smurl.MatchString(url) {
				return true
			}
			return false
		},
		func(v types.Event) error { return ChanSender(v, sm.echan) },
	)
	sm.ctx.SubChan <- sm.sclist

	// initialize service instances
	for m := range Registry.ServiceInstances {
		for _, si := range Registry.ServiceInstances[m] {
			sm.log.Logf(types.LLINFO, "adding service: %s", si.ID())
			sm.AddService(si)
		}
	}

	go func() {
		sm.log.Logf(types.LLDEBUG, "starting initial service sync")
		for _, si := range sm.srv {
			sm.log.Logf(types.LLDDEBUG, "starting initial service sync: %s", si.ID())
			sm.syncService(si.ID())
		}
	}()

	// ready to go
	ready <- nil

	// main listening loop
	for {
		select {
		case v := <-sm.echan:
			// state change for services
			sm.log.Logf(types.LLDDEBUG, "processing state change event: %s", v.URL())
			sm.processStateChange(v.Data().(*StateChangeEvent))
		case su := <-sm.wchan:
			// si changed process state
			sm.log.Logf(types.LLDDEBUG, "processing SI state update: %s -> %s", su.ID, su.State)
			go sm.processUpdate(su)
		}
	}
}

func (sm *ServiceManager) AddService(si types.ServiceInstance) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	if _, ok := sm.srv[si.ID()]; ok {
		sm.log.Logf(types.LLERROR, "tried to add service that already exists: %s", si.ID())
	}
	sm.srv[si.ID()] = si
	si.Watch(sm.wchan)
	si.SetSock(sm.sock)
}

func (sm *ServiceManager) DelService(si string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	if si, ok := sm.srv[si]; ok {
		si.Watch(nil) // we don't want to watch this anymore
		si.SetSock("")
		delete(sm.srv, si.ID())
	}
}

func (sm *ServiceManager) GetService(si string) types.ServiceInstance {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	if si, ok := sm.srv[si]; ok {
		return si
	}
	return nil
}

func (sm *ServiceManager) processStateChange(v *StateChangeEvent) {
	// extract SI
	_, url := util.NodeURLSplit(v.URL)
	us := util.URLToSlice(url)
	si := ""
	// this makes sure we don't get tripped up by leading slashes
	for i := range us {
		if us[i] == "Services" {
			si = us[i+1]
		}
	}
	if si == "" {
		sm.log.Logf(types.LLDEBUG, "failed to parse URL for /Services state change: %s", v.URL)
		return
	}
	sm.syncService(si)
}

func (sm *ServiceManager) processUpdate(su types.ServiceInstanceUpdate) {
	// set the state in the SDE
	switch su.State {
	case types.Service_STOP:
		sm.setServiceStateDsc(su.ID, pb.ServiceInstance_STOP)
	case types.Service_RUN:
		// this is actually pb state INIT; it's up to
		sm.setServiceStateDsc(su.ID, pb.ServiceInstance_INIT)
	case types.Service_ERROR:
		sm.setServiceStateDsc(su.ID, pb.ServiceInstance_ERROR)
	}
}

// syncService is what actually does most of the work.  It compares cfg to dsc and decides what to do
func (sm *ServiceManager) syncService(si string) {
	sm.log.Logf(types.LLDDEBUG, "syncing service: %s", si)
	srv := sm.GetService(si)
	if srv == nil {
		sm.log.Logf(types.LLERROR, "tried to sync non-existent service: %s", si)
		return
	}
	c := sm.getServiceStateCfg(si)
	d := sm.getServiceStateDsc(si)

	if c == d { // nothing to do
		sm.log.Logf(types.LLDDDEBUG, "service already synchronized: %s (%+v == %+v)", si, c, d)
		return
	}
	if d == pb.ServiceInstance_ERROR { // don't clear errors
		return
	}
	switch c {
	case pb.ServiceInstance_RUN: // we're supposed to be running
		if d != pb.ServiceInstance_INIT { // did we already try to start?
			sm.setServiceStateDsc(si, pb.ServiceInstance_INIT)
			sm.log.Logf(types.LLDDEBUG, "starting service: %s", si)
			go srv.Start() // startup
		}
	case pb.ServiceInstance_STOP: // we're supposed to be stopped
		sm.log.Logf(types.LLDDEBUG, "stopping service: %s", si)
		srv.Stop() // stop
	}
}

// Some helper functions...

func (sm *ServiceManager) getServiceStateCfg(si string) pb.ServiceInstance_ServiceState {
	n, _ := sm.query.Read(sm.ctx.Self)
	v, e := n.GetValue(sm.stateURL(si))
	if e != nil {
		sm.log.Logf(types.LLERROR, "failed to get cfg state value (%s): %s", sm.stateURL(si), e.Error())
		return pb.ServiceInstance_UNKNOWN
	}
	return pb.ServiceInstance_ServiceState(v.Int())
}

func (sm *ServiceManager) getServiceStateDsc(si string) pb.ServiceInstance_ServiceState {
	n, _ := sm.query.ReadDsc(sm.ctx.Self)
	v, e := n.GetValue(sm.stateURL(si))
	if e != nil {
		sm.log.Logf(types.LLERROR, "failed to get dsc state value (%s): %s", sm.stateURL(si), e.Error())
		return pb.ServiceInstance_UNKNOWN
	}
	return pb.ServiceInstance_ServiceState(v.Int())
}

func (sm *ServiceManager) setServiceStateDsc(si string, state pb.ServiceInstance_ServiceState) {
	n, _ := sm.query.ReadDsc(sm.ctx.Self)
	_, e := n.SetValue(sm.stateURL(si), reflect.ValueOf(state))
	if e != nil {
		sm.log.Logf(types.LLERROR, "failed to set dsc state value (%s): %s", sm.stateURL(si), e.Error())
		return
	}
	sm.query.UpdateDsc(n)
}

func (sm *ServiceManager) stateURL(si string) string {
	return util.URLPush(util.URLPush("/Services", si), "State")
}
