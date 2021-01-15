/* ModuleAPIServer.go: provides the RPC API.  All gRPC calls live here
 * (except PhoneHome, which is a special exception in StateSyncEngine.go)
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I proto/src --go_out=plugins=grpc:proto proto/src/ModuleAPI.proto

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

// A DiscoveryEvent announce a discovery
// This should probably live elsewhere
// This maps directly to a pb.DiscoveryControl
type DiscoveryEvent struct {
	ID      string // ID of a service instance
	URL     string // fully qualified, with node
	ValueID string
}

func (de *DiscoveryEvent) String() string {
	return fmt.Sprintf("(%s) %s == %s", de.ID, de.URL, de.ValueID)
}

//////////////////////
// ModuleAPIServer Object /
////////////////////

var _ pb.ModuleAPIServer = (*ModuleAPIServer)(nil)

// ModuleAPIServer is the gateway for gRPC calls into Kraken (i.e. the Module interface)
type ModuleAPIServer struct {
	nlist net.Listener
	ulist net.Listener
	query *QueryEngine
	log   lib.Logger
	em    lib.EventEmitter
	sm    lib.ServiceManager
	schan chan<- lib.EventListener
	self  lib.NodeID
}

// NewModuleAPIServer creates a new, initialized API
func NewModuleAPIServer(ctx Context) *ModuleAPIServer {
	api := &ModuleAPIServer{
		nlist: ctx.RPC.NetListner,
		ulist: ctx.RPC.UNIXListener,
		query: &ctx.Query,
		log:   &ctx.Logger,
		em:    NewEventEmitter(lib.Event_API),
		schan: ctx.SubChan,
		self:  ctx.Self,
		sm:    ctx.Sm,
	}
	api.log.SetModule("API")
	return api
}

func (s *ModuleAPIServer) QueryCreate(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
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

func (s *ModuleAPIServer) QueryRead(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var nout lib.Node
	out = &pb.Query{}
	nout, e = s.query.Read(NewNodeIDFromURL(in.URL))
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *ModuleAPIServer) QueryReadDsc(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var nout lib.Node
	out = &pb.Query{}
	nout, e = s.query.ReadDsc(NewNodeIDFromURL(in.URL))
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *ModuleAPIServer) QueryUpdate(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
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

func (s *ModuleAPIServer) QueryUpdateDsc(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
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

func (s *ModuleAPIServer) QueryDelete(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var nout lib.Node
	out = &pb.Query{}
	nout, e = s.query.Delete(NewNodeIDFromURL(in.URL))
	out.URL = in.URL
	if nout != nil {
		out.Payload = &pb.Query_Node{Node: nout.Message().(*pb.Node)}
	}
	return
}

func (s *ModuleAPIServer) QueryReadAll(ctx context.Context, in *empty.Empty) (out *pb.QueryMulti, e error) {
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

func (s *ModuleAPIServer) QueryReadAllDsc(ctx context.Context, in *empty.Empty) (out *pb.QueryMulti, e error) {
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

func (s *ModuleAPIServer) QueryMutationNodes(ctx context.Context, in *empty.Empty) (out *pb.Query, e error) {
	var mnlout pb.MutationNodeList
	url := "/graph/nodes"
	out = &pb.Query{}
	mnlout, e = s.query.ReadMutationNodes(url)
	out.URL = url
	if mnlout.MutationNodeList != nil {
		out.Payload = &pb.Query_MutationNodeList{
			MutationNodeList: &mnlout,
		}
	}
	return
}

func (s *ModuleAPIServer) QueryMutationEdges(ctx context.Context, in *empty.Empty) (out *pb.Query, e error) {
	var melout pb.MutationEdgeList
	url := "/graph/nodes"
	out = &pb.Query{}
	melout, e = s.query.ReadMutationEdges(url)
	out.URL = "/graph/nodes"
	if melout.MutationEdgeList != nil {
		out.Payload = &pb.Query_MutationEdgeList{
			MutationEdgeList: &melout,
		}
	}
	return
}

func (s *ModuleAPIServer) QueryNodeMutationNodes(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var mnlout pb.MutationNodeList
	out = &pb.Query{}
	mnlout, e = s.query.ReadNodeMutationNodes(in.URL)
	out.URL = in.URL
	if mnlout.MutationNodeList != nil {
		out.Payload = &pb.Query_MutationNodeList{
			MutationNodeList: &mnlout,
		}
	}
	return
}

func (s *ModuleAPIServer) QueryNodeMutationEdges(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var melout pb.MutationEdgeList
	out = &pb.Query{}
	melout, e = s.query.ReadNodeMutationEdges(in.URL)
	out.URL = in.URL
	if melout.MutationEdgeList != nil {
		out.Payload = &pb.Query_MutationEdgeList{
			MutationEdgeList: &melout,
		}
	}
	return
}

func (s *ModuleAPIServer) QueryNodeMutationPath(ctx context.Context, in *pb.Query) (out *pb.Query, e error) {
	var mpout pb.MutationPath
	out = &pb.Query{}
	mpout, e = s.query.ReadNodeMutationPath(in.URL)
	out.URL = in.URL
	if mpout.Chain != nil {
		out.Payload = &pb.Query_MutationPath{
			MutationPath: &mpout,
		}
	}
	return
}

func (s *ModuleAPIServer) QueryDeleteAll(ctx context.Context, in *empty.Empty) (out *pb.QueryMulti, e error) {
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

func (s *ModuleAPIServer) QueryFreeze(ctx context.Context, in *empty.Empty) (out *pb.Query, e error) {
	e = s.query.Freeze()
	out = &pb.Query{}
	return
}
func (s *ModuleAPIServer) QueryThaw(ctx context.Context, in *empty.Empty) (out *pb.Query, e error) {
	e = s.query.Thaw()
	out = &pb.Query{}
	return
}
func (s *ModuleAPIServer) QueryFrozen(ctx context.Context, in *empty.Empty) (out *pb.Query, e error) {
	out = &pb.Query{}
	rb, e := s.query.Frozen()
	out.Payload = &pb.Query_Bool{Bool: rb}
	return
}

/*
 * Service management
 */

