/* StateDifferenceEngine.go: the StateDifferenceEngine maintains Configuration (Cfg) and Discoverable (Dsc)
 *            states, and emits events when they are different.
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
	"reflect"

	"github.com/gogo/protobuf/proto"
	pb "github.com/hpc/kraken/core/proto"
	ct "github.com/hpc/kraken/core/proto/customtypes"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

///////////////////////
// Auxiliary Objects /
/////////////////////

const (
	StateChange_CREATE     pb.StateChangeControl_Type = pb.StateChangeControl_CREATE
	StateChange_READ       pb.StateChangeControl_Type = pb.StateChangeControl_READ //unused
	StateChange_UPDATE     pb.StateChangeControl_Type = pb.StateChangeControl_UPDATE
	StateChange_DELETE     pb.StateChangeControl_Type = pb.StateChangeControl_DELETE
	StateChange_CFG_READ   pb.StateChangeControl_Type = pb.StateChangeControl_CFG_READ //unused
	StateChange_CFG_UPDATE pb.StateChangeControl_Type = pb.StateChangeControl_CFG_UPDATE
)

var StateChangeTypeString = map[pb.StateChangeControl_Type]string{
	StateChange_CREATE:     "CREATE",
	StateChange_READ:       "READ",
	StateChange_UPDATE:     "UPDATE",
	StateChange_DELETE:     "DELETE",
	StateChange_CFG_READ:   "CFG_READ",
	StateChange_CFG_UPDATE: "CFG_UPDATE",
}

// A StateChangeEvent is emitted when the StateDifferenceEngine detects a change to either Dsc or Cfg
type StateChangeEvent struct {
	Type  pb.StateChangeControl_Type
	URL   string
	Value reflect.Value
}

func (sce *StateChangeEvent) String() string {
	return fmt.Sprintf("(%s) %s = %s", StateChangeTypeString[sce.Type], sce.URL, util.ValueToString(sce.Value))
}

// NewStateChangeEvent creates a new event of this time, fully specified
func NewStateChangeEvent(t pb.StateChangeControl_Type, u string, v reflect.Value) types.Event {
	sce := &StateChangeEvent{
		Type:  t,
		URL:   u,
		Value: v,
	}
	return NewEvent(types.Event_STATE_CHANGE, sce.URL, sce)
}

///////////////////
// StateDifferenceEngine Object /
/////////////////

var _ types.CRUD = (*StateDifferenceEngine)(nil)
var _ types.Resolver = (*StateDifferenceEngine)(nil)
var _ types.BulkCRUD = (*StateDifferenceEngine)(nil)
var _ types.StateDifferenceEngine = (*StateDifferenceEngine)(nil)

// An StateDifferenceEngine maintains two kinds of state:
// - "Discoverable" (Dsc) is the discovered state of the system
// - "Configuration" (Cfg) is the intended state of the system
// An StateDifferenceEngine provides a simple Query langage for accessing/updating state.
// An StateDifferenceEngine emits Events on state change
type StateDifferenceEngine struct {
	log   types.Logger
	em    *EventEmitter
	dsc   *State
	cfg   *State
	qc    chan types.Query
	schan chan<- types.EventListener
}

// NewStateDifferenceEngine initializes a new StateDifferenceEngine object given a Context
func NewStateDifferenceEngine(ctx Context, qc chan types.Query) *StateDifferenceEngine {
	n := &StateDifferenceEngine{}
	n.dsc = NewState()
	n.cfg = NewState()
	n.qc = qc
	n.em = NewEventEmitter(types.Event_STATE_CHANGE)
	n.schan = ctx.SubChan
	n.log = &ctx.Logger
	n.log.SetModule("StateDifferenceEngine")
	// load initial states
	n.BulkCreate(ctx.SDE.InitialCfg)
	n.BulkUpdateDsc(ctx.SDE.InitialDsc)
	// we're done with this now, so no need to keep them around
	ctx.SDE.InitialCfg = []types.Node{}
	ctx.SDE.InitialDsc = []types.Node{}
	return n
}

// Create a node in the state engine
func (n *StateDifferenceEngine) Create(m types.Node) (r types.Node, e error) {
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
	go n.EmitOne(NewStateChangeEvent(StateChange_CREATE, util.NodeURLJoin(m.ID().String(), ""), reflect.ValueOf(r)))
	return
}

// Read reads a node from Cfg
func (n *StateDifferenceEngine) Read(nid types.NodeID) (r types.Node, e error) {
	// we don't emit read events
	return n.cfg.Read(nid)
}

// ReadDsc reads a node from Dsc
func (n *StateDifferenceEngine) ReadDsc(nid types.NodeID) (r types.Node, e error) {
	return n.dsc.Read(nid)
}

// Update updates a node in Cfg
func (n *StateDifferenceEngine) Update(m types.Node) (r types.Node, e error) {
	return n.updateByType(false, m)
}

// UpdateDsc updates a node in Dsc
func (n *StateDifferenceEngine) UpdateDsc(m types.Node) (r types.Node, e error) {
	return n.updateByType(true, m)
}

// Delete deletes a node
func (n *StateDifferenceEngine) Delete(m types.Node) (r types.Node, e error) {
	return n.DeleteByID(m.ID())
}

// DeleteByID deletes a node by its NodeID
func (n *StateDifferenceEngine) DeleteByID(nid types.NodeID) (r types.Node, e error) {
	n.dsc.DeleteByID(nid)
	r, e = n.cfg.DeleteByID(nid)
	go n.EmitOne(NewStateChangeEvent(StateChange_DELETE, util.NodeURLJoin(nid.String(), ""), reflect.ValueOf(r)))
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

func (n *StateDifferenceEngine) setValueDiff(url string, cur, v reflect.Value) (diff []string, err error) {
	diff = []string{}
	switch cur.Kind() {
	case reflect.Struct: // maybe a message we can diff?
		if m1, ok := cur.Addr().Interface().(proto.Message); ok {
			// this is a proto message
			if m2, ok := v.Addr().Interface().(proto.Message); ok {
				return util.MessageDiff(m1, m2, url)
			}
			// if we reacch this m2 didn't convert
			return diff, fmt.Errorf("invalid value passed to SetValue: %v", v)
		}
		fallthrough
	case reflect.Slice, reflect.Map: // not comparable
		// not comparable by normal means.  Our best bet now is just to deepequal them
		if !reflect.DeepEqual(cur.Interface(), v.Interface()) {
			diff = append(diff, url)
		}
	default: // should be comparable
		if cur.Interface() != v.Interface() {
			diff = append(diff, url)
		}
	}
	return
}

// SetValue sets a specific sub-value in Cfg
func (n *StateDifferenceEngine) SetValue(url string, v reflect.Value) (r reflect.Value, e error) {
	var cur reflect.Value
	cur, e = n.cfg.GetValue(url)
	if e != nil { // this only happens if it's not a valid url
		return
	}
	var diff []string
	if diff, e = n.setValueDiff(url, cur, v); e != nil {
		return
	}
	if len(diff) == 0 { // nothing new to set
		n.Logf(DDEBUG, "SetValue called, but it's not a change: %s", url)
		r = v
		return
	}
	r, e = n.cfg.SetValue(url, v)
	if e != nil {
		n.Logf(ERROR, "failed to set value (cfg): %v", e)
	}
	for _, d := range diff {
		var dv reflect.Value
		if d == url {
			dv = r
		} else {
			dv, _ = n.cfg.GetValue(d)
		}
		go n.EmitOne(NewStateChangeEvent(StateChange_CFG_UPDATE, d, dv))
	}
	return
}

// SetValueDsc sets a specific sub-value in Dsc
func (n *StateDifferenceEngine) SetValueDsc(url string, v reflect.Value) (r reflect.Value, e error) {
	var cur reflect.Value
	cur, e = n.dsc.GetValue(url)
	if e != nil { // this only happens if it's not a valid url
		return
	}
	var diff []string
	if diff, e = n.setValueDiff(url, cur, v); e != nil {
		return
	}
	if len(diff) == 0 { // nothing new to set
		n.Logf(DDEBUG, "SetValueDsc called, but it's not a change: %s", url)
		r = v
		return
	}
	r, e = n.dsc.SetValue(url, v)
	if e != nil {
		n.Logf(ERROR, "failed to set value (dsc): %v", e)
	}
	for _, d := range diff {
		var dv reflect.Value
		if d == url {
			dv = r
		} else {
			dv, _ = n.dsc.GetValue(d)
		}
		go n.EmitOne(NewStateChangeEvent(StateChange_UPDATE, d, dv))
	}
	return
}

// BulkCreate creates multiple nodes
func (n *StateDifferenceEngine) BulkCreate(ms []types.Node) (r []types.Node, e error) {
	r, e = n.cfg.BulkCreate(ms)
	var dms []types.Node
	var evs []types.Event
	// we should just do this for `r`, since it may be a subset of `ms`
	for _, v := range r {
		dms = append(dms, n.makeDscNode(v.(*Node)))
		evs = append(evs, NewStateChangeEvent(StateChange_CREATE, util.NodeURLJoin(v.ID().String(), ""), reflect.ValueOf(v)))
	}
	dr, de := n.dsc.BulkCreate(dms)
	if len(dr) != len(r) {
		// We got unequal adds to dsc & cfg.  Only safe thing is to back out
		n.cfg.BulkDelete(r)
		n.dsc.BulkDelete(dr)
		r = []types.Node{}
		e = fmt.Errorf("failed to add nodes to both dsc & cfg, rolling back: %s, %s", e.Error(), de.Error())
		return
	}
	go n.Emit(evs)
	return
}

// BulkRead reads multiple nodes from Cfg
func (n *StateDifferenceEngine) BulkRead(nids []types.NodeID) (r []types.Node, e error) {
	return n.cfg.BulkRead(nids)
}

// BulkReadDsc reads multiple nodes from Dsc
func (n *StateDifferenceEngine) BulkReadDsc(nids []types.NodeID) (r []types.Node, e error) {
	return n.dsc.BulkRead(nids)
}

// BulkUpdate updates multiple nodes in Cfg
func (n *StateDifferenceEngine) BulkUpdate(ms []types.Node) (r []types.Node, e error) {
	return n.bulkUpdateByType(false, ms)
}

// BulkUpdateDsc updates multiple nodes in Dsc
func (n *StateDifferenceEngine) BulkUpdateDsc(ms []types.Node) (r []types.Node, e error) {
	return n.bulkUpdateByType(true, ms)
}

//BulkDelete deletes multiple nodes
func (n *StateDifferenceEngine) BulkDelete(ms []types.Node) (r []types.Node, e error) {
	r, e = n.cfg.BulkDelete(ms)
	_, de := n.dsc.BulkDelete(ms)
	var evs []types.Event
	for _, v := range r {
		evs = append(evs, NewStateChangeEvent(StateChange_DELETE, util.NodeURLJoin(v.ID().String(), ""), reflect.ValueOf(v)))
	}
	go n.Emit(evs)
	if de != nil && e == nil {
		e = de
	}
	return
}

// BulkDeleteByID deletes multiple nodes, keyed by their NodeID
func (n *StateDifferenceEngine) BulkDeleteByID(nids []types.NodeID) (r []types.Node, e error) {
	r, e = n.cfg.BulkDeleteByID(nids)
	_, de := n.dsc.BulkDeleteByID(nids)
	var evs []types.Event
	for _, v := range r {
		evs = append(evs, NewStateChangeEvent(StateChange_DELETE, util.NodeURLJoin(v.ID().String(), ""), reflect.ValueOf(v)))
	}
	go n.Emit(evs)
	if de != nil && e == nil { // if dr errors, but e does not, pass the error on
		e = de
	}
	return
}

// ReadAll returns a slice of all nodes in Cfg
func (n *StateDifferenceEngine) ReadAll() (r []types.Node, e error) {
	return n.cfg.ReadAll()
}

// ReadAllDsc returns a slice of all nodes in Dsc
func (n *StateDifferenceEngine) ReadAllDsc() (r []types.Node, e error) {
	return n.dsc.ReadAll()
}

// DeleteAll deletes all nodes in the engine, careful!
func (n *StateDifferenceEngine) DeleteAll() (r []types.Node, e error) {
	r, e = n.cfg.DeleteAll()
	_, de := n.cfg.DeleteAll()
	var evs []types.Event
	for _, v := range r {
		evs = append(evs, NewStateChangeEvent(StateChange_DELETE, util.NodeURLJoin(v.ID().String(), ""), reflect.ValueOf(v)))
	}
	go n.Emit(evs)
	if de != nil && e == nil { // if dr errors, but e does not, pass the error on
		e = de
	}
	return
}

// QueryChan returns a chanel that Queries can be sent on
func (n *StateDifferenceEngine) QueryChan() chan<- types.Query {
	return n.qc
}

// Run is a goroutine that manages queries
func (n *StateDifferenceEngine) Run(ready chan<- interface{}) {
	n.Log(INFO, "starting StateDifferenceEngine")
	// we listen for queries as well as discovery events
	dchan := make(chan types.Event)
	list := NewEventListener(
		"SDEDiscovery",
		types.Event_DISCOVERY,
		func(v types.Event) bool { return true },
		func(v types.Event) error { return ChanSender(v, dchan) },
	)
	// subscribe our discovery listener
	n.schan <- list
	ready <- nil
	for {
		select {
		case q := <-n.qc:
			switch q.Type() {
			case types.Query_CREATE:
				if len(q.Value()) < 1 || !q.Value()[0].IsValid() {
					go n.sendQueryResponse(NewQueryResponse([]reflect.Value{}, fmt.Errorf("malformed node in create query")), q.ResponseChan())
					break
				}
				v, e := n.Create(q.Value()[0].Interface().(types.Node))
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case types.Query_READ:
				var v types.Node
				var e error
				switch q.State() {
				case types.QueryState_CONFIG:
					v, e = n.Read(ct.NewNodeIDFromURL(q.URL()))
					break
				case types.QueryState_DISCOVER:
					v, e = n.ReadDsc(ct.NewNodeIDFromURL(q.URL()))
					break
				default:
					e = fmt.Errorf("unknown state for Query_READ")
				}
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case types.Query_UPDATE:
				var v types.Node
				var e error
				switch q.State() {
				case types.QueryState_CONFIG:
					v, e = n.Update(q.Value()[0].Interface().(types.Node))
					break
				case types.QueryState_DISCOVER:
					v, e = n.UpdateDsc(q.Value()[0].Interface().(types.Node))
					break
				default:
					e = fmt.Errorf("unknown state for Query_UPDATE")
				}
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case types.Query_DELETE:
				v, e := n.Delete(q.Value()[0].Interface().(types.Node))
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case types.Query_GETVALUE:
				var v reflect.Value
				var e error
				switch q.State() {
				case types.QueryState_CONFIG:
					v, e = n.GetValue(q.URL())
					break
				case types.QueryState_DISCOVER:
					v, e = n.GetValueDsc(q.URL())
					break
				}
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{v}, e), q.ResponseChan())
				break
			case types.Query_SETVALUE:
				var v reflect.Value
				var e error
				switch q.State() {
				case types.QueryState_CONFIG:
					v, e = n.SetValue(q.URL(), q.Value()[0])
					break
				case types.QueryState_DISCOVER:
					v, e = n.SetValueDsc(q.URL(), q.Value()[0])
					break
				}
				go n.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{v}, e), q.ResponseChan())
				break
			case types.Query_READALL:
				var v []types.Node
				var e error
				switch q.State() {
				case types.QueryState_CONFIG:
					v, e = n.ReadAll()
					break
				case types.QueryState_DISCOVER:
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
			case types.Query_DELETEALL:
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
			_, url := util.NodeURLSplit(data.URL)
			val, ok := Registry.Discoverables[data.ID][url][data.ValueID]
			n.Logf(DDEBUG, "processing discovery: si (%s) url (%s) id(%s)", data.ID, url, data.ValueID)
			if !ok {
				n.Logf(ERROR, "got discover, but can't lookup value: si (%s) url (%s) id(%s)", data.ID, url, data.ValueID)
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

func (n *StateDifferenceEngine) updateByType(dsc bool, m types.Node) (r types.Node, e error) {
	var old types.Node
	var diff []string
	if dsc {
		old, e = n.dsc.Read(m.ID())
	} else {
		old, e = n.cfg.Read(m.ID())
	}
	if e != nil {
		return
	}
	if diff, e = old.(*Node).Diff(m.(*Node), util.NodeURLJoin(m.ID().String(), "")); e != nil {
		return
	}
	if dsc {
		r, e = n.dsc.Update(m)
	} else {
		r, e = n.cfg.Update(m)
	}
	if e == nil && len(diff) > 0 {
		var evs []types.Event
		utype := StateChange_UPDATE
		if !dsc {
			utype = StateChange_CFG_UPDATE
		}
		for _, u := range diff {
			_, url := util.NodeURLSplit(u)
			v, _ := r.GetValue(url)
			evs = append(evs, NewStateChangeEvent(utype, u, reflect.ValueOf(v)))
		}
		go n.Emit(evs)
	}
	return
}

func (n *StateDifferenceEngine) bulkUpdateByType(dsc bool, ms []types.Node) (r []types.Node, e error) {
	var old []types.Node
	var diff []string
	var nids []types.NodeID
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
		if d, e = old[i].(*Node).Diff(ms[i].(*Node), util.NodeURLJoin(ms[i].ID().String(), "")); e != nil {
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
	var evs []types.Event
	utype := StateChange_UPDATE
	if !dsc {
		utype = StateChange_CFG_UPDATE
	}
	for _, v := range diff {
		evs = append(evs, NewStateChangeEvent(utype, v, reflect.Value{}))
	}
	go n.Emit(evs)
	return
}

// goroutine
func (n *StateDifferenceEngine) sendQueryResponse(qr types.QueryResponse, r chan<- types.QueryResponse) {
	r <- qr
}

////////////////////////////
// Passthrough Interfaces /
//////////////////////////

/*
 * Consume Logger
 */
