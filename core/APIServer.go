/* APIServer.go: provides the RPC API.  All gRPC calls live here
 * (except PhoneHome, which is a special exception in StateSyncEngine.go)
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I proto -I proto/include --go_out=plugins=grpc:proto proto/API.proto

package core

import (
	"context"
	"fmt"
	"net"

	"github.com/golang/protobuf/ptypes"

	"github.com/golang/protobuf/ptypes/empty"
	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

///////////////////////
// Auxiliary objects /
/////////////////////

// DiscoveryEvents announce a discovery
// This should probably live elsewhere
// This maps directly to a pb.DiscoveryControl
type DiscoveryEvent struct {
	Module  string
	URL     string // fully qualified, with node
	ValueID string
}

func (de *DiscoveryEvent) String() string {
	return fmt.Sprintf("(%s) %s == %s", de.Module, de.URL, de.ValueID)
}

//////////////////////
// APIServer Object /
////////////////////

var _ pb.APIServer = (*APIServer)(nil)

// APIServer is the gateway for gRPC calls into Kraken (i.e. the Module interface)
type APIServer struct {
	nlist net.Listener
	ulist net.Listener
	query *QueryEngine
	log   lib.Logger
	em    lib.EventEmitter
	sm    lib.ServiceManager
	schan chan<- lib.EventListener
	self  lib.NodeID
}

// NewAPIServer creates a new, initialized API
func NewAPIServer(ctx Context) *APIServer {
	api := &APIServer{
		nlist: ctx.RPC.NetListner,
		ulist: ctx.RPC.UNIXListener,
		query: &ctx.Query,
		log:   &ctx.Logger,
		em:    NewEventEmitter(lib.Event_API),
		sm:    ctx.Services,
		schan: ctx.SubChan,
		self:  ctx.Self,
	}
	api.log.SetModule("API")
	return api
}

func (s *APIServer) QueryCreate(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	pbin := in.GetNode()
	out = &pb.Query{}
	if pbin == nil {
		e = fmt.Errorf("create query must contain a valid node")
		return
	}
	nin := NewNodeFromMessage(pbin)
	var nout lib.Node
	nout, e = s.query.Create(nin)
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *APIServer) QueryRead(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var nout lib.Node
	out = &pb.Query{}
	nout, e = s.query.Read(NewNodeIDFromURL(in.URL))
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *APIServer) QueryReadDot(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	pbin := in.GetNode()
	out = &pb.Query{}
	if pbin == nil {
		e = fmt.Errorf("create query must contain a valid node")
		return
	}
	nin := NewNodeFromMessage(pbin)
	var sout string
	sout, e = s.query.ReadDot(nin)
	out.URL = in.URL
	if sout != "" {
		out.Payload = &pb.Query_Text{Text: sout}
	}
	return
}

func (s *APIServer) QueryReadDsc(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var nout lib.Node
	out = &pb.Query{}
	nout, e = s.query.ReadDsc(NewNodeIDFromURL(in.URL))
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *APIServer) QueryUpdate(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	pbin := in.GetNode()
	out = &pb.Query{}
	if pbin == nil {
		e = fmt.Errorf("update query must contain a valid node")
		return
	}
	nin := NewNodeFromMessage(pbin)
	var nout lib.Node
	nout, e = s.query.Update(nin)
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *APIServer) QueryUpdateDsc(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	pbin := in.GetNode()
	out = &pb.Query{}
	if pbin == nil {
		e = fmt.Errorf("update query must contain a valid node")
		return
	}
	nin := NewNodeFromMessage(pbin)
	var nout lib.Node
	nout, e = s.query.UpdateDsc(nin)
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *APIServer) QueryDelete(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var nout lib.Node
	out = &pb.Query{}
	nout, e = s.query.Delete(NewNodeIDFromURL(in.URL))
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *APIServer) QueryReadAll(ctx context.Context, in *empty.Empty) (out *pb.QueryMulti, e error) {
	var nout []lib.Node
	out = &pb.QueryMulti{}
	out.Queries = []*pb.Query{}
	nout, e = s.query.ReadAll()
	for _, n := range nout {
		q := &pb.Query{
			URL: n.ID().String(),
			Payload: &pb.Query_Node{
				Node: n.Message().(*pb.Node),
			},
		}
		out.Queries = append(out.Queries, q)
	}
	return
}

func (s *APIServer) QueryReadAllDsc(ctx context.Context, in *empty.Empty) (out *pb.QueryMulti, e error) {
	var nout []lib.Node
	out = &pb.QueryMulti{}
	out.Queries = []*pb.Query{}
	nout, e = s.query.ReadAllDsc()
	for _, n := range nout {
		q := &pb.Query{
			URL: n.ID().String(),
			Payload: &pb.Query_Node{
				Node: n.Message().(*pb.Node),
			},
		}
		out.Queries = append(out.Queries, q)
	}
	return
}

func (s *APIServer) QueryDeleteAll(ctx context.Context, in *empty.Empty) (out *pb.QueryMulti, e error) {
	var nout []lib.Node
	out = &pb.QueryMulti{}
	out.Queries = []*pb.Query{}
	nout, e = s.query.DeleteAll()
	for _, n := range nout {
		q := &pb.Query{
			URL: n.ID().String(),
			Payload: &pb.Query_Node{
				Node: n.Message().(*pb.Node),
			},
		}
		out.Queries = append(out.Queries, q)
	}
	return
}

/*
 * Service management
 */

