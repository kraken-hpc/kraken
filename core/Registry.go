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

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/kraken-hpc/kraken/lib/json"
	"github.com/kraken-hpc/kraken/lib/types"
)

//////////////////////
// Global Variables /
////////////////////

// Registry is where we register modules & extensions
// It provides various internal functions
// We need this prior to init(), so it must be a global
var Registry = NewKrakenRegistry()

// Set our registry to be our json resolver
func init() {
	json.Marshaler.AnyResolver = Registry
	json.Unmarshaler.AnyResolver = Registry
}

///////////////////////////
// KrakenRegistry Object /
/////////////////////////

var _ jsonpb.AnyResolver = (*KrakenRegistry)(nil)

type KrakenRegistry struct {
	Modules          map[string]types.Module
	Extensions       map[string]types.Extension
	Discoverables    map[string]map[string]map[string]reflect.Value // d["instance_id"]["property_url"]["value_id"]
	Mutations        map[string]map[string]types.StateMutation      // m["instance_id"]["mutation_id"]
	ServiceInstances map[string]map[string]types.ServiceInstance    // s["module"]["instance_id"]
}

func NewKrakenRegistry() *KrakenRegistry {
	r := &KrakenRegistry{
		Modules:          make(map[string]types.Module),
		Extensions:       make(map[string]types.Extension),
		Discoverables:    make(map[string]map[string]map[string]reflect.Value),
		Mutations:        make(map[string]map[string]types.StateMutation),
		ServiceInstances: make(map[string]map[string]types.ServiceInstance),
	}
	return r
}

// RegisterModule adds an module to the map if it hasn't been already
// It's probably a good idea for this to be done in init()
func (r *KrakenRegistry) RegisterModule(m types.Module) {
	if _, ok := r.Modules[m.Name()]; !ok {
		r.Modules[m.Name()] = m
	}
}

// RegisterExtension adds an extension to the map if it hasn't been already
// It's probably a good idea for this to be done in init()
func (r *KrakenRegistry) RegisterExtension(e types.Extension) {
	if _, ok := r.Extensions[e.Name()]; !ok {
		r.Extensions[e.Name()] = e
	}
}

// RegisterDiscoverable adds a map of discoverables the module can emit
func (r *KrakenRegistry) RegisterDiscoverable(si types.ServiceInstance, d map[string]map[string]reflect.Value) {
	r.Discoverables[si.ID()] = d
}

// RegisterMutations declares mutations a module can perform
func (r *KrakenRegistry) RegisterMutations(si types.ServiceInstance, d map[string]types.StateMutation) {
	r.Mutations[si.ID()] = d
}

// RegisterServiceInstance creates a service instance with a particular module.RegisterServiceInstance
// Note: This can be done after the fact, but serviceinstances that are added after runtime cannot
// (currently) be used as part of mutation chains.
func (r *KrakenRegistry) RegisterServiceInstance(m types.Module, d map[string]types.ServiceInstance) {
	r.ServiceInstances[m.Name()] = d
}

func (r *KrakenRegistry) Resolve(url string) (proto.Message, error) {
	if e, ok := r.Extensions[url]; ok {
		return e.New(), nil
	}
	for _, m := range r.Modules {
		if m, ok := m.(types.ModuleWithConfig); ok {
			if m.ConfigURL() == url {
				return m.NewConfig(), nil
			}
		}
	}
	return nil, fmt.Errorf("proto not found")
}