var _ types.Logger = (*StateDifferenceEngine)(nil)

func (n *StateDifferenceEngine) Log(level types.LoggerLevel, m string) { n.log.Log(level, m) }
func (n *StateDifferenceEngine) Logf(level types.LoggerLevel, fmt string, v ...interface{}) {
	n.log.Logf(level, fmt, v...)
}
func (n *StateDifferenceEngine) SetModule(name string)                  { n.log.SetModule(name) }
func (n *StateDifferenceEngine) GetModule() string                      { return n.log.GetModule() }
func (n *StateDifferenceEngine) SetLoggerLevel(level types.LoggerLevel) { n.log.SetLoggerLevel(level) }
func (n *StateDifferenceEngine) GetLoggerLevel() types.LoggerLevel      { return n.log.GetLoggerLevel() }
func (n *StateDifferenceEngine) IsEnabledFor(level types.LoggerLevel) bool {
	return n.log.IsEnabledFor(level)
}

/*
 * Consume EventEmitter
 */
var _ types.EventEmitter = (*StateDifferenceEngine)(nil)

func (n *StateDifferenceEngine) Subscribe(id string, c chan<- []types.Event) error {
	return n.em.Subscribe(id, c)
}
func (n *StateDifferenceEngine) Unsubscribe(id string) error { return n.em.Unsubscribe(id) }
func (n *StateDifferenceEngine) Emit(v []types.Event)        { n.em.Emit(v) }
func (n *StateDifferenceEngine) EmitOne(v types.Event)       { n.em.EmitOne(v) }
func (n *StateDifferenceEngine) EventType() types.EventType  { return n.em.EventType() }
