/* UUID_type.go: define methods to make a CustomType that stores UUIDs
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package customtypes

import (
	"strconv"
	"strings"

	uuid "github.com/satori/go.uuid"
)

// UUID implements a CustomType as well as a NodeID
// We can't test for that here because importing util/types would cause a loop

type NodeID struct {
	uuid.UUID
}

func (u NodeID) Marshal() ([]byte, error) {
	return u.MarshalBinary()
}

func (u NodeID) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(u.String())), nil
}

func (u *NodeID) MarshalTo(data []byte) (int, error) {
	copy(data, u.Bytes())
	return len(u.Bytes()), nil
}

func (u *NodeID) Unmarshal(data []byte) error {
	return u.UnmarshalBinary(data)
}

func (u *NodeID) Size() int { return len(u.Bytes()) }

func (u *NodeID) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), "\"'")
	return u.UnmarshalText([]byte(s))
}

func (u *NodeID) EqualTo(i interface{}) bool {
	if u2, ok := i.(*NodeID); ok && u2 != nil {
		return uuid.Equal(u.UUID, u2.UUID)
	}
	return false
}

func (u NodeID) Equal(u2 NodeID) bool {
	return uuid.Equal(u.UUID, u2.UUID)
}

func (u NodeID) Compare(u2 NodeID) int {
	return strings.Compare(u.String(), u2.String())
}

func (u *NodeID) Nil() bool {
	return uuid.Equal(u.UUID, uuid.Nil)
}

func NewNodeID(id string) *NodeID {
	u := uuid.FromStringOrNil(id)
	return &NodeID{u}
}

func NewNodeIDFromBinary(b []byte) *NodeID {
	u := uuid.FromBytesOrNil(b)
	return &NodeID{u}
}

func NewNodeIDFromURL(url string) *NodeID {
	id, _ := NodeURLSplit(url)
	return NewNodeID(id)
}

// we can't import this from util
func NodeURLSplit(s string) (node string, url string) {
	ret := strings.SplitN(s, ":", 2)
	switch len(ret) {
	case 0: // empty string
		node = ""
		url = ""
	case 1: // we have just a node?
		node = ret[0]
		url = ""
	case 2:
		node = ret[0]
		url = ret[1]
		break
	}
	return
}
