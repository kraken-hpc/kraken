/* State.go: provides an intermediate between Node and Engine
 *           States store collections of Nodes, and implement node:/node/value URL resolution.
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
	"sync"

	ct "github.com/hpc/kraken/core/proto/customtypes"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

//////////////////
// State Object /
////////////////

var _ types.CRUD = (*State)(nil)
var _ types.BulkCRUD = (*State)(nil)
var _ types.Resolver = (*State)(nil)
var _ types.State = (*State)(nil)

// A State stores and manipulates a collection of Nodes
type State struct {
	nodesMutex *sync.RWMutex
	nodes      map[string]*Node
}

// NewState creates an initialized state
func NewState() *State {
	s := &State{}
	s.nodes = make(map[string]*Node)
	s.nodesMutex = &sync.RWMutex{}
	return s
}

/*
 * CRUD funcs
 */

// Create creates a node in the state
func (s *State) Create(n types.Node) (r types.Node, e error) {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()

	idstr := n.ID().String()
	if _, ok := s.nodes[idstr]; ok {
		e = fmt.Errorf("not creating node with duplicate ID: %s", idstr)
		return
	}
	s.nodes[idstr] = n.(*Node)
	r = s.nodes[idstr]
	return
}

// Read returns a node from the state
func (s *State) Read(nid types.NodeID) (r types.Node, e error) {
	s.nodesMutex.RLock()
	defer s.nodesMutex.RUnlock()

	idstr := nid.String()
	if v, ok := s.nodes[idstr]; ok {
		r = v
		return
	}
	e = fmt.Errorf("no node found by id: %s", idstr)
	return
}

// Update updates a node in the state
func (s *State) Update(n types.Node) (r types.Node, e error) {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()

	idstr := n.ID().String()
	if _, ok := s.nodes[idstr]; ok {
		s.nodes[idstr] = n.(*Node)
		r = s.nodes[idstr]
		return
	}
	e = fmt.Errorf("could not update node, id does not exist: %s", idstr)
	return
}

// Delete removes a node from the state
func (s *State) Delete(n types.Node) (r types.Node, e error) {
	return s.DeleteByID(n.ID())
}

// DeleteByID deletes a node from the state keyed by NodeID
func (s *State) DeleteByID(nid types.NodeID) (r types.Node, e error) {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()

	idstr := nid.String()
	if v, ok := s.nodes[idstr]; ok {
		delete(s.nodes, idstr)
		r = v
		return
	}
	e = fmt.Errorf("could not delete non-existent node: %s", idstr)
	return
}

/*
 * BulkCRUD funcs
 */

// BulkCreate creates multiple nodes
func (s *State) BulkCreate(ns []types.Node) (r []types.Node, e error) {
	return bulkCRUD(ns, s.Create)
}

// BulkRead reads multiple nodes
func (s *State) BulkRead(nids []types.NodeID) (r []types.Node, e error) {
	return bulkCRUDByID(nids, s.Read)
}

// BulkUpdate updates multiple nodes
func (s *State) BulkUpdate(ns []types.Node) (r []types.Node, e error) {
	return bulkCRUD(ns, s.Update)
}

// BulkDelete removes multiple nodes
func (s *State) BulkDelete(ns []types.Node) (r []types.Node, e error) {
	return bulkCRUD(ns, s.Delete)
}

// BulkDeleteByID removes multiple nodes keyed by NodeID
func (s *State) BulkDeleteByID(nids []types.NodeID) (r []types.Node, e error) {
	return bulkCRUDByID(nids, s.DeleteByID)
}

// ReadAll returns a slice of all nodes from the state
func (s *State) ReadAll() (r []types.Node, e error) {
	s.nodesMutex.RLock()
	defer s.nodesMutex.RUnlock()

	for _, v := range s.nodes {
		r = append(r, v)
	}
	return
}

// DeleteAll will remove all nodes from the state
func (s *State) DeleteAll() (r []types.Node, e error) {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()

	r, e = s.ReadAll()
	s.nodes = make(map[string]*Node)
	return
}

/*
 * Resolver funcs
 */

// GetValue will query a property with URL, where node is mapped by ID
func (s *State) GetValue(url string) (r reflect.Value, e error) {
	n, _, sub, e := s.resolveNode(url)
	if e != nil {
		return
	}
	r, e = n.(*Node).GetValue(sub)
	return
}

// SetValue will set a property with URL, where node is mapped by ID
func (s *State) SetValue(url string, v reflect.Value) (r reflect.Value, e error) {
	n, _, sub, e := s.resolveNode(url)
	if e != nil {
		return
	}
	r, e = n.(*Node).SetValue(sub, v)
	return
}

/*
 * TODO: Index funcs
func (s *State) CreateIndex(key string) (e error)                                        { return }
func (s *State) DeleteIndex(key string) (e error)                                        { return }
func (s *State) RebuildIndex(key string) (e error)                                       { return }
func (s *State) QueryIndex(key string, value string) (ns []types.IndexableNode, e error) { return }
*/

/*
 * TODO: Queryable funcs
func (s *State) Search(key string, value reflect.Value) (r []string)                        { return }
func (s *State) QuerySelect(query string) (r []types.Node, e error)                         { return }
func (s *State) QueryUpdate(query string, value reflect.Value) (r []reflect.Value, e error) { return }
func (s *State) QueryDelete(query string) (r []types.Node, e error)                         { return }
*/

////////////////////////
// Unexported methods /
//////////////////////

/*
 * resolveNode is a way to separate URL -> Node resolver
 * n - the node resolved (if any)
 * root - the root that matched the node
 * sub - the sub-URL remaining with root removed (has leading /)
 * e - error should be nil on success. Should always be set if no node found.
 */
func (s *State) resolveNode(url string) (n types.Node, root, sub string, e error) {
	root, sub = util.NodeURLSplit(url)
	n, e = s.Read(ct.NewNodeIDFromURL(root))
	if n == nil {
		e = fmt.Errorf("no such node: %s", root)
	}
	return
}

func bulkCRUD(ns []types.Node, f func(types.Node) (types.Node, error)) (r []types.Node, e error) {
	for _, n := range ns {
		var ret types.Node
		ret, e = f(n)
		if e != nil {
			return
		}
		r = append(r, ret)
	}
	return
}

func bulkCRUDByID(ns []types.NodeID, f func(types.NodeID) (types.Node, error)) (r []types.Node, e error) {
	for _, n := range ns {
		var ret types.Node
		ret, e = f(n)
		if e != nil {
			return
		}
		r = append(r, ret)
	}
	return
}
