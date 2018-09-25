/* EventDispatchEngine.go: the EventDispatchEngine is a communications hub between pieces
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"fmt"

	"github.com/hpc/kraken/lib"
)

//////////////////////////
// EventDispatchEngine Object /
////////////////////////

var _ lib.EventDispatchEngine = (*EventDispatchEngine)(nil)

// EventDispatchEngine redistributes event to (possibly filtered) listeners.
type EventDispatchEngine struct {
	lists map[string]lib.EventListener // place we send Events to
	schan chan lib.EventListener       // where we get subscriptions
	// should we have a unique echan for each source?
	echan chan []lib.Event // where we get events
	log   lib.Logger
}

// NewEventDispatchEngine creates an initialized EventDispatchEngine
func NewEventDispatchEngine(ctx Context) (v *EventDispatchEngine) {
	v = &EventDispatchEngine{}
	v.lists = make(map[string]lib.EventListener)
	v.schan = make(chan lib.EventListener)
	v.echan = make(chan []lib.Event)
	v.log = &ctx.Logger
	v.log.SetModule("EventDispatchEngine")
	return
}

// AddListener gets called to add a Listener; Listeners are filtered subscribers
func (v *EventDispatchEngine) AddListener(el lib.EventListener) (e error) {
	k := el.Name()
	switch el.State() {
	case lib.EventListener_UNSUBSCRIBE:
		if _, ok := v.lists[k]; ok {
			delete(v.lists, k)
		} else {
			e = fmt.Errorf("cannot unsubscribe unknown listener: %s", k)
		}
		return
	case lib.EventListener_STOP:
		fallthrough
	case lib.EventListener_RUN:
		// should we check validity?
		v.lists[k] = el
		return
	default:
		e = fmt.Errorf("unknown EventListener state: %d", el.State())
	}
	return
}

// SubscriptionChan gets the channel we can subscribe new EventListeners with
func (v *EventDispatchEngine) SubscriptionChan() chan<- lib.EventListener { return v.schan }

// EventChan is the channel emitters should send new events on
func (v *EventDispatchEngine) EventChan() chan<- []lib.Event { return v.echan }

// Run is a goroutine than handles event dispatch and subscriptions
// There's currently no way to stop this once it's started.
func (v *EventDispatchEngine) Run() {
	v.Log(INFO, "starting EventDispatchEngine")
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
				v.Logf(DEBUG, "dispatching event: %v %v %v\n", e[0].Type(), e[0].URL(), e[0].Data())
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
func (v *EventDispatchEngine) sendEvents(evs []lib.Event) {
	for _, ev := range evs {
		for _, el := range v.lists {
			if ev.Type() == el.Type() {
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
var _ lib.Logger = (*EventDispatchEngine)(nil)

func (v *EventDispatchEngine) Log(level lib.LoggerLevel, m string) { v.log.Log(level, m) }
func (v *EventDispatchEngine) Logf(level lib.LoggerLevel, fmt string, va ...interface{}) {
	v.log.Logf(level, fmt, va...)
}
func (v *EventDispatchEngine) SetModule(name string)                { v.log.SetModule(name) }
func (v *EventDispatchEngine) GetModule() string                    { return v.log.GetModule() }
func (v *EventDispatchEngine) SetLoggerLevel(level lib.LoggerLevel) { v.log.SetLoggerLevel(level) }
func (v *EventDispatchEngine) GetLoggerLevel() lib.LoggerLevel      { return v.log.GetLoggerLevel() }
func (v *EventDispatchEngine) IsEnabledFor(level lib.LoggerLevel) bool {
	return v.log.IsEnabledFor(level)
}
