/* StateSpec.go: defines a state matching critereon used for StateMutation
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

	"github.com/hpc/kraken/lib/types"
)

//////////////////////
// StateSpec Object /
////////////////////

var _ types.StateSpec = (*StateSpec)(nil)

// A StateSpec is essentially a filter that determines if a given state
// falls within the spec or not.  It currently systems of required and excluded
// values for specific URLs.
type StateSpec struct {
	// we actually use maps, not StateValue, for efficiency
	req map[string]reflect.Value
	exc map[string]reflect.Value
}

// NewStateSpec creates a new initialized and fully specified StateSpec
func NewStateSpec(req map[string]reflect.Value, exc map[string]reflect.Value) *StateSpec {
	s := &StateSpec{}
	s.req = req
	s.exc = exc
	return s
}

// Requires returns the map of URLs/values that this spec must have
func (s *StateSpec) Requires() map[string]reflect.Value { return s.req }

// Excludes returns the map of URLs/values that this spec cannot have
func (s *StateSpec) Excludes() map[string]reflect.Value { return s.exc }

// NodeMatch determines of a Node matches a spec
func (s *StateSpec) NodeMatch(n types.Node) (r bool) {
	r = true
	// Are required values correct?
	for u, v := range s.req {
		val, e := n.GetValue(u)
		if e != nil || val.Interface() != v.Interface() {
			return false
		}
	}
	// Are excluded values incorrect?
	for u, v := range s.exc {
		val, e := n.GetValue(u)
		if e == nil && val.Interface() == v.Interface() {
			return false
		}
	}
	return
}

// NodeCompatWithMutators is how we find endpoints for mutation paths
// 1) For each mutator that is in the node, the spec must be equal
// 2) For each requires in the spec that is not a mutator, node must be equal
// 3) For each excludes in the spec that is not a mutator, node must not be equal
func (s *StateSpec) NodeCompatWithMutators(n types.Node, muts map[string]uint32) (r bool) {
	r = true
	// 1) For each mutator that is in the node, the spec must be equal
	for m := range muts {
		v, e := n.GetValue(m)
		if e == nil && v.IsValid() && v.Interface() != reflect.Zero(v.Type()).Interface() { // valid & non-zero
			// must be in spec requires
			sv, ok := s.req[m]
			if !ok {
				return false
			}
			if sv.Interface() != v.Interface() {
				return false
			}
		}
	}
	// 2) For each requires in the spec that is not a mutator, node must be equal
	for u, v := range s.req {
		if _, ok := muts[u]; ok {
			continue
		}
		nv, e := n.GetValue(u)
		if e != nil || nv.Interface() != v.Interface() {
			return false
		}
	}
	// 3) For each excludes in the spec that is not a mutator, node must not be equal (should this check for mutator status?)
	for u, v := range s.exc {
		nv, e := n.GetValue(u)
		if e == nil && nv.Interface() == v.Interface() {
			return false
		}
	}
	return
}

// NodeMatchWithMutators is like NodeMatch, but requires any mutators be present and match
// if they are non-zero in the node
func (s *StateSpec) NodeMatchWithMutators(n types.Node, muts map[string]uint32) bool {
	for m := range muts {
		val, e := n.GetValue(m)
		if e == nil && val.IsValid() && val.Interface() != reflect.Zero(val.Type()).Interface() {
			if v, ok := s.req[m]; ok {
				if v.Interface() != val.Interface() {
					return false
				}
			} else {
				return false
			}
		}
	}
	return s.NodeMatch(n)
}

// SpecCompat determines if two specs are compatible.
// Note: this doesn't mean that a Node that matches one will match
//  the other; it's just possible that they would.
func (a *StateSpec) SpecCompat(b types.StateSpec) (r bool) {
	r = true
	req := b.Requires()
	exc := b.Excludes()
	for u, v := range a.req {
		// are any a requires incompatible with b requires
		if val, ok := req[u]; ok {
			if val.Interface() != v.Interface() {
				return false
			}
		}
		// are any a requires in b excludes
		if val, ok := exc[u]; ok {
			if val.Interface() == v.Interface() {
				return false
			}
		}
	}
	for u, v := range req {
		// are any b requires incompatible with a requires
		if val, ok := a.req[u]; ok {
			if val.Interface() != v.Interface() {
				return false
			}
		}
		// are any b requires in a excludes
		if val, ok := a.exc[u]; ok {
			if val.Interface() == v.Interface() {
				return false
			}
		}
	}
	return
}

// SpecMergeMust makes the most specified version of two compbined StateSpecs
func (s *StateSpec) SpecMergeMust(b types.StateSpec) (ns types.StateSpec) {
	creq := make(map[string]reflect.Value)
	cexc := make(map[string]reflect.Value)

	for u, v := range s.req {
		creq[u] = v
	}
	for u, v := range b.Requires() {
		creq[u] = v
	}
	for u, v := range s.exc {
		if r, ok := creq[u]; ok && r.Interface() == v.Interface() {
			// exc and req can't contradict each other, req wins
			continue
		}
		cexc[u] = v
	}
	for u, v := range b.Excludes() {
		if r, ok := creq[u]; ok && r.Interface() == v.Interface() {
			// exc and req can't contradict each other, req wins
			continue
		}
		cexc[u] = v
	}

	ns = NewStateSpec(creq, cexc)
	return
}

// SpecMerge is the same as SpecMergeMust, but don't assume compat, and return an error if not
func (s *StateSpec) SpecMerge(b types.StateSpec) (ns types.StateSpec, e error) {
	if !s.SpecCompat(b) {
		e = fmt.Errorf("cannot merge incompatible StateSpecs")
		return
	}
	ns = s.SpecMergeMust(b)
	return
}

// LeastCommon keeps only the values that are also in the supplied spec
func (s *StateSpec) LeastCommon(b types.StateSpec) {
	for u, v := range b.Requires() {
		if sv, ok := s.req[u]; ok {
			if v.Interface() != sv.Interface() {
				// not in common, drop it
				delete(s.req, u)
			}
		}
		// else, not in common
	}
	for u, v := range b.Excludes() {
		if sv, ok := s.exc[u]; ok {
			if v.Interface() != sv.Interface() {
				delete(s.exc, u)
			}
		}
	}
}

// ReqsEqual tests if two specs have the same requirements
func (s *StateSpec) ReqsEqual(b types.StateSpec) bool {
	if len(s.Requires()) != len(b.Requires()) {
		return false
	}
	for r, v := range s.Requires() {
		bv, ok := b.Requires()[r]
		if !ok {
			return false
		}
		if v.Interface() != bv.Interface() {
			return false
		}
	}
	return true
}

// ExcsEqual tests if two specs have the same excludes
func (s *StateSpec) ExcsEqual(b types.StateSpec) bool {
	if len(s.Excludes()) != len(b.Excludes()) {
		return false
	}
	for r, v := range s.Excludes() {
		bv, ok := b.Excludes()[r]
		if !ok {
			return false
		}
		if v.Interface() != bv.Interface() {
			return false
		}
	}
	return true
}

// Equal tests if two specs are identical.  Note: DeepEqual doesn't work because it doesn't turn vals into interfaces.
func (s *StateSpec) Equal(b types.StateSpec) bool {
	if !s.ReqsEqual(b) {
		return false
	}
	if !s.ExcsEqual(b) {
		return false
	}
	return true
}

// StripZeros removes any reqs/excs that are equal to zero value
func (s *StateSpec) StripZeros() {
	for i, v := range s.req {
		if v.IsZero() {
			delete(s.req, i)
		}
	}
	for i, v := range s.exc {
		if v.IsZero() {
			delete(s.req, i)
		}
	}
}
