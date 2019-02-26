/* Event.go: Event objects get distributed through the EventDispatchEngine
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"github.com/hpc/kraken/lib"
)

//////////////////
// Event Object /
////////////////

var _ lib.Event = (*Event)(nil)

// An Event is a generic container for internal events, like state changes
type Event struct {
	t    lib.EventType
	url  string
	data interface{}
}

// NewEvent creates an initialized, fully specified Event
func NewEvent(t lib.EventType, url string, data interface{}) lib.Event {
	ev := &Event{
		t:    t,
		url:  url,
		data: data,
	}
	return ev
}

func (v *Event) Type() lib.EventType { return v.t }
func (v *Event) URL() string         { return v.url }
func (v *Event) Data() interface{}   { return v.data }
