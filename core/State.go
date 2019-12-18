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

	"github.com/hpc/kraken/lib"
)

//////////////////
// State Object /
////////////////

var _ lib.CRUD = (*State)(nil)
var _ lib.BulkCRUD = (*State)(nil)
var _ lib.Resolver = (*State)(nil)
var _ lib.State = (*State)(nil)

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
func (s *State) Create(n lib.Node) (r lib.Node, e error) {
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
func (s *State) Read(nid lib.NodeID) (r lib.Node, e error) {
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
func (s *State) Update(n lib.Node) (r lib.Node, e error) {
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
func (s *State) Delete(n lib.Node) (r lib.Node, e error) {
	return s.DeleteByID(n.ID())
}

// DeleteByID deletes a node from the state keyed by NodeID
func (s *State) DeleteByID(nid lib.NodeID) (r lib.Node, e error) {
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
func (s *State) BulkCreate(ns []lib.Node) (r []lib.Node, e error) {
	return bulkCRUD(ns, s.Create)
}

// BulkRead reads multiple nodes
func (s *State) BulkRead(nids []lib.NodeID) (r []lib.Node, e error) {
	return bulkCRUDByID(nids, s.Read)
}

// BulkUpdate updates multiple nodes
func (s *State) BulkUpdate(ns []lib.Node) (r []lib.Node, e error) {
	return bulkCRUD(ns, s.Update)
}

// BulkDelete removes multiple nodes
func (s *State) BulkDelete(ns []lib.Node) (r []lib.Node, e error) {
	return bulkCRUD(ns, s.Delete)
}

// BulkDeleteByID removes multiple nodes keyed by NodeID
func (s *State) BulkDeleteByID(nids []lib.NodeID) (r []lib.Node, e error) {
	return bulkCRUDByID(nids, s.DeleteByID)
}

// ReadAll returns a slice of all nodes from the state
func (s *State) ReadAll() (r []lib.Node, e error) {
	s.nodesMutex.RLock()
	defer s.nodesMutex.RUnlock()

	for _, v := range s.nodes {
		r = append(r, v)
	}
	return
}

// DeleteAll will remove all nodes from the state
func (s *State) DeleteAll() (r []lib.Node, e error) {
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
func (s *State) QueryIndex(key string, value string) (ns []lib.IndexableNode, e error) { return }
*/

/*
 * TODO: Queryable funcs
func (s *State) Search(key string, value reflect.Value) (r []string)                        { return }
func (s *State) QuerySelect(query string) (r []lib.Node, e error)                         { return }
func (s *State) QueryUpdate(query string, value reflect.Value) (r []reflect.Value, e error) { return }
func (s *State) QueryDelete(query string) (r []lib.Node, e error)                         { return }
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
func (s *State) resolveNode(url string) (n lib.Node, root, sub string, e error) {
	root, sub = lib.NodeURLSplit(url)
	n, e = s.Read(NewNodeIDFromURL(root))
	if n == nil {
		e = fmt.Errorf("no such node: %s", root)
	}
	return
}

func bulkCRUD(ns []lib.Node, f func(lib.Node) (lib.Node, error)) (r []lib.Node, e error) {
	for _, n := range ns {
		var ret lib.Node
		ret, e = f(n)
		if e != nil {
			return
		}
		r = append(r, ret)
	}
	return
}

func bulkCRUDByID(ns []lib.NodeID, f func(lib.NodeID) (lib.Node, error)) (r []lib.Node, e error) {
	for _, n := range ns {
		var ret lib.Node
		ret, e = f(n)
		if e != nil {
			return
		}
		r = append(r, ret)
	}
	return
}
