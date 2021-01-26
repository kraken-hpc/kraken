/* Event.go: Event objects get distributed through the EventDispatchEngine
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"github.com/hpc/kraken/lib/types"
)

//////////////////
// Event Object /
////////////////

var _ types.Event = (*Event)(nil)

// An Event is a generic container for internal events, like state changes
type Event struct {
	t    types.EventType
	url  string
	data interface{}
}

// NewEvent creates an initialized, fully specified Event
func NewEvent(t types.EventType, url string, data interface{}) types.Event {
	ev := &Event{
		t:    t,
		url:  url,
		data: data,
	}
	return ev
}

func (v *Event) Type() types.EventType { return v.t }
func (v *Event) URL() string           { return v.url }
func (v *Event) Data() interface{}     { return v.data }
