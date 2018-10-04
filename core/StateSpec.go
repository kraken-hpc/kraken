/* StateSpec.go: defines a state matching critereon used for StateMutation
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

//////////////////////
// StateSpec Object /
////////////////////

var _ lib.StateSpec = (*StateSpec)(nil)

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
func (s *StateSpec) NodeMatch(n lib.Node) (r bool) {
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

// NodeCompat is like NodeMatch, but if a required value is zero still matches
func (s *StateSpec) NodeCompat(n lib.Node) (r bool) {
	r = true
	// Are required values correct?
	for u, v := range s.req {
		val, e := n.GetValue(u)
		if e != nil || val.Interface() != v.Interface() {
			if val.Interface() == reflect.Zero(val.Type()).Interface() { // value just isn't set?
				continue
			}
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

// NodeMatchWithMutators is like NodeMatch, but requires any mutators be present and match
// if they are non-zero in the node
func (s *StateSpec) NodeMatchWithMutators(n lib.Node, muts map[string]uint32) bool {
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
func (a *StateSpec) SpecCompat(b lib.StateSpec) (r bool) {
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
func (s *StateSpec) SpecMergeMust(b lib.StateSpec) (ns lib.StateSpec) {
	creq := make(map[string]reflect.Value)
	cexc := make(map[string]reflect.Value)

	for u, v := range s.req {
		creq[u] = v
	}
	for u, v := range b.Requires() {
		creq[u] = v
	}
	for u, v := range s.exc {
		cexc[u] = v
	}
	for u, v := range b.Excludes() {
		cexc[u] = v
	}

	ns = NewStateSpec(creq, cexc)
	return
}

// SpecMerge is the same as SpecMergeMust, but don't assume compat, and return an error if not
func (s *StateSpec) SpecMerge(b lib.StateSpec) (ns lib.StateSpec, e error) {
	if !s.SpecCompat(b) {
		e = fmt.Errorf("cannot merge incompatible StateSpecs")
		return
	}
	ns = s.SpecMergeMust(b)
	return
}

// Equal tests if two specs are identical.  Note: DeepEqual doesn't work because it doesn't turn vals into interfaces.
func (s *StateSpec) Equal(b lib.StateSpec) bool {
	if len(s.Requires()) != len(b.Requires()) {
		return false
	}
	if len(s.Excludes()) != len(b.Excludes()) {
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
