/* NodeID.go: NodeID's are mandatory, read-only attributes of nodes.
 *            This implementation uses UUIDs for NodeIDs.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"reflect"

	"github.com/hpc/kraken/lib"
	uuid "github.com/satori/go.uuid"
)

///////////////////
// NodeID Object /
/////////////////

/*
 * NodeIDs are intended to be read-only.
 * If you need a new value, create a new one.
 */

var _ lib.NodeID = (*NodeID)(nil)

// NodeID uses UUID for node IDs, the default
type NodeID struct {
	u uuid.UUID
}

// NewNodeID creates a new NodeID object based on the ID string
func NewNodeID(id string) *NodeID {
	u := uuid.FromStringOrNil(id)
	nid := NodeID{
		u: u,
	}
	return &nid
}

// NewNodeIDFromBinary creates a new NodeID object from binary interpretation
func NewNodeIDFromBinary(b []byte) *NodeID {
	u := uuid.FromBytesOrNil(b)
	nid := NodeID{
		u: u,
	}
	return &nid
}

// NewNodeIDFromURL creates a NodeID matching the url string
func NewNodeIDFromURL(url string) *NodeID {
	id, _ := lib.NodeURLSplit(url)
	return NewNodeID(id)
}

// Equal determines if two NodeIDs are equal
func (n *NodeID) Equal(n2 lib.NodeID) bool {
	if reflect.TypeOf(n) != reflect.TypeOf(n2) {
		return false
	}
	return uuid.Equal(n.u, n2.(*NodeID).u)
}

// Binary converts the NodeID to a binary representation in []bytes
func (n *NodeID) Binary() []byte {
	return n.u.Bytes()
}

// String ...
func (n *NodeID) String() string {
	return n.u.String()
}

// Nil determines this NodeID is Nil (effectively: is it valid?)
func (n *NodeID) Nil() bool {
	return uuid.Equal(n.u, uuid.UUID{})
}
