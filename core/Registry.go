/* Registry.go: the registry manages static module and extension properties on initialization
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
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/hpc/kraken/lib"
)

//////////////////////
// Global Variables /
////////////////////

// Registry is where we register modules & extensions
// It provides various internal functions
var Registry = NewKrakenRegistry()

/*
 * Marshaling routines live here because they rely on Registry as an Any resolver
 */

// MarshalJSON is a helper for default JSON marshaling
func MarshalJSON(m proto.Message) ([]byte, error) {
	jm := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "  ",
		OrigName:     false,
		AnyResolver:  Registry,
	}
	j, err := jm.MarshalToString(m)
	return []byte(j), err
}

// UnmarshalJSON is a helper for default JSON unmarshaling
func UnmarshalJSON(in []byte, p proto.Message) error {
	um := &jsonpb.Unmarshaler{
		AllowUnknownFields: false,
		AnyResolver:        Registry,
	}
	return um.Unmarshal(strings.NewReader(string(in)), p)
}

///////////////////////////
// KrakenRegistry Object /
/////////////////////////

var _ jsonpb.AnyResolver = (*KrakenRegistry)(nil)

type KrakenRegistry struct {
	Modules          map[string]lib.Module
	Extensions       map[string]lib.Extension
	ServiceInstances map[string]map[string]lib.ServiceInstance      // s["module"]["instance_id"]
	Discoverables    map[string]map[string]map[string]reflect.Value // d["service_instance"]["property_url"]["value_id"]
	Mutations        map[string]map[string]lib.StateMutation        // m["service_instance"]["mutation_id"]
}

func NewKrakenRegistry() *KrakenRegistry {
	r := &KrakenRegistry{
		Modules:          make(map[string]lib.Module),
		Extensions:       make(map[string]lib.Extension),
		Discoverables:    make(map[string]map[string]map[string]reflect.Value),
		Mutations:        make(map[string]map[string]lib.StateMutation),
		ServiceInstances: make(map[string]map[string]lib.ServiceInstance),
	}
	return r
}

// RegisterModule adds a module to the map if it hasn't been already
// It's probably a good idea for this to be done in init()
func (r *KrakenRegistry) RegisterModule(m lib.Module) {
	if _, ok := r.Modules[m.Name()]; !ok {
		r.Modules[m.Name()] = m
	}
}

// RegisterExtension adds an extension to the map if it hasn't been already
// It's probably a good idea for this to be done in init()
func (r *KrakenRegistry) RegisterExtension(e lib.Extension) {
	if _, ok := r.Extensions[e.Name()]; !ok {
		r.Extensions[e.Name()] = e
	}
}

// RegisterServiceInstance creates a service instance with a particular module.
// Note: This can be done after the fact, but serviceinstances that are added after runtime cannot
// (currently) be used as part of mutation chains.
func (r *KrakenRegistry) RegisterServiceInstance(m lib.Module, d map[string]lib.ServiceInstance) {
	r.ServiceInstances[m.Name()] = d
}

// RegisterDiscoverable adds a map of discoverables the module can emit
func (r *KrakenRegistry) RegisterDiscoverable(si lib.ServiceInstance, d map[string]map[string]reflect.Value) {
	r.Discoverables[si.ID()] = d
}

// RegisterMutations declares mutations a module can perform
func (r *KrakenRegistry) RegisterMutations(si lib.ServiceInstance, d map[string]lib.StateMutation) {
	r.Mutations[si.ID()] = d
}

// Resolve provides a protobuf resolver for module config objects
func (r *KrakenRegistry) Resolve(url string) (proto.Message, error) {
	if e, ok := r.Extensions[url]; ok {
		return e.New(), nil
	}
	for _, m := range r.Modules {
		if m, ok := m.(lib.ModuleWithConfig); ok {
			if m.ConfigURL() == url {
				return m.NewConfig(), nil
			}
		}
	}
	return nil, fmt.Errorf("proto not found")
}
