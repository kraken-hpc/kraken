/* Query.go: defines the Query object used by the QueryEngine for querying state
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"reflect"

	"github.com/kraken-hpc/kraken/lib/types"
)

//////////////////
// Query Object /
////////////////

var _ types.Query = (*Query)(nil)

// Query objects describe a state query
type Query struct {
	t types.QueryType
	s types.QueryState
	u string
	v []reflect.Value
	c chan types.QueryResponse
}

// NewQuery creates an initialized query; this is how all Queries should be created
func NewQuery(t types.QueryType, s types.QueryState, url string, v []reflect.Value) (*Query, chan types.QueryResponse) {
	q := &Query{}
	q.t = t
	q.s = s
	q.u = url
	q.v = v
	q.c = make(chan types.QueryResponse)
	return q, q.c
}

// Type returns the type of the query (e.g., Create, Update...)
func (q *Query) Type() types.QueryType { return q.t }

// State returns the state (Dsc, Cfg, or Both) we are querying
func (q *Query) State() types.QueryState { return q.s }

// URL returns a string representing the object being queried
func (q *Query) URL() string { return q.u }

// Value returns an array of associated refelct.Value's with this query
func (q *Query) Value() []reflect.Value { return q.v }

// ResponseChan returns the channel that a QueryResponse should be sent on
func (q *Query) ResponseChan() chan<- types.QueryResponse { return q.c }

//////////////////////////
// QueryResponse Object /
////////////////////////

var _ types.QueryResponse = (*QueryResponse)(nil)

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
