/* StateMutation.go: a state mutation describes a mutation of state
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"reflect"
	"time"

	"github.com/hpc/kraken/lib"
)

//////////////////////////
// StateMutation Object /
////////////////////////

var _ lib.StateMutation = (*StateMutation)(nil)

// A StateMutation describes a possible mutation of state
// These are declared by modules
// These are used to construct the state evolution graph
type StateMutation struct {
	mut     map[string][2]reflect.Value
	context lib.StateMutationContext
	base    *StateSpec // this is the spec, less the mutation value
	timeout time.Duration
	failto  [3]string
}

// NewStateMutation creates an initialized, specified StateMutation object
func NewStateMutation(mut map[string][2]reflect.Value, req map[string]reflect.Value, exc map[string]reflect.Value, context lib.StateMutationContext, timeout time.Duration, failto [3]string) *StateMutation {
	for u := range mut {
		if _, ok := req[u]; ok {
			// FIXME: this should probably error out, but we just fix the problem
			delete(req, u)
		}
		if _, ok := exc[u]; ok {
			// FIXME: this should probably error out, but we just fix the problem
			delete(exc, u)
		}
	}
	r := &StateMutation{}
	r.mut = mut
	r.base = NewStateSpec(req, exc)
	r.context = context
	r.timeout = timeout
	r.failto = failto
	return r
}

// Mutates returns the map of URLs/values (before & after) that mutate in this mutation
func (s *StateMutation) Mutates() map[string][2]reflect.Value { return s.mut }

// Requires returns the map of URLs/values that are required for this mutation
func (s *StateMutation) Requires() map[string]reflect.Value { return s.base.Requires() }

// Excludes returns the map of URLs/values that are mutally exclusive with this mutation
func (s *StateMutation) Excludes() map[string]reflect.Value { return s.base.Excludes() }

// Context specifies in which context (Self/Child/All) this mutation applies to
// Note: this doesn't affect the graph; just who does the work.
func (s *StateMutation) Context() lib.StateMutationContext { return s.context }

// Before returns a StatSpec representing the state of of a matching Node before the mutation
func (s *StateMutation) Before() lib.StateSpec {
	r := make(map[string]reflect.Value)
	for u, v := range s.mut {
		r[u] = v[0]
	}
	before, _ := s.base.SpecMerge(NewStateSpec(r, make(map[string]reflect.Value)))
	return before
}

// After returns a StatSpec representing the state of of a matching Node after the mutation
func (s *StateMutation) After() lib.StateSpec {
	r := make(map[string]reflect.Value)
	for u, v := range s.mut {
		r[u] = v[1]
	}
	after, _ := s.base.SpecMerge(NewStateSpec(r, make(map[string]reflect.Value)))
	return after
}

// CanMutateNode returns whether a particular node is eligable for mutation by this StateMutation
func (s *StateMutation) CanMutateNode(n lib.Node) (r bool) {
	return s.Before().NodeMatch(n)
}

// MutationCompat determines if two mutations are compatible to be connected
// This is non-commutative.  Is the beginning of b compat with end of s?
func (s *StateMutation) MutationCompat(b lib.StateMutation) (r bool) {
	return b.SpecCompat(s.After())
}

// SpecCompat determines if a spec is compatable with the beginning of a mutation
// Unlike state specs, this is not commutative
func (s *StateMutation) SpecCompat(sp lib.StateSpec) bool {
	if !s.Before().SpecCompat(sp) {
		return false
	}
	spr := sp.Requires()
	// we have stronger requirements for mutation urls
	for u, v := range s.Mutates() {
		spv, ok := spr[u]
		if !ok {
			// make a zero value of the type we're comparing
			spv = reflect.Indirect(reflect.New(v[0].Type()))
		}
		if spv.Interface() != v[0].Interface() {
			return false
		}
	}
	return true
}

// SpecCompatWithMutators adds to the requirements of SpecCompat that any requires that is also
// a mutator must be present and equal
func (s *StateMutation) SpecCompatWithMutators(sp lib.StateSpec, muts map[string]uint32) bool {
	if !s.SpecCompat(sp) {
		return false
	}
	for r := range s.Requires() {
		if _, ok := muts[r]; ok { // our requires is also a mutator
			spv, ok := sp.Requires()[r]
			if !ok {
				return false
			}
			if spv.Interface() != s.Requires()[r].Interface() {
				return false
			}
		}
	}
	return true
}

func (s *StateMutation) Timeout() time.Duration { return s.timeout }

func (s *StateMutation) FailTo() [3]string { return s.failto }
