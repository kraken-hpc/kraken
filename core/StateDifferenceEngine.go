/* StateDifferenceEngine.go: the StateDifferenceEngine maintains Configuration (Cfg) and Discoverable (Dsc)
 *            states, and emits events when they are different.
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
	"net"
	"reflect"

	"github.com/hpc/kraken/lib"
)

///////////////////////
// Auxiliary Objects /
/////////////////////

// StateChangeType is an enumeration for state change events
type StateChangeType uint8

const (
	StateChange_CREATE StateChangeType = 0
	StateChange_READ   StateChangeType = 1 //unused
	StateChange_UPDATE StateChangeType = 2
	StateChange_DELETE StateChangeType = 3
)

// A StateChangeEvent is emitted when the StateDifferenceEngine detects a change to either Dsc or Cfg
type StateChangeEvent struct {
	Type  StateChangeType
	URL   string
	Value reflect.Value
}

// NewStateChangeEvent creates a new event of this time, fully specified
func NewStateChangeEvent(t StateChangeType, u string, v reflect.Value) lib.Event {
	sce := &StateChangeEvent{
		Type:  t,
		URL:   u,
		Value: v,
	}
	return NewEvent(lib.Event_STATE_CHANGE, sce.URL, sce)
}

///////////////////
// StateDifferenceEngine Object /
/////////////////

var _ lib.CRUD = (*StateDifferenceEngine)(nil)
var _ lib.Resolver = (*StateDifferenceEngine)(nil)
var _ lib.BulkCRUD = (*StateDifferenceEngine)(nil)
var _ lib.StateDifferenceEngine = (*StateDifferenceEngine)(nil)

// An StateDifferenceEngine maintains two kinds of state:
// - "Discoverable" (Dsc) is the discovered state of the system
// - "Configuration" (Cfg) is the intended state of the system
// An StateDifferenceEngine provides a simple Query langage for accessing/updating state.
// An StateDifferenceEngine emits Events on state change
type StateDifferenceEngine struct {
	log   lib.Logger
	em    *EventEmitter
	dsc   *State
	cfg   *State
	qc    chan lib.Query
	schan chan<- lib.EventListener
}

// NewStateDifferenceEngine initializes a new StateDifferenceEngine object given a Context
func NewStateDifferenceEngine(ctx Context) *StateDifferenceEngine {
	n := &StateDifferenceEngine{}
	n.dsc = NewState()
	n.cfg = NewState()
	n.qc = make(chan lib.Query)
	n.em = NewEventEmitter(lib.Event_STATE_CHANGE)
	n.schan = ctx.SubChan
	n.log = &ctx.Logger
	n.log.SetModule("StateDifferenceEngine")
	// every engine should know a little something about itself
	me := NewNodeWithID(ctx.Self.String())
	if _, e := me.SetValue(ctx.SSE.AddrURL, reflect.ValueOf([]byte(net.ParseIP(ctx.SSE.Addr).To4()))); e != nil {
		n.Logf(CRITICAL, "failed to set our own IP: %v\n", e)
	}
	n.Create(me)
	return n
}

// Create a node in the state engine
func (n *StateDifferenceEngine) Create(m lib.Node) (r lib.Node, e error) {
	r, e = n.cfg.Create(m)
	if e != nil {
		return
	}
	_, e = n.dsc.Create(n.makeDscNode(m.(*Node)))
	if e != nil {
		n.cfg.Delete(m)
		r = nil
		return
	}
	go n.EmitOne(NewStateChangeEvent(StateChange_CREATE, lib.NodeURLJoin(m.ID().String(), ""), reflect.ValueOf(r)))
	return
}

// Read reads a node from Cfg
func (n *StateDifferenceEngine) Read(nid lib.NodeID) (r lib.Node, e error) {
	// we don't emit read events
	return n.cfg.Read(nid)
}

// ReadDsc reads a node from Dsc
func (n *StateDifferenceEngine) ReadDsc(nid lib.NodeID) (r lib.Node, e error) {
	return n.dsc.Read(nid)
}

// Update updates a node in Cfg
func (n *StateDifferenceEngine) Update(m lib.Node) (r lib.Node, e error) {
	return n.updateByType(false, m)
}

// UpdateDsc updates a node in Dsc
func (n *StateDifferenceEngine) UpdateDsc(m lib.Node) (r lib.Node, e error) {
	return n.updateByType(true, m)
}

// Delete deletes a node
func (n *StateDifferenceEngine) Delete(m lib.Node) (r lib.Node, e error) {
	return n.DeleteByID(m.ID())
}

// DeleteByID deletes a node by its NodeID
func (n *StateDifferenceEngine) DeleteByID(nid lib.NodeID) (r lib.Node, e error) {
	n.dsc.DeleteByID(nid)
	r, e = n.cfg.DeleteByID(nid)
	go n.EmitOne(NewStateChangeEvent(StateChange_DELETE, lib.NodeURLJoin(nid.String(), ""), reflect.ValueOf(r)))
	return
}

// GetValue gets a specific sub-value from Cfg
func (n *StateDifferenceEngine) GetValue(url string) (r reflect.Value, e error) {
	return n.cfg.GetValue(url)
}

// GetValueDsc gets a specific sub-value from Dsc
func (n *StateDifferenceEngine) GetValueDsc(url string) (r reflect.Value, e error) {
	return n.dsc.GetValue(url)
}

// SetValue sets a specific sub-value in Cfg
func (n *StateDifferenceEngine) SetValue(url string, v reflect.Value) (r reflect.Value, e error) {
	r, e = n.cfg.SetValue(url, v)
	if e != nil {
		n.Logf(ERROR, "failed to set value (cfg): %v", e)
	}
	go n.EmitOne(NewStateChangeEvent(StateChange_UPDATE, url, reflect.ValueOf(r)))
	return
}

// SetValueDsc sets a specific sub-value in Cfg
func (n *StateDifferenceEngine) SetValueDsc(url string, v reflect.Value) (r reflect.Value, e error) {
	r, e = n.dsc.SetValue(url, v)
	if e != nil {
		n.Logf(ERROR, "failed to set value (dsc): %v", e)
	}
	go n.EmitOne(NewStateChangeEvent(StateChange_UPDATE, url, reflect.ValueOf(r)))
	return
}

// BulkCreate creates multiple nodes
func (n *StateDifferenceEngine) BulkCreate(ms []lib.Node) (r []lib.Node, e error) {
	r, e = n.cfg.BulkCreate(ms)
	var dms []lib.Node
	var evs []lib.Event
	// we should just do this for `r`, since it may be a subset of `ms`
	for _, v := range r {
		dms = append(dms, n.makeDscNode(v.(*Node)))
		evs = append(evs, NewStateChangeEvent(StateChange_CREATE, lib.NodeURLJoin(v.ID().String(), ""), reflect.ValueOf(v)))
	}
	dr, de := n.dsc.BulkCreate(dms)
	if len(dr) != len(r) {
		// We got unequal adds to dsc & cfg.  Only safe thing is to back out
		n.cfg.BulkDelete(r)
		n.dsc.BulkDelete(dr)
		r = []lib.Node{}
		e = fmt.Errorf("failed to add nodes to both dsc & cfg, rolling back: %s, %s", e.Error(), de.Error())
		return
	}
	go n.Emit(evs)
	return
}

// BulkRead reads multiple nodes from Cfg
func (n *StateDifferenceEngine) BulkRead(nids []lib.NodeID) (r []lib.Node, e error) {
	return n.cfg.BulkRead(nids)
}

// BulkReadDsc reads multiple nodes from Dsc
func (n *StateDifferenceEngine) BulkReadDsc(nids []lib.NodeID) (r []lib.Node, e error) {
	return n.dsc.BulkRead(nids)
}

// BulkUpdate updates multiple nodes in Cfg
func (n *StateDifferenceEngine) BulkUpdate(ms []lib.Node) (r []lib.Node, e error) {
	return n.bulkUpdateByType(false, ms)
}

// BulkUpdateDsc updates multiple nodes in Dsc
func (n *StateDifferenceEngine) BulkUpdateDsc(ms []lib.Node) (r []lib.Node, e error) {
	return n.bulkUpdateByType(true, ms)
}

//BulkDelete deletes multiple nodes
func (n *StateDifferenceEngine) BulkDelete(ms []lib.Node) (r []lib.Node, e error) {
	r, e = n.cfg.BulkDelete(ms)
	_, de := n.dsc.BulkDelete(ms)
	var evs []lib.Event
	for _, v := range r {
		evs = append(evs, NewStateChangeEvent(StateChange_DELETE, lib.NodeURLJoin(v.ID().String(), ""), reflect.ValueOf(v)))
	}
	go n.Emit(evs)
	if de != nil && e == nil {
		e = de
	}
	return
}

// BulkDeleteByID deletes multiple nodes, keyed by their NodeID
func (n *StateDifferenceEngine) BulkDeleteByID(nids []lib.NodeID) (r []lib.Node, e error) {
	r, e = n.cfg.BulkDeleteByID(nids)
	_, de := n.dsc.BulkDeleteByID(nids)
	var evs []lib.Event
	for _, v := range r {
		evs = append(evs, NewStateChangeEvent(StateChange_DELETE, lib.NodeURLJoin(v.ID().String(), ""), reflect.ValueOf(v)))
	}
	go n.Emit(evs)
	if de != nil && e == nil { // if dr errors, but e does not, pass the error on
		e = de
	}
	return
}

// ReadAll returns a slice of all nodes in Cfg
func (n *StateDifferenceEngine) ReadAll() (r []lib.Node, e error) {
	return n.cfg.ReadAll()
}

// ReadAllDsc returns a slice of all nodes in Dsc
func (n *StateDifferenceEngine) ReadAllDsc() (r []lib.Node, e error) {
	return n.dsc.ReadAll()
}

// DeleteAll deletes all nodes in the engine, careful!
func (n *StateDifferenceEngine) DeleteAll() (r []lib.Node, e error) {
	r, e = n.cfg.DeleteAll()
	_, de := n.cfg.DeleteAll()
	var evs []lib.Event
	for _, v := range r {
		evs = append(evs, NewStateChangeEvent(StateChange_DELETE, lib.NodeURLJoin(v.ID().String(), ""), reflect.ValueOf(v)))
	}
	go n.Emit(evs)
	if de != nil && e == nil { // if dr errors, but e does not, pass the error on
		e = de
	}
	return
}

// QueryChan returns a chanel that Queries can be sent on
func (n *StateDifferenceEngine) QueryChan() chan<- lib.Query {
	return n.qc
}

// Run is a goroutine that manages queries
func (n *StateDifferenceEngine) Run() {
	n.Log(INFO, "starting StateDifferenceEngine")
	// we listen for queries as well as discovery events
	dchan := make(chan lib.Event)
	list := NewEventListener(
		"SDEDiscovery",
		lib.Event_DISCOVERY,
		func(v lib.Event) bool { return true },
		func(v lib.Event) error { return ChanSender(v, dchan) },
	)
	// subscribe our discovery listener
	n.schan <- list
	for {
		select {
		case q := <-n.qc:
			switch q.Type() {
			case lib.Query_CREATE:
				if len(q.Value()) < 1 || !q.Value()[0].IsValid() {
					go n.sendQueryResponse(NewQueryResponse([]reflect.Value{}, fmt.Errorf("malformed node in create query")), q.ResponseChan())
					break
				}
				v, e := n.Create(q.Value()[0].Interface().(lib.Node))
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case lib.Query_READ:
				var v lib.Node
				var e error
				switch q.State() {
				case lib.QueryState_CONFIG:
					v, e = n.Read(NewNodeIDFromURL(q.URL()))
					break
				case lib.QueryState_DISCOVER:
					v, e = n.ReadDsc(NewNodeIDFromURL(q.URL()))
					break
				default:
					e = fmt.Errorf("unknown state for Query_READ")
				}
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case lib.Query_UPDATE:
				var v lib.Node
				var e error
				switch q.State() {
				case lib.QueryState_CONFIG:
					v, e = n.Update(q.Value()[0].Interface().(lib.Node))
					break
				case lib.QueryState_DISCOVER:
					v, e = n.UpdateDsc(q.Value()[0].Interface().(lib.Node))
					break
				default:
					e = fmt.Errorf("unknown state for Query_UPDATE")
				}
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case lib.Query_DELETE:
				v, e := n.Delete(q.Value()[0].Interface().(lib.Node))
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case lib.Query_GETVALUE:
				var v reflect.Value
				var e error
				switch q.State() {
				case lib.QueryState_CONFIG:
					v, e = n.GetValue(q.URL())
					break
				case lib.QueryState_DISCOVER:
					v, e = n.GetValueDsc(q.URL())
					break
				}
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{v}, e), q.ResponseChan())
				break
			case lib.Query_SETVALUE:
				var v reflect.Value
				var e error
				switch q.State() {
				case lib.QueryState_CONFIG:
					v, e = n.SetValue(q.URL(), q.Value()[0])
					break
				case lib.QueryState_DISCOVER:
					v, e = n.SetValueDsc(q.URL(), q.Value()[0])
					break
				}
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{v}, e), q.ResponseChan())
				break
			case lib.Query_READALL:
				var v []lib.Node
				var e error
				switch q.State() {
				case lib.QueryState_CONFIG:
					v, e = n.ReadAll()
					break
				case lib.QueryState_DISCOVER:
					v, e = n.ReadAllDsc()
					break
				default:
					e = fmt.Errorf("unknown state for Query_READALL")
				}
				var vs []reflect.Value
				for _, i := range v {
					vs = append(vs, reflect.ValueOf(i))
				}
				go n.sendQueryResponse(NewQueryResponse(vs, e), q.ResponseChan())
				break
			case lib.Query_DELETEALL:
				v, e := n.DeleteAll()
				var vs []reflect.Value
				for _, i := range v {
					vs = append(vs, reflect.ValueOf(i))
				}
				go n.sendQueryResponse(NewQueryResponse(vs, e), q.ResponseChan())
				break
			default:
				n.Logf(NOTICE, "unsupported query type: %d", q.Type())
			}
			break
		case v := <-dchan: // got a discovery
			data := v.Data().(*DiscoveryEvent)
			_, url := lib.NodeURLSplit(data.URL)
			val, ok := Registry.Discoverables[data.Module][url][data.ValueID]
			if !ok {
				n.Logf(ERROR, "got discover, but can't lookup value: mod (%s) url (%s) id(%s)", data.Module, url, data.ValueID)
				break
			}
			vset, e := n.SetValueDsc(data.URL, val)
			if e != nil {
				n.Logf(ERROR, "failed to set discovered value: url (%s) val (%v)", data.URL, val.Interface())
			}
			n.Logf(DEBUG, "discovered %s is %v\n", data.URL, vset.Interface())
			break
		}
	}
}

////////////////////////
// Unexported methods /
//////////////////////

func (n *StateDifferenceEngine) makeDscNode(m *Node) (r *Node) {
	r = NewNodeWithID(m.ID().String())
	return r
}

func (n *StateDifferenceEngine) updateByType(dsc bool, m lib.Node) (r lib.Node, e error) {
	var old lib.Node
	var diff []string
	if dsc {
		old, e = n.dsc.Read(m.ID())
	} else {
		old, e = n.cfg.Read(m.ID())
	}
	if e != nil {
		return
	}
	if diff, e = old.(*Node).Diff(m.(*Node), lib.NodeURLJoin(m.ID().String(), "")); e != nil {
		return
	}
	if dsc {
		r, e = n.dsc.Update(m)
	} else {
		r, e = n.cfg.Update(m)
	}
	if e == nil && len(diff) > 0 {
		var evs []lib.Event
		for _, u := range diff {
			// TODO: should this include the updated value(s)?
			evs = append(evs, NewStateChangeEvent(StateChange_UPDATE, u, reflect.Value{}))
		}
		go n.Emit(evs)
	}
	return
}

func (n *StateDifferenceEngine) bulkUpdateByType(dsc bool, ms []lib.Node) (r []lib.Node, e error) {
	var old []lib.Node
	var diff []string
	var nids []lib.NodeID
	for _, v := range ms {
		nids = append(nids, v.ID())
	}
	if dsc {
		old, e = n.dsc.BulkRead(nids)
	} else {
		old, e = n.cfg.BulkRead(nids)
	}
	if e != nil {
		return
	}
	for i := range ms {
		var d []string
		if d, e = old[i].(*Node).Diff(ms[i].(*Node), lib.NodeURLJoin(ms[i].ID().String(), "")); e != nil {
			return
		}
		diff = append(diff, d...)
	}

	if dsc {
		r, e = n.dsc.BulkUpdate(ms)
	} else {
		r, e = n.cfg.BulkUpdate(ms)
	}
	if e != nil { // no emit on error
		//FIXME - there is an edge case here where *some* of the updates happen
		/// but we still get an error
		return
	}
	var evs []lib.Event
	for _, v := range diff {
		evs = append(evs, NewStateChangeEvent(StateChange_UPDATE, v, reflect.Value{}))
	}
	go n.Emit(evs)
	return
}

// goroutine
func (n *StateDifferenceEngine) sendQueryResponse(qr lib.QueryResponse, r chan<- lib.QueryResponse) {
	r <- qr
}

////////////////////////////
// Passthrough Interfaces /
//////////////////////////

/*
 * Consume Logger
 */
