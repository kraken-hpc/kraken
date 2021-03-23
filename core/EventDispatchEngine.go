/* EventDispatchEngine.go: the EventDispatchEngine is a communications hub between pieces
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
	"sync"

	"github.com/kraken-hpc/kraken/lib/types"
)

//////////////////////////
// EventDispatchEngine Object /
////////////////////////

var _ types.EventDispatchEngine = (*EventDispatchEngine)(nil)

// EventDispatchEngine redistributes event to (possibly filtered) listeners.
type EventDispatchEngine struct {
	lists map[string]types.EventListener // place we send Events to
	schan chan types.EventListener       // where we get subscriptions
	// should we have a unique echan for each source?
	echan chan []types.Event // where we get events
	log   types.Logger
	lock  *sync.RWMutex // Make sure we don't concurrently add a listener and iterate through the list
}

// NewEventDispatchEngine creates an initialized EventDispatchEngine
func NewEventDispatchEngine(ctx Context) (v *EventDispatchEngine) {
	v = &EventDispatchEngine{
		lists: make(map[string]types.EventListener),
		schan: make(chan types.EventListener),
		echan: make(chan []types.Event),
		log:   &ctx.Logger,
		lock:  &sync.RWMutex{},
	}
	v.log.SetModule("EventDispatchEngine")
	return
}

// AddListener gets called to add a Listener; Listeners are filtered subscribers
func (v *EventDispatchEngine) AddListener(el types.EventListener) (e error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	k := el.Name()
	switch el.State() {
	case types.EventListener_UNSUBSCRIBE:
		if _, ok := v.lists[k]; ok {
			delete(v.lists, k)
		} else {
			e = fmt.Errorf("cannot unsubscribe unknown listener: %s", k)
		}
		return
	case types.EventListener_STOP:
		fallthrough
	case types.EventListener_RUN:
		// should we check validity?
		v.lists[k] = el
		return
	default:
		e = fmt.Errorf("unknown EventListener state: %d", el.State())
	}
	return
}

// SubscriptionChan gets the channel we can subscribe new EventListeners with
func (v *EventDispatchEngine) SubscriptionChan() chan<- types.EventListener { return v.schan }

// EventChan is the channel emitters should send new events on
func (v *EventDispatchEngine) EventChan() chan<- []types.Event { return v.echan }

// Run is a goroutine than handles event dispatch and subscriptions
// There's currently no way to stop this once it's started.
func (v *EventDispatchEngine) Run(ready chan<- interface{}) {
	v.Log(INFO, "starting EventDispatchEngine")
	ready <- nil
	for {
		select {
		case el := <-v.schan:
			if e := v.AddListener(el); e != nil {
				v.Logf(ERROR, "failed to add new listener: %s, %v", el.Name(), e)
			} else {
				v.Logf(DEBUG, "successfully added new listener: %s", el.Name())
			}
			break
		case e := <-v.echan:
			if len(e) == 0 {
				v.Log(ERROR, "got empty event list")
				break
			} else {
				v.Logf(DEBUG, "dispatching event: %s %s %v\n", types.EventTypeString[e[0].Type()], e[0].URL(), e[0].Data())
			}
			go v.sendEvents(e)
			break
		}
	}
}

////////////////////////
// Unexported methods /
//////////////////////

// goroutine
func (v *EventDispatchEngine) sendEvents(evs []types.Event) {
	v.lock.RLock()
	defer v.lock.RUnlock()
	for _, ev := range evs {
		for _, el := range v.lists {
			if el.Type() == types.Event_ALL {
				el.Send(ev)
			} else if ev.Type() == el.Type() {
				el.Send(ev)
			}
		}
	}
}

////////////////////////////
// Passthrough Interfaces /
//////////////////////////

/*
 * Consume Logger
 */
var _ types.Logger = (*EventDispatchEngine)(nil)

func (v *EventDispatchEngine) Log(level types.LoggerLevel, m string) { v.log.Log(level, m) }
func (v *EventDispatchEngine) Logf(level types.LoggerLevel, fmt string, va ...interface{}) {
	v.log.Logf(level, fmt, va...)
}
func (v *EventDispatchEngine) SetModule(name string)                  { v.log.SetModule(name) }
func (v *EventDispatchEngine) GetModule() string                      { return v.log.GetModule() }
func (v *EventDispatchEngine) SetLoggerLevel(level types.LoggerLevel) { v.log.SetLoggerLevel(level) }
func (v *EventDispatchEngine) GetLoggerLevel() types.LoggerLevel      { return v.log.GetLoggerLevel() }
func (v *EventDispatchEngine) IsEnabledFor(level types.LoggerLevel) bool {
	return v.log.IsEnabledFor(level)
}