func (s *APIServer) ServiceInit(sir *pb.ServiceInitRequest, stream pb.API_ServiceInitServer) (e error) {
	srv := s.sm.Service(sir.GetId())

	self, _ := s.query.Read(s.self)
	any, _ := ptypes.MarshalAny(self.Message())
	stream.Send(&pb.ServiceControl{
		Command: pb.ServiceControl_INIT,
		Config:  any,
	})
	if srv.Config() != nil {
		e = stream.Send(&pb.ServiceControl{Command: pb.ServiceControl_UPDATE, Config: srv.Config()})
		if e != nil {
			s.Logf(ERROR, "send error: %v", e)
		}
	}
	c := make(chan lib.ServiceControl)
	srv.SetCtl(c)
	for {
		ctl := <-c
		stream.Send(&pb.ServiceControl{
			Command: pb.ServiceControl_Command(ctl.Command),
			Config:  ctl.Config,
		})
	}
	return
}

/*
 * Mutation management
 */

// MutationInit handles establishing the mutation stream
// This just caputures (filtered) mutation events and sends them over the stream
func (s *APIServer) MutationInit(sir *pb.ServiceInitRequest, stream pb.API_MutationInitServer) (e error) {
	module := sir.GetModule()
	echan := make(chan lib.Event)
	list := NewEventListener("MutationFor:"+module, lib.Event_STATE_MUTATION,
		func(e lib.Event) bool {
			d := e.Data().(*MutationEvent)
			if d.Mutation[0] == module {
				return true
			}
			return false
		},
		func(v lib.Event) error { return ChanSender(v, echan) })
	// subscribe our listener
	s.schan <- list

	for {
		v := <-echan
		smev := v.Data().(*MutationEvent)
		mc := &pb.MutationControl{
			Module: smev.Mutation[0],
			Id:     smev.Mutation[1],
			Type:   smev.Type,
			Cfg:    smev.NodeCfg.Message().(*pb.Node),
			Dsc:    smev.NodeDsc.Message().(*pb.Node),
		}
		if e := stream.Send(mc); e != nil {
			s.Logf(INFO, "mutation stream closed: %v", e)
			break
		}
	}

	// politely unsubscribe
	list.SetState(lib.EventListener_UNSUBSCRIBE)
	s.schan <- list
	return
}

// DiscoveryInit handles discoveries from nodes
// This dispatches nodes
func (s *APIServer) DiscoveryInit(stream pb.API_DiscoveryInitServer) (e error) {
	for {
		dc, e := stream.Recv()
		if e != nil {
			s.Logf(INFO, "discovery stream closed: %v", e)
			break
		}
		dv := &DiscoveryEvent{
			Module:  dc.GetModule(),
			URL:     dc.GetUrl(),
			ValueID: dc.GetValueId(),
		}
		v := NewEvent(
			lib.Event_DISCOVERY,
			dc.GetUrl(),
			dv)
		s.EmitOne(v)
	}
	return
}

// LoggerInit initializes and RPC logger stream
func (s *APIServer) LoggerInit(stream pb.API_LoggerInitServer) (e error) {
	for {
		msg, e := stream.Recv()
		if e != nil {
			s.Logf(INFO, "logger stream closted: %v", e)
			break
		}
		s.Logf(lib.LoggerLevel(msg.Level), "%s:%s", msg.Origin, msg.Msg)
	}
	return
}

// Run starts the API service listener
func (s *APIServer) Run() {
	s.Log(INFO, "starting API")
	srv := grpc.NewServer()
	pb.RegisterAPIServer(srv, s)
	reflection.Register(srv)
	if e := srv.Serve(s.ulist); e != nil {
		s.Logf(CRITICAL, "couldn't start API service: %v", e)
		return
	}
}

////////////////////////////
// Passthrough Interfaces /
//////////////////////////

/*
 * Consume Logger
 */
var _ lib.Logger = (*APIServer)(nil)

func (s *APIServer) Log(level lib.LoggerLevel, m string) { s.log.Log(level, m) }
func (s *APIServer) Logf(level lib.LoggerLevel, fmt string, v ...interface{}) {
	s.log.Logf(level, fmt, v...)
}
func (s *APIServer) SetModule(name string)                { s.log.SetModule(name) }
func (s *APIServer) GetModule() string                    { return s.log.GetModule() }
func (s *APIServer) SetLoggerLevel(level lib.LoggerLevel) { s.log.SetLoggerLevel(level) }
func (s *APIServer) GetLoggerLevel() lib.LoggerLevel      { return s.log.GetLoggerLevel() }
func (s *APIServer) IsEnabledFor(level lib.LoggerLevel) bool {
	return s.log.IsEnabledFor(level)
}

/*
 * Consume an emitter, so we implement EventEmitter directly
 */
var _ lib.EventEmitter = (*APIServer)(nil)

func (s *APIServer) Subscribe(id string, c chan<- []lib.Event) error {
	return s.em.Subscribe(id, c)
}
func (s *APIServer) Unsubscribe(id string) error { return s.em.Unsubscribe(id) }
func (s *APIServer) Emit(v []lib.Event)          { s.em.Emit(v) }
func (s *APIServer) EmitOne(v lib.Event)         { s.em.EmitOne(v) }
func (s *APIServer) EventType() lib.EventType    { return s.em.EventType() }