func (s *ModuleAPIServer) ServiceInit(sir *pb.ServiceInitRequest, stream pb.ModuleAPI_ServiceInitServer) (e error) {
	srv := s.sm.GetService(sir.GetId())

	self, _ := s.query.Read(s.self)
	any, _ := ptypes.MarshalAny(self.Message())
	stream.Send(&pb.ServiceControl{
		Command: pb.ServiceControl_INIT,
		Config:  any,
	})
	c := make(chan lib.ServiceControl)
	srv.SetCtl(c)
	for {
		ctl := <-c
		stream.Send(&pb.ServiceControl{
			Command: pb.ServiceControl_Command(ctl.Command),
		})
	}
}

/*
 * Mutation management
 */

// MutationInit handles establishing the mutation stream
// This just caputures (filtered) mutation events and sends them over the stream
func (s *ModuleAPIServer) MutationInit(sir *pb.ServiceInitRequest, stream pb.ModuleAPI_MutationInitServer) (e error) {
	sid := sir.GetId()
	echan := make(chan lib.Event)
	list := NewEventListener("MutationFor:"+sid, lib.Event_STATE_MUTATION,
		func(e lib.Event) bool {
			d := e.Data().(*MutationEvent)
			if d.Mutation[0] == sid {
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

// EventInit handles establishing the event stream
// This just caputures all events and sends them over the stream
func (s *ModuleAPIServer) EventInit(sir *pb.ServiceInitRequest, stream pb.ModuleAPI_EventInitServer) (e error) {
	module := sir.GetModule()
	echan := make(chan lib.Event)
	filterFunction := func(e lib.Event) bool {
		return true
	}
	list := NewEventListener("EventFor:"+module, lib.Event_ALL,
		filterFunction,
		func(v lib.Event) error { return ChanSender(v, echan) })
	// subscribe our listener
	s.schan <- list

	for {
		v := <-echan
		var ec = &pb.EventControl{}
		switch v.Type() {
		case lib.Event_STATE_MUTATION:
			smev := v.Data().(*MutationEvent)
			ec = &pb.EventControl{
				Type: pb.EventControl_Mutation,
				Event: &pb.EventControl_MutationControl{
					MutationControl: &pb.MutationControl{
						Module: smev.Mutation[0],
						Id:     smev.Mutation[1],
						Type:   smev.Type,
						Cfg:    smev.NodeCfg.Message().(*pb.Node),
						Dsc:    smev.NodeDsc.Message().(*pb.Node),
					},
				},
			}
		case lib.Event_STATE_CHANGE:
			scev := v.Data().(*StateChangeEvent)
			s.Logf(lib.LLDEBUG, "api server got state change event: %+v\n%v", scev, scev.Value)
			ec = &pb.EventControl{
				Type: pb.EventControl_StateChange,
				Event: &pb.EventControl_StateChangeControl{
					StateChangeControl: &pb.StateChangeControl{
						Type:  scev.Type,
						Url:   scev.URL,
						Value: lib.ValueToString(scev.Value),
					},
				},
			}
		case lib.Event_DISCOVERY:
			dev := v.Data().(*DiscoveryEvent)
			ec = &pb.EventControl{
				Type: pb.EventControl_Discovery,
				Event: &pb.EventControl_DiscoveryEvent{
					DiscoveryEvent: &pb.DiscoveryEvent{
						Id:      dev.ID,
						Url:     dev.URL,
						ValueId: dev.ValueID,
					},
				},
			}
		default:
			s.Logf(lib.LLERROR, "Couldn't convert Event into mutation, statechange, or discovery: %+v", v)
		}
		if e := stream.Send(ec); e != nil {
			s.Logf(INFO, "event stream closed: %v", e)
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
func (s *ModuleAPIServer) DiscoveryInit(stream pb.ModuleAPI_DiscoveryInitServer) (e error) {
	for {
		dc, e := stream.Recv()
		if e != nil {
			s.Logf(INFO, "discovery stream closed: %v", e)
			break
		}
		dv := &DiscoveryEvent{
			ID:      dc.GetId(),
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
func (s *ModuleAPIServer) LoggerInit(stream pb.ModuleAPI_LoggerInitServer) (e error) {
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
func (s *ModuleAPIServer) Run(ready chan<- interface{}) {
	s.Log(INFO, "starting API")
	srv := grpc.NewServer()
	pb.RegisterModuleAPIServer(srv, s)
	reflection.Register(srv)
	ready <- nil
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
var _ lib.Logger = (*ModuleAPIServer)(nil)

func (s *ModuleAPIServer) Log(level lib.LoggerLevel, m string) { s.log.Log(level, m) }
func (s *ModuleAPIServer) Logf(level lib.LoggerLevel, fmt string, v ...interface{}) {
	s.log.Logf(level, fmt, v...)
}
func (s *ModuleAPIServer) SetModule(name string)                { s.log.SetModule(name) }
func (s *ModuleAPIServer) GetModule() string                    { return s.log.GetModule() }
func (s *ModuleAPIServer) SetLoggerLevel(level lib.LoggerLevel) { s.log.SetLoggerLevel(level) }
func (s *ModuleAPIServer) GetLoggerLevel() lib.LoggerLevel      { return s.log.GetLoggerLevel() }
func (s *ModuleAPIServer) IsEnabledFor(level lib.LoggerLevel) bool {
	return s.log.IsEnabledFor(level)
}

/*
 * Consume an emitter, so we implement EventEmitter directly
 */
var _ lib.EventEmitter = (*ModuleAPIServer)(nil)

func (s *ModuleAPIServer) Subscribe(id string, c chan<- []lib.Event) error {
	return s.em.Subscribe(id, c)
}
func (s *ModuleAPIServer) Unsubscribe(id string) error { return s.em.Unsubscribe(id) }
func (s *ModuleAPIServer) Emit(v []lib.Event)          { s.em.Emit(v) }
func (s *ModuleAPIServer) EmitOne(v lib.Event)         { s.em.EmitOne(v) }
func (s *ModuleAPIServer) EventType() lib.EventType    { return s.em.EventType() }
