/* EventEmitter.go: event emitters can send events to EventDispatchEngine for distribution
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

	"github.com/kraken-hpc/kraken/lib/types"
)

/////////////////////////
// EventEmitter Object /
///////////////////////

var _ types.EventEmitter = (*EventEmitter)(nil)

// EventEmitter is really just a helper object, and not a core type.
// It simplifies making an engine that Emits events to event dispatch.
type EventEmitter struct {
	subs map[string]chan<- []types.Event
	t    types.EventType
}

// NewEventEmitter creates a new initialized EventEmitter.
// It must be Subscribed to do anything interesting.
func NewEventEmitter(t types.EventType) *EventEmitter {
	ne := &EventEmitter{
		subs: make(map[string]chan<- []types.Event),
		t:    t,
	}
	return ne
}

// Subscribe links the Emitter to an Event chan, allowing it to actually send events.
func (m *EventEmitter) Subscribe(id string, c chan<- []types.Event) (e error) {
	if _, ok := m.subs[id]; ok {
		e = fmt.Errorf("subscription id already in use: %s", id)
		return
	}
	m.subs[id] = c
	return
}

// Unsubscribe removes an event chan from the subscriber list
func (m *EventEmitter) Unsubscribe(id string) (e error) {
	if _, ok := m.subs[id]; !ok {
		e = fmt.Errorf("cannot unsubscribe, no such subscription: %s", id)
		return
	}
	delete(m.subs, id)
	return
}

// EventType returns the event type that this Emitter sends
func (m *EventEmitter) EventType() types.EventType { return m.t }

// Emit emits (non-blocking) a slice of Events
// NOT a goroutine; handles that internally
func (m *EventEmitter) Emit(v []types.Event) {
	go m.emit(v)
}

// EmitOne is a helper for when we have a single event
func (m *EventEmitter) EmitOne(v types.Event) {
	m.Emit([]types.Event{v})
}

////////////////////////
// Unexported methods /
//////////////////////

func (m *EventEmitter) emit(v []types.Event) {
	for _, s := range m.subs {
		s <- v // should we introduce timeouts?
	}
}
