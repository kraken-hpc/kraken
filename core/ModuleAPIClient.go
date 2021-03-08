/* ModuleAPIClient.go: provides wrappers to make the API easier to use for Go modules.
 *               note: you can use the API without this; it's just a helper.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"context"
	"fmt"
	"reflect"
	"time"

	ptypes "github.com/gogo/protobuf/types"

	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
	"google.golang.org/grpc"
)

var _ types.ModuleAPIClient = (*ModuleAPIClient)(nil)

type ModuleAPIClient struct {
	sock    string
	self    types.NodeID
	logChan chan LoggerEvent
	log     types.Logger
}

func NewModuleAPIClient(sock string) *ModuleAPIClient {
	a := &ModuleAPIClient{
		sock: sock,
	}
	return a
}

func (a *ModuleAPIClient) Self() types.NodeID { return a.self }

func (a *ModuleAPIClient) SetSelf(s types.NodeID) { a.self = s }

func (a *ModuleAPIClient) QueryCreate(n types.Node) (r types.Node, e error) {
	q := &pb.Query{
		Payload: &pb.Query_Node{
			Node: n.Message().(*pb.Node),
		},
	}
	rv, e := a.oneshot("QueryCreate", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *ModuleAPIClient) QueryRead(id string) (r types.Node, e error) {
	q := &pb.Query{URL: id}
	rv, e := a.oneshot("QueryRead", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *ModuleAPIClient) QueryReadDsc(id string) (r types.Node, e error) {
	q := &pb.Query{URL: id}
	rv, e := a.oneshot("QueryReadDsc", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *ModuleAPIClient) QueryUpdate(n types.Node) (r types.Node, e error) {
	q := &pb.Query{
		Payload: &pb.Query_Node{
			Node: n.Message().(*pb.Node),
		},
	}
	rv, e := a.oneshot("QueryUpdate", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *ModuleAPIClient) QueryUpdateDsc(n types.Node) (r types.Node, e error) {
	q := &pb.Query{
		Payload: &pb.Query_Node{
			Node: n.Message().(*pb.Node),
		},
	}
	rv, e := a.oneshot("QueryUpdateDsc", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *ModuleAPIClient) querySetValuesByKind(dsc bool, id string, values map[string]interface{}) (ret map[string]interface{}, err error) {
	n := NewNodeWithID(id)
	q := &pb.Query{
		Filter: []string{},
	}
	for k, v := range values {
		if _, err = n.SetValue(k, reflect.ValueOf(v)); err != nil {
			return nil, err
		}
		q.Filter = append(q.Filter, k)
	}
	q.Payload = &pb.Query_Node{
		Node: n.Message().(*pb.Node),
	}
	var rv reflect.Value
	if dsc {
		rv, err = a.oneshot("QueryUpdateDsc", reflect.ValueOf(q))
	} else {
		rv, err = a.oneshot("QueryUpdate", reflect.ValueOf(q))
	}
	rq := rv.Interface().(*pb.Query)
	if rq.Filter == nil {
		return
	}
	ret = make(map[string]interface{})
	for _, k := range rq.Filter {
		ret[k] = values[k]
	}
	return
}

// QuerySetValues sets a specified list of configuration state values on node ID
func (a *ModuleAPIClient) QuerySetValues(id string, values map[string]interface{}) (ret map[string]interface{}, err error) {
	return a.querySetValuesByKind(false, id, values)
}

// QuerySetValue sets a single configurate state value on node ID
func (a *ModuleAPIClient) QuerySetValue(id string, url string, value interface{}) (err error) {
	vs, err := a.QuerySetValues(id, map[string]interface{}{url: value})
	if err != nil {
		return err
	}
	if _, ok := vs[url]; !ok {
		return fmt.Errorf("value not set: %s", url)
	}
	return
}

// QuerySetValuesDsc sets a specified list of discoverable state values on node ID
func (a *ModuleAPIClient) QuerySetValuesDsc(id string, values map[string]interface{}) (ret map[string]interface{}, err error) {
	return a.querySetValuesByKind(true, id, values)
}

// QuerySetValueDsc sets a single discoverable state value on node ID
func (a *ModuleAPIClient) QuerySetValueDsc(id string, url string, value interface{}) (err error) {
	vs, err := a.QuerySetValuesDsc(id, map[string]interface{}{url: value})
	if err != nil {
		return err
	}
	if _, ok := vs[url]; !ok {
		return fmt.Errorf("value not set: %s", url)
	}
	return
}

func (a *ModuleAPIClient) queryGetValuesByKind(dsc bool, id string, urls []string) (values map[string]interface{}, err error) {
	var n types.Node
	if dsc {
		n, err = a.QueryReadDsc(id)
	} else {
		n, err = a.QueryRead(id)
	}
	if err != nil {
		return
	}
	vs, err := n.GetValues(urls)
	if err != nil {
		return
	}
	values = make(map[string]interface{})
	for k, v := range vs {
		if !v.CanInterface() {
			return values, fmt.Errorf("could not interface value for: %s", k)
		}
		values[k] = v.Interface()
	}
	return
}

// QueryGetValues returns the configuration values `urls` for `id`
func (a *ModuleAPIClient) QueryGetValues(id string, urls []string) (values map[string]interface{}, err error) {
	return a.queryGetValuesByKind(false, id, urls)
}

// QueryGetValue returns the configuration value `url` for `id`
func (a *ModuleAPIClient) QueryGetValue(id string, url string) (value interface{}, err error) {
	vs, err := a.queryGetValuesByKind(false, id, []string{url})
	if err != nil {
		return nil, err
	}
	return vs[url], nil
}

// QueryGetValuesDsc returns the discoverable values `urls` for `id`
func (a *ModuleAPIClient) QueryGetValuesDsc(id string, urls []string) (values map[string]interface{}, err error) {
	return a.queryGetValuesByKind(true, id, urls)
}

// QueryGetValueDsc returns the configuration value `url` for `id`
func (a *ModuleAPIClient) QueryGetValueDsc(id string, url string) (value interface{}, err error) {
	vs, err := a.queryGetValuesByKind(true, id, []string{url})
	if err != nil {
		return nil, err
	}
	return vs[url], nil
}

// QueryDelete deletes a node by id
func (a *ModuleAPIClient) QueryDelete(id string) (r types.Node, e error) {
	q := &pb.Query{URL: id}
	rv, e := a.oneshot("QueryDelete", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *ModuleAPIClient) QueryReadAll() (r []types.Node, e error) {
	q := &ptypes.Empty{}
	rvs, e := a.oneshot("QueryReadAll", reflect.ValueOf(q))
	if e != nil {
		return
	}
	mquery := rvs.Interface().(*pb.QueryMulti)
	for _, q := range mquery.Queries {
		r = append(r, NewNodeFromMessage(q.GetNode()))
	}
	return
}

func (a *ModuleAPIClient) QueryReadAllDsc() (r []types.Node, e error) {
	q := &ptypes.Empty{}
	rvs, e := a.oneshot("QueryReadAllDsc", reflect.ValueOf(q))
	if e != nil {
		return
	}
	mquery := rvs.Interface().(*pb.QueryMulti)
	for _, q := range mquery.Queries {
		r = append(r, NewNodeFromMessage(q.GetNode()))
	}
	return
}

func (a *ModuleAPIClient) QueryMutationNodes() (r pb.MutationNodeList, e error) {
	q := &ptypes.Empty{}
	rv, e := a.oneshot("QueryMutationNodes", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = *rv.Interface().(*pb.Query).GetMutationNodeList()
	return
}

func (a *ModuleAPIClient) QueryMutationEdges() (r pb.MutationEdgeList, e error) {
	q := &ptypes.Empty{}
	rv, e := a.oneshot("QueryMutationEdges", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = *rv.Interface().(*pb.Query).GetMutationEdgeList()
	return
}

func (a *ModuleAPIClient) QueryNodeMutationNodes(id string) (r pb.MutationNodeList, e error) {
	q := &pb.Query{URL: util.NodeURLJoin(id, "/graph/nodes")}
	rv, e := a.oneshot("QueryNodeMutationNodes", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = *rv.Interface().(*pb.Query).GetMutationNodeList()
	return
}

func (a *ModuleAPIClient) QueryNodeMutationEdges(id string) (r pb.MutationEdgeList, e error) {
	q := &pb.Query{URL: util.NodeURLJoin(id, "/graph/edges")}
	rv, e := a.oneshot("QueryNodeMutationEdges", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = *rv.Interface().(*pb.Query).GetMutationEdgeList()
	return
}

func (a *ModuleAPIClient) QueryNodeMutationPath(id string) (r pb.MutationPath, e error) {
	q := &pb.Query{URL: util.NodeURLJoin(id, "/graph/path")}
	rv, e := a.oneshot("QueryNodeMutationPath", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = *rv.Interface().(*pb.Query).GetMutationPath()
	return
}

func (a *ModuleAPIClient) QueryDeleteAll() (r []types.Node, e error) {
	q := &ptypes.Empty{}
	rvs, e := a.oneshot("QueryDeleteAll", reflect.ValueOf(q))
	if e != nil {
		return
	}
	mquery := rvs.Interface().(*pb.QueryMulti)
	for _, q := range mquery.Queries {
		r = append(r, NewNodeFromMessage(q.GetNode()))
	}
	return
}
func (a *ModuleAPIClient) QueryFreeze() (e error) {
	q := &ptypes.Empty{}
	_, e = a.oneshot("QueryFreeze", reflect.ValueOf(q))
	return
}
func (a *ModuleAPIClient) QueryThaw() (e error) {
	q := &ptypes.Empty{}
	_, e = a.oneshot("QueryThaw", reflect.ValueOf(q))
	return
}

func (a *ModuleAPIClient) QueryFrozen() (r bool, e error) {
	q := &ptypes.Empty{}
	rv, e := a.oneshot("QueryFrozen", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = rv.Interface().(*pb.Query).GetBool()
	return
}

func (a *ModuleAPIClient) ServiceInit(id string, module string) (c <-chan types.ServiceControl, e error) {
	var stream grpc.ClientStream
	stream, e = a.serverStream("ServiceInit", reflect.ValueOf(&pb.ServiceInitRequest{Id: id, Module: module}))
	if e != nil {
		return
	}
	// read our init
	init, e := stream.(pb.ModuleAPI_ServiceInitClient).Recv()
	if e != nil || init.Command != pb.ServiceControl_INIT {
		e = fmt.Errorf("%s failed to init, (got %v, err %v)", id, init.Command, e)
		return
	}
	self := &pb.Node{}
	ptypes.UnmarshalAny(init.Config, self)
	a.self = self.GetId()

	cc := make(chan types.ServiceControl)
	go func() {
		for {
			var ctl *pb.ServiceControl
			ctl, e = stream.(pb.ModuleAPI_ServiceInitClient).Recv()
			if e != nil {
				return
			}
			cc <- types.ServiceControl{Command: types.ServiceControl_Command(ctl.Command)}
		}
	}()
	c = cc
	return
}

func (a *ModuleAPIClient) MutationInit(id string, module string) (c <-chan types.Event, e error) {
	var stream grpc.ClientStream
	if stream, e = a.serverStream("MutationInit", reflect.ValueOf(&pb.ServiceInitRequest{Id: id, Module: module})); e != nil {
		return
	}
	cc := make(chan types.Event)
	go func() {
		for {
			var mc *pb.MutationControl
			if mc, e = stream.(pb.ModuleAPI_MutationInitClient).Recv(); e != nil {
				a.Logf(types.LLERROR, "got stream read error on mutation stream: %v\n", e)
				return
			}
			cfg := NewNodeFromMessage(mc.GetCfg())
			dsc := NewNodeFromMessage(mc.GetDsc())
			cc <- NewEvent(
				types.Event_STATE_MUTATION,
				cfg.ID().String(),
				&MutationEvent{
					Type:     mc.GetType(),
					NodeCfg:  cfg,
					NodeDsc:  dsc,
					Mutation: [2]string{mc.GetModule(), mc.GetId()},
				})
		}
	}()
	c = cc
	return
}

func (a *ModuleAPIClient) EventInit(id string, module string) (c <-chan types.Event, e error) {
	var stream grpc.ClientStream
	if stream, e = a.serverStream("EventInit", reflect.ValueOf(&pb.ServiceInitRequest{Id: id, Module: module})); e != nil {
		return
	}
	cc := make(chan types.Event)
	go func() {
		for {
			var ec *pb.EventControl
			if ec, e = stream.(pb.ModuleAPI_EventInitClient).Recv(); e != nil {
				a.Logf(types.LLERROR, "got stream read error on event stream: %v\n", e)
				return
			}
			switch ec.GetType() {
			case pb.EventControl_Mutation:
				event := ec.GetMutationControl()
				cfg := NewNodeFromMessage(event.GetCfg())
				dsc := NewNodeFromMessage(event.GetDsc())
				cc <- NewEvent(
					types.Event_STATE_MUTATION,
					cfg.ID().String(),
					&MutationEvent{
						Type:     event.GetType(),
						NodeCfg:  cfg,
						NodeDsc:  dsc,
						Mutation: [2]string{event.GetModule(), event.GetId()},
					})
			case pb.EventControl_StateChange:
				event := ec.GetStateChangeControl()
				cc <- NewEvent(
					types.Event_STATE_CHANGE,
					event.GetUrl(),
					&StateChangeEvent{
						Type:  event.GetType(),
						URL:   event.GetUrl(),
						Value: reflect.ValueOf(event.GetValue()),
					})
			case pb.EventControl_Discovery:
				event := ec.GetDiscoveryEvent()
				cc <- NewEvent(
					types.Event_DISCOVERY,
					event.GetUrl(),
					&DiscoveryEvent{
						ID:      event.GetId(),
						URL:     event.GetUrl(),
						ValueID: event.GetValueId(),
					})
			}
		}
	}()
	c = cc
	return
}

func (a *ModuleAPIClient) DiscoveryInit(id string) (c chan<- types.Event, e error) {
	var stream pb.ModuleAPI_DiscoveryInitClient
	var conn *grpc.ClientConn
	if conn, e = grpc.Dial(a.sock, grpc.WithInsecure()); e != nil {
		return
	}
	client := pb.NewModuleAPIClient(conn)
	if stream, e = client.DiscoveryInit(context.Background()); e != nil {
		return
	}
	cc := make(chan types.Event)
	go func() {
		for {
			v := <-cc
			de, ok := v.Data().(*DiscoveryEvent)
			if !ok {
				a.Logf(ERROR, "got event that is not *DiscoveryEvent: %v", v.Data())
				continue
			}
			d := &pb.DiscoveryEvent{
				Id:      id,
				Url:     de.URL,
				ValueId: de.ValueID,
			}
			if e = stream.Send(d); e != nil {
				a.Logf(CRITICAL, "got stream send error on discovery stream: %v\n", e)
				return
			}
		}
	}()
	c = cc
	return
}

func (a *ModuleAPIClient) LoggerInit(si string) (e error) {
	var stream pb.ModuleAPI_LoggerInitClient
	var conn *grpc.ClientConn
	if conn, e = grpc.Dial(a.sock, grpc.WithInsecure()); e != nil {
		return
	}
	client := pb.NewModuleAPIClient(conn)
	if stream, e = client.LoggerInit(context.Background()); e != nil {
		return
	}
	a.logChan = make(chan LoggerEvent)
	a.log = &ServiceLogger{}
	a.log.SetLoggerLevel(types.LLDDDEBUG)
	a.log.SetModule(si)
	a.log.(*ServiceLogger).RegisterChannel(a.logChan)
	go func() {
		for {
			l := <-a.logChan
			msg := &pb.LogMessage{
				Origin: l.Module,
				Level:  uint32(l.Level),
				Msg:    l.Message,
			}
			if e = stream.Send(msg); e != nil {
				fmt.Printf("got stream send error on logger stream: %v\n", e)
				return
			}
		}
	}()
	return
}

// use reflection to call API methods by name and encapsulate
// all of the one-time connection symantics
// this is convoluted, but makes everything else DRYer
func (a *ModuleAPIClient) oneshot(call string, in reflect.Value) (out reflect.Value, e error) {
	var conn *grpc.ClientConn
	if conn, e = grpc.Dial(a.sock, grpc.WithInsecure()); e != nil {
		return
	}
	defer conn.Close()
	c := pb.NewModuleAPIClient(conn)
	fv := reflect.ValueOf(c).MethodByName(call)
	if !fv.IsValid() {
		e = fmt.Errorf("no such API call: %s", call)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := fv.Call([]reflect.Value{reflect.ValueOf(ctx), in})
	if len(r) != 2 {
		// ?!
		e = fmt.Errorf("bad API call result: %s", call)
		return
	}
	out = r[0]
	if !r[1].IsNil() {
		e = r[1].Interface().(error)
	}
	return
}

func (a *ModuleAPIClient) serverStream(call string, in reflect.Value) (out grpc.ClientStream, e error) {
	var conn *grpc.ClientConn
	conn, e = grpc.Dial(a.sock, grpc.WithInsecure())
	if e != nil {
		return
	}
	//defer conn.Close()
	c := pb.NewModuleAPIClient(conn)
	fv := reflect.ValueOf(c).MethodByName(call)
	if fv.IsNil() {
		e = fmt.Errorf("no such API call: %s", call)
		return
	}
	r := fv.Call([]reflect.Value{reflect.ValueOf(context.Background()), in})
	if len(r) != 2 {
		// ?!
		e = fmt.Errorf("bad API call result: %s", call)
		return
	}
	if r[1].Interface() != nil {
		e = r[1].Interface().(error)
		return
	}
	out = r[0].Interface().(grpc.ClientStream)
	return
}

////////////////////////////
// Passthrough Interfaces /
//////////////////////////

/*
 * Consume Logger
 */
var _ types.Logger = (*ModuleAPIClient)(nil)

func (a *ModuleAPIClient) Log(level types.LoggerLevel, m string) { a.log.Log(level, m) }
func (a *ModuleAPIClient) Logf(level types.LoggerLevel, fmt string, v ...interface{}) {
	a.log.Logf(level, fmt, v...)
}
func (a *ModuleAPIClient) SetModule(name string)                  { a.log.SetModule(name) }
func (a *ModuleAPIClient) GetModule() string                      { return a.log.GetModule() }
func (a *ModuleAPIClient) SetLoggerLevel(level types.LoggerLevel) { a.log.SetLoggerLevel(level) }
func (a *ModuleAPIClient) GetLoggerLevel() types.LoggerLevel      { return a.log.GetLoggerLevel() }
func (a *ModuleAPIClient) IsEnabledFor(level types.LoggerLevel) bool {
	return a.log.IsEnabledFor(level)
}
