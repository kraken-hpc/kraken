/* Node.go: nodes are basic data containers for the state store
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I proto/src -I proto --gogo_out=Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,plugins=grpc:proto proto/src/Node.proto

package core

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	pb "github.com/hpc/kraken/core/proto"
	ct "github.com/hpc/kraken/core/proto/customtypes"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

/////////////////
// Node Object /
///////////////

var _ types.Node = (*Node)(nil)

// A Node object is the basic data store of the state engine. It is also a wrapper for a protobuf object.
type Node struct {
	pb    *pb.Node                       // data lives here
	exts  map[string]proto.Message       // keeps an index map from extension URL -> extension Proto
	srvs  map[string]*pb.ServiceInstance // keeps an index map from service ID -> service Proto
	mutex *sync.RWMutex
}

// NewNodeWithID creates a new node with an ID pre-set
func NewNodeWithID(id string) *Node {
	//n := newNode()
	n := NewNodeFromJSON([]byte(nodeFixture))
	n.pb.Id = ct.NewNodeID(id)
	n.indexServices()
	return n
}

// NewNodeFromJSON creates a new node from JSON bytes
func NewNodeFromJSON(j []byte) *Node {
	n := newNode()
	e := n.pb.UnmarshalJSON(j)
	if e != nil {
		fmt.Printf("UnmarshJSON failed: %v\n", e)
		return nil
	}
	n.importExtensions()
	n.indexServices()
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
	n.indexServices()
	return n
}

// NewNodeFromMessage creats a new node based on a proto message
func NewNodeFromMessage(m *pb.Node) *Node {
	n := newNode()
	n.pb = m
	n.importExtensions()
	n.indexServices()
	return n
}

// ID returns the NodeID object for the node
// Note: we don't lock on this under the assumption that ID's don't typically change
func (n *Node) ID() types.NodeID {
	return n.pb.Id
}

// ParentID returns the NodeID of the parent of this node
func (n *Node) ParentID() (pid types.NodeID) {
	n.mutex.RLock()
	pid = n.pb.ParentId
	n.mutex.RUnlock()
	return
}

// String is important, as we can make sure prints on a Node object obey locking
func (n *Node) String() string {
	return fmt.Sprintf("Node<%s>", n.ID().String())
}

// JSON returns a JSON representation of the node
func (n *Node) JSON() []byte {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.exportExtensions()
	b, e := n.pb.MarshalJSON()
	if e != nil {
		fmt.Printf("error marshalling JSON for node: %v\n", e)
	}
	n.importExtensions()
	return b
}

// Binary returns a Binary representation of the node
func (n *Node) Binary() []byte {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.exportExtensions()
	// If we're doing our job, this should never error.
	b, _ := proto.Marshal(n.pb)
	n.importExtensions()
	return b
}

// Message returns the proto.Message interface for the node
func (n *Node) Message() proto.Message {
	// This is a strange way to do things, but proto.Clone doesn't work with custom types
	// TODO: do this better.
	bytes := n.Binary()
	m := NewNodeFromBinary(bytes)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.exportExtensions()
	return m.pb
}

// GetValue returns a specific value (reflect.Value) by URL
// note: we can't just wrap everything in a lock because n.GetService will lock too
func (n *Node) GetValue(url string) (v reflect.Value, e error) {
	root, sub := util.URLShift(url)
	switch root {
	case "type.googleapis.com":
		fallthrough
	case "/type.googleapis.com": // resolve extension
		p, sub := util.URLShift(sub)
		n.mutex.RLock()
		ext, ok := n.exts[util.URLPush(root, p)]
		if !ok {
			e = fmt.Errorf("node does not have extension: %s", util.URLPush(root, p))
			return
		}
		defer n.mutex.RUnlock()
		return util.ResolveURL(sub, reflect.ValueOf(ext))
	case "Services":
		fallthrough
	case "/Services": // resolve service
		p, sub := util.URLShift(sub)
		srv := n.GetService(p)
		if srv == nil {
			e = fmt.Errorf("nodes does not have service instance: %s", p)
			return
		}
		n.mutex.RLock()
		defer n.mutex.RUnlock()
		return util.ResolveURL(sub, reflect.ValueOf(srv))
	default: // everything else
		n.mutex.RLock()
		defer n.mutex.RUnlock()
		return util.ResolveURL(url, reflect.ValueOf(n.pb))
	}
}

// SetValue sets a specific value (reflect.Value) by URL
// Returns the value, post-set (same if input if all went well)
// note: we can't just wrap everything in a lock because n.GetService will lock too
func (n *Node) SetValue(url string, value reflect.Value) (v reflect.Value, e error) {
	var r reflect.Value
	root, sub := util.URLShift(url)
	switch root {
	case "/type.googleapis.com":
		fallthrough
	case "type.googleapis.com":
		p, sub := util.URLShift(sub)
		ext, ok := n.exts[util.URLPush(root, p)]
		if !ok {
			// ok, if this is a type we know, we'll add it
			extension, ok := Registry.Extensions[util.URLPush(root, p)]
			if !ok {
				e = fmt.Errorf("unknown extension: %s", ext)
				return
			}
			if err := n.AddExtension(extension.New()); e != nil {
				e = fmt.Errorf("failed to add extension: %v", err)
				return
			}
			// ok, new extension added...
		}
		n.mutex.Lock()
		defer n.mutex.Unlock()
		r, e = util.ResolveOrMakeURL(sub, reflect.ValueOf(ext))
	case "/Services":
		fallthrough
	case "Services":
		p, sub := util.URLShift(sub)
		srv := n.GetService(p)
		if srv == nil {
			// we don't create services on the fly like this right now
			e = fmt.Errorf("node does not have service instance: %s", p)
			return
		}
		n.mutex.Lock()
		defer n.mutex.Unlock()
		r, e = util.ResolveOrMakeURL(sub, reflect.ValueOf(srv))
	default:
		n.mutex.Lock()
		defer n.mutex.Unlock()
		r, e = util.ResolveOrMakeURL(url, reflect.ValueOf(n.pb))
	}
	if e != nil {
		return
	}
	if !r.IsValid() {
		panic(url)
	}
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if r.Type() != value.Type() {
		e = fmt.Errorf("type mismatch: %s != %s", value.Type(), r.Type())
		return
	}
	// should already be locked from above
	if !r.CanSet() {
		e = fmt.Errorf("value %s is not settable", url)
		return
	}
	r.Set(value)
	v = r
	return
}

// GetValues gets multiple values in one call
func (n *Node) GetValues(urls []string) (v map[string]reflect.Value, e error) {
	v = make(map[string]reflect.Value)
	for _, url := range urls {
		t, e := n.GetValue(url)
		if e == nil {
			v[url] = t
		} else {
			e = fmt.Errorf("Error occurred while getting value %v: %v", url, e)
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
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	for u := range n.exts {
		exts = append(exts, u)
	}
	return exts
}

// GetExtensions returns the exts map
func (n *Node) GetExtensions() (r map[string]proto.Message) {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.exts
}

// AddExtension adds a new extension to the node.  It will fail if marshal fails, or if it's a dupe.
func (n *Node) AddExtension(m proto.Message) (e error) {
	any, e := ptypes.MarshalAny(m)
	if e != nil {
		return e
	}
	url := any.GetTypeUrl()
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.exts[url]; ok {
		e = fmt.Errorf("duplicate extension: %s", url)
		return
	}
	n.exts[url] = m
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
	n.mutex.RLock()
	_, ok := n.exts[url]
	n.mutex.RUnlock()
	return ok
}

// GetServiceIDs returns a slice of service ID strings
func (n *Node) GetServiceIDs() (r []string) {
	n.mutex.RLock()
	for k := range n.srvs {
		r = append(r, k)
	}
	n.mutex.RUnlock()
	return r
}

// GetServices returns a slice of pb.ServiceInstance objects
func (n *Node) GetServices() (r []*pb.ServiceInstance) {
	n.mutex.RLock()
	for _, srv := range n.srvs {
		r = append(r, srv)
	}
	n.mutex.RUnlock()
	return
}

// AddService adds a ServiceInstance to the node, updates the index
func (n *Node) AddService(si *pb.ServiceInstance) (e error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.srvs[si.Id]; ok {
		return fmt.Errorf("duplicate service: %s", si.Id)
	}
	n.srvs[si.Id] = si
	n.pb.Services = append(n.pb.Services, si)
	return
}

// DelService removes a ServiceInstance from a node
// This does not error if the instance does not exist
func (n *Node) DelService(id string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if si, ok := n.srvs[id]; ok {
		delete(n.srvs, id)
		for i, psi := range n.pb.Services {
			if psi == si {
				// we don't need to preserve order, so we can do a fast (constant time) delete
				n.pb.Services[i] = n.pb.Services[len(n.pb.Services)-1]
				n.pb.Services = n.pb.Services[:len(n.pb.Services)-1]
				break
			}
		}
	}
}

// GetService returns the ServiceInstance by ID
func (n *Node) GetService(id string) (r *pb.ServiceInstance) {
	var ok bool
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	if r, ok = n.srvs[id]; ok {
		return r
	}
	return nil
}

// HasService returns a bool stating if a service exists
func (n *Node) HasService(id string) bool {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	var ok bool
	_, ok = n.srvs[id]
	return ok
}

// Diff finds URLs that are different between this Node and another
// prefix allows a string prefix to be prepended to diffs
// note: we have to be especially careful about locking in this function
func (n *Node) Diff(node types.Node, prefix string) (r []string, e error) {
	if reflect.TypeOf(n) != reflect.TypeOf(node) {
		e = fmt.Errorf("cannot diff nodes of different types")
		return
	}
	m := node.(*Node)

	// These have to live up here to avoid possible deadlocks
	eleft := m.GetExtensionURLs()
	eright := n.GetExtensionURLs()
	sleft := m.GetServiceIDs()
	sright := n.GetServiceIDs()

	n.mutex.RLock()
	m.mutex.RLock()
	defer n.mutex.RUnlock()
	defer m.mutex.RUnlock()
	// !!!IMPORTANT!!! we can't call any functions that lock after this, or we risk a deadlock

	r, e = util.MessageDiff(n.pb, m.pb, prefix)

	// handle extensions
	for _, u := range eright {
		nodeExt, ok := m.exts[u]
		if !ok {
			r = append(r, fmt.Sprintf("%s%s", prefix, u))
			continue
		}
		d, _ := util.MessageDiff(n.exts[u], nodeExt, fmt.Sprintf("%s%s", prefix, u))
		r = append(r, d...)
		for i := range eleft {
			if eleft[i] == u {
				eleft = append(eleft[:i], eleft[i+1:]...)
				break
			}
		}
	}
	for _, u := range eleft {
		r = append(r, fmt.Sprintf("%s%s", prefix, u))
	}

	// handle services
	prefix = util.URLPush(prefix, "Services")
	for _, u := range sright {
		nodeSrv, ok := m.srvs[u]
		if !ok { // new one doesn't have this
			r = append(r, util.URLPush(prefix, u))
			continue
		}
		d, _ := util.MessageDiff(n.srvs[u], nodeSrv, util.URLPush(prefix, u))
		r = append(r, d...)
		for i := range sleft {
			if sleft[i] == u {
				sleft = append(sleft[:i], sleft[i+1:]...)
				break
			}
		}
	}
	for _, u := range sleft { // these are new services in m
		r = append(r, util.URLPush(prefix, u))
	}
	return
}

// MergeDiff does a merge if of what is in diff (URLs) only
// it returns a slice of changes made
func (n *Node) MergeDiff(node types.Node, diff []string) (changes []string, e error) {
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
		m.mutex.RLock()
		n.mutex.Lock()
		if vm.Interface() == vn.Interface() {
			m.mutex.RUnlock()
			n.mutex.Unlock()
			continue
		}
		vn.Set(vm)
		m.mutex.RUnlock()
		n.mutex.Unlock()
		changes = append(changes, d)
	}
	return
}

// Merge takes any non-nil values in m into n
// We don't use protobuf's merge because we generally want to know what values changed!
// It returns a slice of URLs to changes made
func (n *Node) Merge(node types.Node, pre string) (changes []string, e error) {
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
	n.srvs = make(map[string]*pb.ServiceInstance)
	n.mutex = &sync.RWMutex{}
	for _, e := range Registry.Extensions {
		n.AddExtension(e.New())
	}
	n.indexServices()
	return n
}

// Assume n.mutex is locked
func (n *Node) importExtensions() {
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
	// now we clear the field
	n.pb.Extensions = []*ptypes.Any{}
}

// Assume n.mutex is locked
func (n *Node) exportExtensions() {
	n.pb.Extensions = []*ptypes.Any{}
	for _, ext := range n.exts {
		if any, e := ptypes.MarshalAny(ext); e == nil {
			n.pb.Extensions = append(n.pb.Extensions, any)
		}
	}
}

// indexServices will (re)create the srv index
// Assume n.mutex is locked
func (n *Node) indexServices() {
	n.srvs = map[string]*pb.ServiceInstance{}
	for _, si := range n.pb.GetServices() {
		n.srvs[si.Id] = si
	}
	// we always want a stub for every service, so add ones that weren't indexed
	for i := range Registry.ServiceInstances {
		for j := range Registry.ServiceInstances[i] {
			si := Registry.ServiceInstances[i][j]
			if _, ok := n.srvs[si.ID()]; ok {
				continue // we already have this one, skip
			}
			srv := &pb.ServiceInstance{
				Id:     si.ID(),
				Module: si.Module(),
			}
			if mc, ok := Registry.Modules[si.Module()].(types.ModuleWithConfig); ok {
				cfg, e := Registry.Resolve(mc.ConfigURL())
				if e != nil {
					fmt.Printf("MarshalAny failure for service config: %v\n", e)
				}
				cfgType := reflect.ValueOf(cfg).Elem().Type()
				any, e := ptypes.MarshalAny(reflect.New(cfgType).Interface().(proto.Message))
				if e != nil {
					// this shouldn't happen
					fmt.Printf("MarshalAny failure for service config: %v\n", e)
					continue
				}
				srv.Config = any
			}
			n.AddService(srv)
		}
	}
}

//FIXME: hack to get the default extension, need a better way:

const nodeFixture string = `
{
	"id": "123e4567-e89b-12d3-a456-426655440000",
	"nodename": "",
	"runState": "UNKNOWN",
	"physState": "PHYS_UNKNOWN",
	"arch": "",
	"platform": "",
	"busy": "FREE",
	"extensions": [
		]
	  }
	]
  }`
