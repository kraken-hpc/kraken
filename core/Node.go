/* Node.go: nodes are basic data containers for the state store
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I proto/include -I proto --go_out=plugins=grpc:proto proto/ServiceInstance.proto
//go:generate protoc -I proto/include -I proto --go_out=plugins=grpc:proto proto/Node.proto

package core

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
)

/////////////////
// Node Object /
///////////////

var _ lib.Node = (*Node)(nil)

// A Node object is the basic data store of the state engine. It is also a wrapper for a protobuf object.
type Node struct {
	pb    *pb.Node                 // data lives here
	exts  map[string]proto.Message // for internal bookkeeping
	srvs  map[string]lib.ServiceInstance
	mutex *sync.Mutex
}

// NewNodeWithID creates a new node with an ID pre-set
func NewNodeWithID(id string) *Node {
	//n := newNode()
	n := NewNodeFromJSON([]byte(nodeFixture))
	n.pb.Id = NewNodeID(id).Binary()
	return n
}

// NewNodeFromJSON creates a new node from JSON bytes
func NewNodeFromJSON(j []byte) *Node {
	n := newNode()
	e := UnmarshalJSON(j, n.pb)
	if e != nil {
		fmt.Printf("UnmarshJSON failed: %v\n", e)
		return nil
	}
	n.importExtensions()
	n.importServices()
	return n
}

// NewNodeFromBinary creates a new node from Binary (proto)
func NewNodeFromBinary(b []byte) *Node {
	n := newNode()
	e := proto.Unmarshal(b, n.pb)
	// this could error out; we return nil if it does
	if e != nil {
		return nil
	}
	n.importExtensions()
	n.importServices()
	return n
}

// NewNodeFromMessage creats a new node based on a proto message
func NewNodeFromMessage(m *pb.Node) *Node {
	n := newNode()
	n.pb = m
	n.importExtensions()
	n.importServices()
	return n
}

// ID returns the NodeID object for the node
func (n *Node) ID() lib.NodeID {
	return NewNodeIDFromBinary(n.pb.Id)
}

func (n *Node) ParentID() lib.NodeID {
	return NewNodeIDFromBinary(n.pb.ParentId)
}

// JSON returns a JSON representation of the node
func (n *Node) JSON() []byte {
	n.exportExtensions()
	n.exportServices()
	b, _ := MarshalJSON(n.pb)
	n.importExtensions()
	n.importServices()
	return b
}

// Binary returns a Binary representation of the node
func (n *Node) Binary() []byte {
	n.exportExtensions()
	n.exportServices()
	// If we're doing our job, this should never error.
	b, _ := proto.Marshal(n.pb)
	n.importExtensions()
	n.importServices()
	return b
}

func (n *Node) Message() proto.Message {
	n.exportExtensions()
	n.exportServices()
	m := proto.Clone(n.pb)
	n.importExtensions()
	n.importServices()
	return m
}

// GetValue returns a specific value (reflect.Value) by URL
func (n *Node) GetValue(url string) (v reflect.Value, e error) {
	root, sub := lib.URLShift(url)
	switch root {
	case "type.googleapis.com": // resolve extension
		p, sub := lib.URLShift(sub)
		ext, ok := n.exts[lib.URLPush(root, p)]
		if !ok {
			e = fmt.Errorf("node does not have extension: %s", lib.URLPush(root, p))
			return
		}
		return lib.ResolveURL(sub, reflect.ValueOf(ext))
	case "Services": // resolve service
		p, sub := lib.URLShift(sub)
		srv := n.GetService(p)
		if srv == nil {
			e = fmt.Errorf("nodes does not have service instance: %s", p)
			return
		}
		return lib.ResolveURL(sub, reflect.ValueOf(srv.Message()))
	default: // everything else
		return lib.ResolveURL(url, reflect.ValueOf(n.pb))
	}
}

// SetValue sets a specific value (reflect.Value) by URL
// Returns the value, post-set (same if input if all went well)
func (n *Node) SetValue(url string, value reflect.Value) (v reflect.Value, e error) {
	var r reflect.Value
	r, e = n.GetValue(url)
	if e != nil {
		return
	}
	if r.Type() != value.Type() {
		e = fmt.Errorf("type mismatch: %s != %s", value.Type(), r.Type())
		return
	}
	r.Set(value)
	v = r
	return
}

// GetValues gets multiple values in one call
func (n *Node) GetValues(urls []string) (v map[string]reflect.Value) {
	v = make(map[string]reflect.Value)
	for _, url := range urls {
		t, e := n.GetValue(url)
		if e != nil {
			v[url] = t
		}
	}
	return
}

// SetValues sets multiple values.
// TODO: Need a way to dynamically added new sub-structs
func (n *Node) SetValues(valmap map[string]reflect.Value) (v map[string]reflect.Value) {
	v = make(map[string]reflect.Value)
	for url, val := range valmap {
		t, e := n.SetValue(url, val)
		if e != nil {
			v[url] = t
		}
	}
	return
}

// GetExtensionURLs returns a slice of currently added extensions
func (n *Node) GetExtensionURLs() (r []string) {
	exts := []string{}
	for u := range n.exts {
		exts = append(exts, u)
	}
	return exts
}

// AddExtension adds a new extension to the node.  It will fail if marshal fails, or if it's a dupe.
func (n *Node) AddExtension(m proto.Message) (e error) {
	any, e := ptypes.MarshalAny(m)
	if e != nil {
		return e
	}
	url := any.GetTypeUrl()
	n.mutex.Lock()
	if _, ok := n.exts[url]; ok {
		e = fmt.Errorf("duplicate extension: %s", url)
		return
	}
	n.exts[url] = m
	n.mutex.Unlock()
	return
}

// DelExtension removes an extension from the node.  Has no return value, even if extension isn't there.
func (n *Node) DelExtension(url string) {
	n.mutex.Lock()
	delete(n.exts, url)
	n.mutex.Unlock()
}

// HasExtension determines if the node has an extension by URL
func (n *Node) HasExtension(url string) bool {
	_, ok := n.exts[url]
	return ok
}

func (n *Node) GetServiceIDs() (r []string) {
	for k := range n.srvs {
		r = append(r, k)
	}
	return r
}

func (n *Node) GetServices() (r []lib.ServiceInstance) {
	for _, srv := range n.srvs {
		r = append(r, srv)
	}
	return
}

func (n *Node) AddService(si lib.ServiceInstance) (e error) {
	n.mutex.Lock()
	if _, ok := n.srvs[si.ID()]; ok {
		return fmt.Errorf("duplicate service: %s", si.ID())
	}
	n.srvs[si.ID()] = si
	n.mutex.Unlock()
	return
}

func (n *Node) DelService(id string) {
	n.mutex.Lock()
	if _, ok := n.srvs[id]; ok {
		delete(n.srvs, id)
	}
	n.mutex.Unlock()
}

func (n *Node) GetService(id string) (r lib.ServiceInstance) {
	var ok bool
	if r, ok = n.srvs[id]; ok {
		return r
	}
	return nil
}

func (n *Node) HasService(id string) bool {
	if _, ok := n.srvs[id]; ok {
		return true
	}
	return false
}

// Diff finds URLs that are different between this Node and another
// prefix allows a string prefix to be prepended to diffs
func (n *Node) Diff(node lib.Node, prefix string) (r []string, e error) {
	if reflect.TypeOf(n) != reflect.TypeOf(node) {
		e = fmt.Errorf("cannot diff nodes of different types")
		return
	}
	m := node.(*Node)
	r, e = lib.MessageDiff(n.pb, m.pb, prefix)

	// handle extensions
	eleft := m.GetExtensionURLs()
	for _, u := range n.GetExtensionURLs() {
		nodeExt, ok := m.exts[u]
		if !ok {
			r = append(r, lib.URLPush(prefix, u))
			continue
		}
		d, _ := lib.MessageDiff(n.exts[u], nodeExt, lib.URLPush(prefix, u))
		r = append(r, d...)
		for i := range eleft {
			if eleft[i] == u {
				eleft = append(eleft[:i], eleft[i+1:]...)
				break
			}
		}
	}
	for _, u := range eleft {
		r = append(r, lib.URLPush(prefix, u))
	}

	// handle services
	prefix = lib.URLPush(prefix, "Services")
	sleft := m.GetServiceIDs()
	for _, u := range n.GetServiceIDs() {
		nodeSrv, ok := m.srvs[u]
		if !ok { // new one doesn't have this
			r = append(r, lib.URLPush(prefix, u))
			continue
		}
		d, _ := lib.MessageDiff(n.srvs[u].Message(), nodeSrv.Message(), lib.URLPush(prefix, u))
		r = append(r, d...)
		for i := range sleft {
			if sleft[i] == u {
				sleft = append(sleft[:i], sleft[i+1:]...)
				break
			}
		}
	}
	for _, u := range sleft { // these are new services in m
		r = append(r, lib.URLPush(prefix, u))
	}
	return
}

// MergeDiff does a merge if of what is in diff (URLs) only
// it returns a slice of changes made
func (n *Node) MergeDiff(node lib.Node, diff []string) (changes []string, e error) {
	if reflect.TypeOf(n) != reflect.TypeOf(node) {
		e = fmt.Errorf("cannot diff nodes of different types")
		return
	}
	m := node.(*Node)
	for _, d := range diff {
		var vn, vm reflect.Value
		vn, e = n.GetValue(d)
		if e != nil {
			return
		}
		vm, e = m.GetValue(d)
		if e != nil {
			return
		}
		if vm.Interface() == vn.Interface() {
			continue
		}
		vn.Set(vm)
		changes = append(changes, d)
	}
	return
}

// Merge takes any non-nil values in m into n
// We don't use protobuf's merge because we generally want to know what values changed!
// It returns a slice of URLs to changes made
func (n *Node) Merge(node lib.Node, pre string) (changes []string, e error) {
	d, e := n.Diff(node, pre)
	if e != nil {
		return
	}
	return n.MergeDiff(node, d)
}

////////////////////////
// Unexported methods /
//////////////////////

// newNode creates a new, completely empty Node
// We don't even want a way to have a node with no ID, so we don't export.
// We can assign a Nil ID if we have a really good reason
func newNode() *Node {
	n := &Node{}
	n.pb = &pb.Node{}
	n.exts = make(map[string]proto.Message)
	n.srvs = make(map[string]lib.ServiceInstance)
	n.mutex = &sync.Mutex{}
	for _, e := range Registry.Extensions {
		n.AddExtension(e.New())
	}
	for i := range Registry.ServiceInstances {
		for j := range Registry.ServiceInstances[i] {
			si := Registry.ServiceInstances[i][j]
			n.AddService(si)
		}
	}
	return n
}

func (n *Node) importExtensions() {
	n.mutex.Lock()
	for _, ext := range n.pb.Extensions {
		// any that error just get thrown out
		var x proto.Message
		var e error
		x, e = Registry.Resolve(ext.GetTypeUrl())
		e = ptypes.UnmarshalAny(ext, x)
		if e == nil {
			// we overwrite duplicates
			n.exts[ext.GetTypeUrl()] = x
		}
	}
	n.mutex.Unlock()
	// now we clear the field
	n.pb.Extensions = []*any.Any{}
}

func (n *Node) exportExtensions() {
	n.mutex.Lock()
	for _, ext := range n.exts {
		any, e := ptypes.MarshalAny(ext)
		if e == nil {
			n.pb.Extensions = append(n.pb.Extensions, any)
		}
	}
	n.mutex.Unlock()
}

func (n *Node) importServices() {
	n.mutex.Lock()
	for _, srv := range n.pb.Services {
		si := NewServiceInstanceFromMessage(srv)
		n.srvs[si.ID()] = si
	}
	n.pb.Services = []*pb.ServiceInstance{}
	n.mutex.Unlock()
}

func (n *Node) exportServices() {
	n.mutex.Lock()
	for _, si := range n.srvs {
		n.pb.Services = append(n.pb.Services, si.Message())
	}
	n.mutex.Unlock()
}

//FIXME: hack to get the default extension, need a better way:

const nodeFixture string = `
{
	"id": "Ej5FZ+ibEtOkVkJmVUQAAA==",
	"nodename": "",
	"runState": "INIT",
	"physState": "PHYS_UNKNOWN",
	"arch": "",
	"platform": "",
	"extensions": [
		]
	  }
	]
  }`
