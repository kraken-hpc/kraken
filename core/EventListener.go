/* EventListener.go: event listeners listen for events in dispatch.  They include filtering.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"regexp"

	"github.com/kraken-hpc/kraken/lib/types"
)

///////////////////////
// Auxiliary Objects /
/////////////////////

/*
 * Filter generators - for URL based filtering
 */

// FilterSimple is mostly for an example; it's not very useful
func FilterSimple(ev types.Event, match []string) (r bool) {
	for _, v := range match {
		if ev.URL() == v {
			r = true
		}
	}
	return
}

// FilterRegexpStr matches URL to a regexp (string)
// FilterRegexp is probably more efficient for repeated filtering
func FilterRegexpStr(ev types.Event, re string) (r bool) {
	r, e := regexp.Match(re, []byte(ev.URL()))
	if e != nil {
		r = false
	}
	return
}

// FilterRegexp matches URL to a compiled Regexp
func FilterRegexp(ev types.Event, re *regexp.Regexp) (r bool) {
	return re.Match([]byte(ev.URL()))
}

/*
 * Sender generators
 */

// ChanSender is for the simple case were we just retransmit on a chan
func ChanSender(ev types.Event, c chan<- types.Event) error {
	c <- ev
	return nil
}

//////////////////////////
// EventListener Object /
////////////////////////

var _ types.EventListener = (*EventListener)(nil)

// An EventListener implementation that leaves filter/send as arbitrary function pointers.
type EventListener struct {
	name   string
	s      types.EventListenerState
	filter func(types.Event) bool
	send   func(types.Event) error
	t      types.EventType
}

// NewEventListener creates a new initialized, full specified EventListener
func NewEventListener(name string, t types.EventType, filter func(types.Event) bool, send func(types.Event) error) *EventListener {
	el := &EventListener{}
	el.name = name
	el.s = types.EventListener_RUN
	el.filter = filter
	el.send = send
	el.t = t
	return el
}

// Name returns the name of this listener; names are used to make unique keys, must be unique
func (v *EventListener) Name() string { return v.name }

// State is the current state of the listener; listeners can be temporarily muted, for instance
func (v *EventListener) State() types.EventListenerState { return v.s }

// SetState sets the listener runstate
func (v *EventListener) SetState(s types.EventListenerState) { v.s = s }

// Send processes the callback to send the event to the object listening.
func (v *EventListener) Send(ev types.Event) (e error) {
	if v.filter(ev) {
		return v.send(ev)
	}
	return
}

// Filter processes a filter callback, returns whether this event would be filtered.
// Send uses this automatically.
func (v *EventListener) Filter(ev types.Event) (r bool) {
	return v.filter(ev)
}

// Type returns the type of event we're listening for.  This is another kind of filter.
func (v *EventListener) Type() types.EventType { return v.t }
