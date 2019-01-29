/* Query.go: defines QueryEngine and other useful functions for querying state
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
	"reflect"

	"github.com/hpc/kraken/lib"
)

//////////////////
// Query Object /
////////////////

var _ lib.Query = (*Query)(nil)

// Query objects describe a state query
type Query struct {
	t lib.QueryType
	s lib.QueryState
	u string
	v []reflect.Value
	c chan lib.QueryResponse
}

// NewQuery creates an initialized query; this is how all Queries should be created
func NewQuery(t lib.QueryType, s lib.QueryState, u string, v []reflect.Value) (*Query, chan lib.QueryResponse) {
	q := &Query{}
	q.t = t
	q.s = s
	q.u = u
	q.v = v
	q.c = make(chan lib.QueryResponse)
	return q, q.c
}

// Type returns the type of the query (e.g., Create, Update...)
func (q *Query) Type() lib.QueryType { return q.t }

// State returns the state (Dsc, Cfg, or Both) we are querying
func (q *Query) State() lib.QueryState { return q.s }

// URL returns a string representing the object being queried
func (q *Query) URL() string { return q.u }

// Value returns an array of associated refelct.Value's with this query
func (q *Query) Value() []reflect.Value { return q.v }

// ResponseChan returns the channel taht a QueryResponse should be sent on
func (q *Query) ResponseChan() chan<- lib.QueryResponse { return q.c }

//////////////////////////
// QueryResponse Object /
////////////////////////

var _ lib.QueryResponse = (*QueryResponse)(nil)

// A QueryResponse is sent by the Engine to the requester with results and possible errors
type QueryResponse struct {
	e error
	v []reflect.Value
}

// NewQueryResponse creates an initialized and fully specified QueryResponse
func NewQueryResponse(v []reflect.Value, e error) *QueryResponse {
	qr := &QueryResponse{
		e: e,
		v: v,
	}
	return qr
}

// Error returns the error value of the QueryResponse
func (q *QueryResponse) Error() error { return q.e }

// Value returns an array of []reflect.Value's that may have resulted from the query
func (q *QueryResponse) Value() []reflect.Value { return q.v }

////////////////////////
// QueryEngine Object /
//////////////////////

// QueryEngine provides a simple mechanism for state queries
// FIXME: QueryEngine should probably be abstracted
type QueryEngine struct {
	sd chan<- lib.Query
	sm chan<- lib.Query
}

// NewQueryEngine creates a specified QueryEngine; this is the only way to set it up
func NewQueryEngine(sd chan<- lib.Query, sm chan<- lib.Query) *QueryEngine {
	qe := &QueryEngine{
		sd: sd,
		sm: sm,
	}
	return qe
}

// Create can be used to create a new node in the Engine
func (q *QueryEngine) Create(n lib.Node) (nc lib.Node, e error) {
	query, r := NewQuery(
		lib.Query_CREATE,
		lib.QueryState_BOTH,
		"",
		[]reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(lib.Node), e
}

// Read will read a node from the Engine's Cfg store
func (q *QueryEngine) Read(n lib.NodeID) (nc lib.Node, e error) {
	if n.Nil() {
		return nil, fmt.Errorf("invalid NodeID in read")
	}
	query, r := NewQuery(lib.Query_READ, lib.QueryState_CONFIG, n.String(), []reflect.Value{})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(lib.Node), e
}

// ReadDsc will read a node from the Engine's Dsc store
func (q *QueryEngine) ReadDsc(n lib.NodeID) (nc lib.Node, e error) {
	query, r := NewQuery(lib.Query_READ, lib.QueryState_DISCOVER, n.String(), []reflect.Value{})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(lib.Node), e
}

// ReadDot will get a dot graph from the sme for a node
func (q *QueryEngine) ReadDot(n lib.Node) (sc string, e error) {
	query, r := NewQuery(lib.Query_READDOT, lib.QueryState_BOTH, "", []reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(string), e
}

// Update will update a node in the Engine's Cfg store
func (q *QueryEngine) Update(n lib.Node) (nc lib.Node, e error) {
	query, r := NewQuery(
		lib.Query_UPDATE,
		lib.QueryState_CONFIG,
		"",
		[]reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(lib.Node), e
}

// UpdateDsc will update a node in the Engine's Dsc store
func (q *QueryEngine) UpdateDsc(n lib.Node) (nc lib.Node, e error) {
	query, r := NewQuery(
		lib.Query_UPDATE,
		lib.QueryState_DISCOVER,
		"",
		[]reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(lib.Node), e
}

// Delete will delete a Node from the Engine
func (q *QueryEngine) Delete(nid lib.NodeID) (nc lib.Node, e error) {
	query, r := NewQuery(
		lib.Query_DELETE,
		lib.QueryState_BOTH,
		lib.NodeURLJoin(nid.String(), ""),
		[]reflect.Value{})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(lib.Node), e
}

// ReadAll will get a slice of all nodes from the Cfg state
func (q *QueryEngine) ReadAll() (nc []lib.Node, e error) {
	query, r := NewQuery(
		lib.Query_READALL,
		lib.QueryState_CONFIG,
		"",
		[]reflect.Value{})
	v, e := q.blockingQuery(query, r)
	for _, i := range v {
		nc = append(nc, i.Interface().(lib.Node))
	}
	return
}

// ReadAllDsc will get a slice of all nodes from the Dsc state
func (q *QueryEngine) ReadAllDsc() (nc []lib.Node, e error) {
	query, r := NewQuery(
		lib.Query_READALL,
		lib.QueryState_DISCOVER,
		"",
		[]reflect.Value{})
	v, e := q.blockingQuery(query, r)
	for _, i := range v {
		nc = append(nc, i.Interface().(lib.Node))
	}
	return
}

// DeleteAll will delete all nodes from the Engine.  !!!DANGEROUS!!!
func (q *QueryEngine) DeleteAll() (nc []lib.Node, e error) {
	query, r := NewQuery(
		lib.Query_DELETEALL,
		lib.QueryState_BOTH,
		"",
		[]reflect.Value{})
	v, e := q.blockingQuery(query, r)
	for _, i := range v {
		nc = append(nc, i.Interface().(lib.Node))
	}
	return
}

// GetValue will get a value from the Cfg state via a URL
func (q *QueryEngine) GetValue(url string) (v reflect.Value, e error) {
	query, r := NewQuery(
		lib.Query_GETVALUE,
		lib.QueryState_CONFIG,
		url,
		[]reflect.Value{})
	vs, e := q.blockingQuery(query, r)
	if len(vs) < 1 {
		return
	}
	return vs[0], e
}

// GetValueDsc will get a value from the Dsc state via a URL
func (q *QueryEngine) GetValueDsc(url string) (v reflect.Value, e error) {
	query, r := NewQuery(
		lib.Query_GETVALUE,
		lib.QueryState_DISCOVER,
		url,
		[]reflect.Value{})
	vs, e := q.blockingQuery(query, r)
	if len(vs) < 1 {
		return
	}
	return vs[0], e
}

// SetValue will set a value in the Cfg state via a URL
func (q *QueryEngine) SetValue(url string, v reflect.Value) (rv reflect.Value, e error) {
	query, r := NewQuery(
		lib.Query_SETVALUE,
		lib.QueryState_CONFIG,
		url,
		[]reflect.Value{v})
	vs, e := q.blockingQuery(query, r)
	if len(vs) < 1 {
		return
	}
	return vs[0], e
}

// SetValueDsc will set a value in the Dsc state via a URL
func (q *QueryEngine) SetValueDsc(url string, v reflect.Value) (rv reflect.Value, e error) {
	query, r := NewQuery(
		lib.Query_SETVALUE,
		lib.QueryState_DISCOVER,
		url,
		[]reflect.Value{v})
	vs, e := q.blockingQuery(query, r)
	if len(vs) < 1 {
		return
	}
	return vs[0], e
}

// TODO: write a better query language

////////////////////////
// Unexported methods /
//////////////////////

func (q *QueryEngine) blockingQuery(query lib.Query, r <-chan lib.QueryResponse) ([]reflect.Value, error) {
	var qr lib.QueryResponse
	var s chan<- lib.Query
	if query.Type() != lib.Query_READDOT {
		s = q.sd
	} else {
		s = q.sm
	}
	s <- query
	qr = <-r
	return qr.Value(), qr.Error()
}