var _ lib.Logger = (*StateDifferenceEngine)(nil)

func (n *StateDifferenceEngine) Log(level lib.LoggerLevel, m string) { n.log.Log(level, m) }
func (n *StateDifferenceEngine) Logf(level lib.LoggerLevel, fmt string, v ...interface{}) {
	n.log.Logf(level, fmt, v...)
}
func (n *StateDifferenceEngine) SetModule(name string)                { n.log.SetModule(name) }
func (n *StateDifferenceEngine) GetModule() string                    { return n.log.GetModule() }
func (n *StateDifferenceEngine) SetLoggerLevel(level lib.LoggerLevel) { n.log.SetLoggerLevel(level) }
func (n *StateDifferenceEngine) GetLoggerLevel() lib.LoggerLevel      { return n.log.GetLoggerLevel() }
func (n *StateDifferenceEngine) IsEnabledFor(level lib.LoggerLevel) bool {
	return n.log.IsEnabledFor(level)
}

/*
 * Consume EventEmitter
 */
var _ lib.EventEmitter = (*StateDifferenceEngine)(nil)

func (n *StateDifferenceEngine) Subscribe(id string, c chan<- []lib.Event) error {
	return n.em.Subscribe(id, c)
}
func (n *StateDifferenceEngine) Unsubscribe(id string) error { return n.em.Unsubscribe(id) }
func (n *StateDifferenceEngine) Emit(v []lib.Event)          { n.em.Emit(v) }
func (n *StateDifferenceEngine) EmitOne(v lib.Event)         { n.em.EmitOne(v) }
func (n *StateDifferenceEngine) EventType() lib.EventType    { return n.em.EventType() }
