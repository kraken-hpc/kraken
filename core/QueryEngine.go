/* QueryEngine.go: defines QueryEngine and other useful functions for querying state
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
	reflect "reflect"

	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

////////////////////////
// QueryEngine Object /
//////////////////////

// QueryEngine provides a simple mechanism for state queries
// FIXME: QueryEngine should probably be abstracted
type QueryEngine struct {
	sd chan<- types.Query
	sm chan<- types.Query
}

// NewQueryEngine creates a specified QueryEngine; this is the only way to set it up
func NewQueryEngine(sd chan<- types.Query, sm chan<- types.Query) *QueryEngine {
	qe := &QueryEngine{
		sd: sd,
		sm: sm,
	}
	return qe
}

// Create can be used to create a new node in the Engine
func (q *QueryEngine) Create(n types.Node) (nc types.Node, e error) {
	query, r := NewQuery(
		types.Query_CREATE,
		types.QueryState_BOTH,
		"",
		[]reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(types.Node), e
}

// Read will read a node from the Engine's Cfg store
func (q *QueryEngine) Read(n types.NodeID) (nc types.Node, e error) {
	if n.Nil() {
		return nil, fmt.Errorf("invalid NodeID in read")
	}
	query, r := NewQuery(types.Query_READ, types.QueryState_CONFIG, n.String(), []reflect.Value{})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(types.Node), e
}

// ReadDsc will read a node from the Engine's Dsc store
func (q *QueryEngine) ReadDsc(n types.NodeID) (nc types.Node, e error) {
	query, r := NewQuery(types.Query_READ, types.QueryState_DISCOVER, n.String(), []reflect.Value{})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(types.Node), e
}

func (q *QueryEngine) ReadMutationNodes(url string) (mc pb.MutationNodeList, e error) {
	query, r := NewQuery(types.Query_MUTATIONNODES, types.QueryState_BOTH, url, []reflect.Value{})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(pb.MutationNodeList), e
}

func (q *QueryEngine) ReadMutationEdges(url string) (mc pb.MutationEdgeList, e error) {
	query, r := NewQuery(types.Query_MUTATIONEDGES, types.QueryState_BOTH, url, []reflect.Value{})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(pb.MutationEdgeList), e
}

func (q *QueryEngine) ReadNodeMutationNodes(url string) (mc pb.MutationNodeList, e error) {
	n := pb.NewNodeIDFromURL(url)
	query, r := NewQuery(types.Query_MUTATIONNODES, types.QueryState_BOTH, url, []reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(pb.MutationNodeList), e
}

func (q *QueryEngine) ReadNodeMutationEdges(url string) (mc pb.MutationEdgeList, e error) {
	n := pb.NewNodeIDFromURL(url)
	query, r := NewQuery(types.Query_MUTATIONEDGES, types.QueryState_BOTH, url, []reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(pb.MutationEdgeList), e
}

func (q *QueryEngine) ReadNodeMutationPath(url string) (mc pb.MutationPath, e error) {
	n := pb.NewNodeIDFromURL(url)
	query, r := NewQuery(types.Query_MUTATIONPATH, types.QueryState_BOTH, url, []reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(pb.MutationPath), e
}

func (q *QueryEngine) Freeze() (e error) {
	query, r := NewQuery(
		types.Query_FREEZE,
		types.QueryState_BOTH,
		"",
		[]reflect.Value{})
	_, e = q.blockingQuery(query, r)
	return
}

func (q *QueryEngine) Thaw() (e error) {
	query, r := NewQuery(
		types.Query_THAW,
		types.QueryState_BOTH,
		"",
		[]reflect.Value{})
	_, e = q.blockingQuery(query, r)
	return
}

func (q *QueryEngine) Frozen() (b bool, e error) {
	query, r := NewQuery(
		types.Query_FROZEN,
		types.QueryState_BOTH,
		"",
		[]reflect.Value{})
	rb, e := q.blockingQuery(query, r)

	return rb[0].Interface().(bool), e
}

// Update will update a node in the Engine's Cfg store
func (q *QueryEngine) Update(n types.Node) (nc types.Node, e error) {
	query, r := NewQuery(
		types.Query_UPDATE,
		types.QueryState_CONFIG,
		"",
		[]reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(types.Node), e
}

// UpdateDsc will update a node in the Engine's Dsc store
func (q *QueryEngine) UpdateDsc(n types.Node) (nc types.Node, e error) {
	query, r := NewQuery(
		types.Query_UPDATE,
		types.QueryState_DISCOVER,
		"",
		[]reflect.Value{reflect.ValueOf(n)})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(types.Node), e
}

// Delete will delete a Node from the Engine
func (q *QueryEngine) Delete(nid types.NodeID) (nc types.Node, e error) {
	query, r := NewQuery(
		types.Query_DELETE,
		types.QueryState_BOTH,
		util.NodeURLJoin(nid.String(), ""),
		[]reflect.Value{})
	v, e := q.blockingQuery(query, r)
	if len(v) < 1 || !v[0].IsValid() {
		return
	}
	return v[0].Interface().(types.Node), e
}

// ReadAll will get a slice of all nodes from the Cfg state
func (q *QueryEngine) ReadAll() (nc []types.Node, e error) {
	query, r := NewQuery(
		types.Query_READALL,
		types.QueryState_CONFIG,
		"",
		[]reflect.Value{})
	v, e := q.blockingQuery(query, r)
	for _, i := range v {
		nc = append(nc, i.Interface().(types.Node))
	}
	return
}

// ReadAllDsc will get a slice of all nodes from the Dsc state
func (q *QueryEngine) ReadAllDsc() (nc []types.Node, e error) {
	query, r := NewQuery(
		types.Query_READALL,
		types.QueryState_DISCOVER,
		"",
		[]reflect.Value{})
	v, e := q.blockingQuery(query, r)
	for _, i := range v {
		nc = append(nc, i.Interface().(types.Node))
	}
	return
}

// DeleteAll will delete all nodes from the Engine.  !!!DANGEROUS!!!
func (q *QueryEngine) DeleteAll() (nc []types.Node, e error) {
	query, r := NewQuery(
		types.Query_DELETEALL,
		types.QueryState_BOTH,
		"",
		[]reflect.Value{})
	v, e := q.blockingQuery(query, r)
	for _, i := range v {
		nc = append(nc, i.Interface().(types.Node))
	}
	return
}

// GetValue will get a value from the Cfg state via a URL
func (q *QueryEngine) GetValue(url string) (v reflect.Value, e error) {
	query, r := NewQuery(
		types.Query_GETVALUE,
		types.QueryState_CONFIG,
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
		types.Query_GETVALUE,
		types.QueryState_DISCOVER,
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
		types.Query_SETVALUE,
		types.QueryState_CONFIG,
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
		types.Query_SETVALUE,
		types.QueryState_DISCOVER,
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

func (q *QueryEngine) blockingQuery(query types.Query, r <-chan types.QueryResponse) ([]reflect.Value, error) {
	var qr types.QueryResponse
	var s chan<- types.Query
	t, ok := types.QueryTypeMap[query.Type()]
	if !ok {
		return nil, fmt.Errorf("invalid query type: %v", query.Type())
	}
	switch t {
	case types.Query_SDE:
		s = q.sd
	case types.Query_SME:
		s = q.sm
	default:
		return nil, fmt.Errorf("QueryType %v not mapped to a QueryEngineType", query.Type())
	}
	s <- query
	qr = <-r
	return qr.Value(), qr.Error()
}
