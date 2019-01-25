/* APIClient.go: provides wrappers to make the API easier to use for Go modules.
 *               note: you can use the API without this; it's just a helper.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/golang/protobuf/ptypes"

	"github.com/golang/protobuf/ptypes/empty"
	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
	"google.golang.org/grpc"
)

var _ lib.APIClient = (*APIClient)(nil)

type APIClient struct {
	sock    string
	self    lib.NodeID
	logChan chan LoggerEvent
	log     lib.Logger
}

func NewAPIClient(sock string) *APIClient {
	a := &APIClient{
		sock: sock,
	}
	return a
}

func (a *APIClient) Self() lib.NodeID { return a.self }

func (a *APIClient) SetSelf(s lib.NodeID) { a.self = s }

func (a *APIClient) QueryCreate(n lib.Node) (r lib.Node, e error) {
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

func (a *APIClient) QueryRead(id string) (r lib.Node, e error) {
	q := &pb.Query{URL: id}
	rv, e := a.oneshot("QueryRead", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *APIClient) QueryReadDot(n lib.Node) (r string, e error) {
	q := &pb.Query{
		Payload: &pb.Query_Node{
			Node: n.Message().(*pb.Node),
		},
	}
	a.Logf(lib.LLDEBUG, "Made it to apiclient!")
	rv, e := a.oneshot("QueryReadDot", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = rv.Interface().(*pb.Query).GetText()
	return
}

func (a *APIClient) QueryReadDsc(id string) (r lib.Node, e error) {
	q := &pb.Query{URL: id}
	rv, e := a.oneshot("QueryReadDsc", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *APIClient) QueryUpdate(n lib.Node) (r lib.Node, e error) {
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

func (a *APIClient) QueryUpdateDsc(n lib.Node) (r lib.Node, e error) {
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

func (a *APIClient) QueryDelete(id string) (r lib.Node, e error) {
	q := &pb.Query{URL: id}
	rv, e := a.oneshot("QueryDelete", reflect.ValueOf(q))
	if e != nil {
		return
	}
	r = NewNodeFromMessage(rv.Interface().(*pb.Query).GetNode())
	return
}

func (a *APIClient) QueryReadAll() (r []lib.Node, e error) {
	q := &empty.Empty{}
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

func (a *APIClient) QueryReadAllDsc() (r []lib.Node, e error) {
	q := &empty.Empty{}
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

func (a *APIClient) QueryDeleteAll() (r []lib.Node, e error) {
	q := &empty.Empty{}
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

func (a *APIClient) ServiceInit(id string, module string) (c <-chan lib.ServiceControl, e error) {
	var stream grpc.ClientStream
	stream, e = a.serverStream("ServiceInit", reflect.ValueOf(&pb.ServiceInitRequest{Id: id, Module: module}))
	if e != nil {
		return
	}
	// read our init
	init, e := stream.(pb.API_ServiceInitClient).Recv()
	if e != nil || init.Command != pb.ServiceControl_INIT {
		e = fmt.Errorf("%s failed to init, (got %v, err %v)", id, init.Command, e)
		return
	}
	self := &pb.Node{}
	ptypes.UnmarshalAny(init.Config, self)
	a.self = NewNodeIDFromBinary(self.GetId())

	cc := make(chan lib.ServiceControl)
	go func() {
		for {
			var ctl *pb.ServiceControl
			ctl, e = stream.(pb.API_ServiceInitClient).Recv()
			if e != nil {
				return
			}
			cc <- lib.ServiceControl{Command: lib.ServiceControl_Command(ctl.Command), Config: ctl.Config}
		}
	}()
	c = cc
	return
}

func (a *APIClient) MutationInit(id string, module string) (c <-chan lib.Event, e error) {
	var stream grpc.ClientStream
	if stream, e = a.serverStream("MutationInit", reflect.ValueOf(&pb.ServiceInitRequest{Id: id, Module: module})); e != nil {
		return
	}
	cc := make(chan lib.Event)
	go func() {
		for {
			var mc *pb.MutationControl
			if mc, e = stream.(pb.API_MutationInitClient).Recv(); e != nil {
				fmt.Printf("got stream read error on mutation stream: %v\n", e)
				return
			}
			cfg := NewNodeFromMessage(mc.GetCfg())
			dsc := NewNodeFromMessage(mc.GetDsc())
			cc <- NewEvent(
				lib.Event_STATE_MUTATION,
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

func (a *APIClient) DiscoveryInit() (c chan<- lib.Event, e error) {
	var stream pb.API_DiscoveryInitClient
	var conn *grpc.ClientConn
	if conn, e = grpc.Dial(a.sock, grpc.WithInsecure()); e != nil {
		return
	}
	client := pb.NewAPIClient(conn)
	if stream, e = client.DiscoveryInit(context.Background()); e != nil {
		return
	}
	cc := make(chan lib.Event)
	go func() {
		for {
			v := <-cc
			de, ok := v.Data().(*DiscoveryEvent)
			if !ok {
				a.Logf(ERROR, "got event that is not *DiscoveryEvent: %v", v.Data())
				continue
			}
			d := &pb.DiscoveryEvent{
				Module:  de.Module,
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

func (a *APIClient) LoggerInit(si string) (e error) {
	var stream pb.API_LoggerInitClient
	var conn *grpc.ClientConn
	if conn, e = grpc.Dial(a.sock, grpc.WithInsecure()); e != nil {
		return
	}
	client := pb.NewAPIClient(conn)
	if stream, e = client.LoggerInit(context.Background()); e != nil {
		return
	}
	a.logChan = make(chan LoggerEvent)
	a.log = &ServiceLogger{}
	a.log.SetLoggerLevel(lib.LLDDDEBUG)
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
func (a *APIClient) oneshot(call string, in reflect.Value) (out reflect.Value, e error) {
	var conn *grpc.ClientConn
	if conn, e = grpc.Dial(a.sock, grpc.WithInsecure()); e != nil {
		return
	}
	defer conn.Close()
	c := pb.NewAPIClient(conn)
	fv := reflect.ValueOf(c).MethodByName(call)
	if fv.IsNil() {
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

func (a *APIClient) serverStream(call string, in reflect.Value) (out grpc.ClientStream, e error) {
	var conn *grpc.ClientConn
	conn, e = grpc.Dial(a.sock, grpc.WithInsecure())
	if e != nil {
		return
	}
	//defer conn.Close()
	c := pb.NewAPIClient(conn)
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
var _ lib.Logger = (*APIClient)(nil)

func (a *APIClient) Log(level lib.LoggerLevel, m string) { a.log.Log(level, m) }
func (a *APIClient) Logf(level lib.LoggerLevel, fmt string, v ...interface{}) {
	a.log.Logf(level, fmt, v...)
}
func (a *APIClient) SetModule(name string)                { a.log.SetModule(name) }
func (a *APIClient) GetModule() string                    { return a.log.GetModule() }
func (a *APIClient) SetLoggerLevel(level lib.LoggerLevel) { a.log.SetLoggerLevel(level) }
func (a *APIClient) GetLoggerLevel() lib.LoggerLevel      { return a.log.GetLoggerLevel() }
func (a *APIClient) IsEnabledFor(level lib.LoggerLevel) bool {
	return a.log.IsEnabledFor(level)
}
