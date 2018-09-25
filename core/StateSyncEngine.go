/* StateSyncEngine.go: implements the binary state synchronization protocol
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I proto/include -I proto --go_out=plugins=grpc:proto proto/StateSyncMessage.proto

package core

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"time"

	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

/*
 * Note: we choose not to use gRPC for this because we don't want persistent connections.
 * Instead, we send HELLO-style packets consisting of serialized ProtoBufs over raw UDP.
 */

///////////////////////
// Auxiliary Objects /
/////////////////////

type recvPacket struct {
	From lib.NodeID
	Node lib.Node
}

/*
 * stateSyncNeighbor: keeps track of timers on a neighbor
 */

type stateSyncNeighbor struct {
	parent bool // sync up? (or down)
	key    []byte
	id     lib.NodeID
	// this allows us to have this configurable per-node in the future
	helloTime time.Duration
	deadTime  time.Duration
	lastSent  time.Time
	lastRecv  time.Time
}

// are we due to send?
func (ssn *stateSyncNeighbor) due() bool {
	return time.Since(ssn.lastSent) >= ssn.helloTime
}

// have we passed the dead timer?
func (ssn *stateSyncNeighbor) dead() bool {
	return time.Since(ssn.lastRecv) > ssn.deadTime
}

// mark lastRecv as now
func (ssn *stateSyncNeighbor) recv() {
	ssn.lastRecv = time.Now()
}

// mark lastSent as now
func (ssn *stateSyncNeighbor) sent() {
	ssn.lastSent = time.Now()
}

func (ssn *stateSyncNeighbor) nextAction() time.Time {
	d := ssn.lastRecv.Add(ssn.deadTime)
	h := ssn.lastSent.Add(ssn.helloTime)
	if d.Before(h) {
		return d
	}
	return h
}

////////////////////////////
// StateSyncEngine Object /
//////////////////////////

var _ lib.StateSyncEngine = (*StateSyncEngine)(nil)

// StateSyncEngine manages tree-style, eventual consistency state synchronization
type StateSyncEngine struct {
	cfg     ContextSSE
	pool    map[string]*stateSyncNeighbor // it's useful to keep an index
	queue   []*stateSyncNeighbor          // but we need a timing queue too
	query   *QueryEngine
	schan   chan<- lib.EventListener
	tchan   chan interface{} // channel tells us when to wake up and do work
	em      *EventEmitter
	log     lib.Logger
	self    lib.NodeID
	parents []string
	conn    net.PacketConn
	rpc     ContextRPC
}

// NewStateSyncEngine creates a new initialized StateSyncEngine
func NewStateSyncEngine(ctx Context) *StateSyncEngine {
	sse := &StateSyncEngine{
		cfg:     ctx.SSE,
		pool:    make(map[string]*stateSyncNeighbor),
		queue:   []*stateSyncNeighbor{},
		em:      NewEventEmitter(lib.Event_STATE_SYNC),
		query:   &ctx.Query,
		schan:   ctx.SubChan,
		tchan:   make(chan interface{}),
		log:     &ctx.Logger,
		self:    ctx.Self,
		parents: ctx.Parents,
		rpc:     ctx.RPC,
	}
	sse.log.SetModule("StateSyncEngine")
	return sse
}

var _ pb.StateSyncServer = (*StateSyncEngine)(nil)

// RPCPhoneHome is a gRPC call.  It establishes state sync properties with a child.
func (sse *StateSyncEngine) RPCPhoneHome(ctx context.Context, in *pb.PhoneHomeRequest) (out *pb.PhoneHomeReply, e error) {
	// we don't really have any way to make sure this is the right client but timing right now
	id := NewNodeIDFromBinary(in.GetId())
	if id.Nil() {
		e = fmt.Errorf("could not interpet NodeID")
		return
	}
	// see if this node exists
	n, e := sse.query.Read(id)
	if e != nil {
		sse.Logf(NOTICE, "attempted phone home for unknown node: %s", id.String())
		return
	}
	// our one glimmer of security: are we in the correct state?
	v, e := sse.query.GetValue(lib.NodeURLJoin(id.String(), "/RunState"))
	if e != nil {
		return
	}
	if v.Interface() != pb.Node_INIT {
		e = fmt.Errorf("attempted phone home out-of-turn: %s", id.String())
		sse.Logf(NOTICE, "attempted phone home out-of-turn: %s", id.String())
		return
	}
	// ok, proceed
	//_, e = sse.query.SetValue(lib.NodeURLJoin(id.String(), "/RunState"), reflect.ValueOf(pb.Node_SYNC))
	// Node_SYNC should probably be propagated up?  But something needs to keep other nodes from interjecting themselves perhaps.
	sse.addNeighbor(id.String(), false)
	msg, e := sse.nodeToMessage(n.ID().String(), n)
	if e != nil {
		return
	}
	sse.Logf(DEBUG, "successful phone home for: %s", id.String())
	return &pb.PhoneHomeReply{Pid: sse.self.Binary(), Key: sse.pool[id.String()].key, Msg: msg}, nil
}

// Run is a goroutine that makes StateSyncEngine active
func (sse *StateSyncEngine) Run() {
	rchan := make(chan recvPacket) // receive chan
	echan := make(chan lib.Event)  // event chan
	sse.Log(INFO, "starting StateSyncEngine")

	elist := NewEventListener(
		"StateSyncEngine",
		lib.Event_STATE_CHANGE,
		eventFilter,
		func(v lib.Event) error {
			return ChanSender(v, echan)
		})

	// subscribe to events we care about
	sse.schan <- elist

	var e error
	sse.conn, e = net.ListenPacket(sse.cfg.Network, sse.cfg.Addr+":"+strconv.Itoa(sse.cfg.Port))
	if e != nil {
		sse.Logf(ERROR, "StateSyncEngine could not listen to UDP: %v", e)
		return
	}
	go sse.listen(rchan, sse.conn)
	sse.Logf(INFO, "sync protocol listening on %s:%s:%d", sse.cfg.Network, sse.cfg.Addr, sse.cfg.Port)

	s := grpc.NewServer()
	pb.RegisterStateSyncServer(s, sse)
	reflection.Register(s)
	go func(lis net.Listener, s *grpc.Server) {
		if e = s.Serve(lis); e != nil {
			sse.Logf(CRITICAL, "couldn't start RPC service: %v\n", e)
			return
		}
	}(sse.rpc.NetListner, s)

	for _, p := range sse.parents {
		// phone home
		sse.callParent(p)
	}
	if len(sse.parents) < 1 {
		sse.Log(INFO, "no parents specified, I will run as a full-state node")
	}

	debugchan := make(chan interface{})
	// if we're at debug level, wakeup every 10 seconds and print some stuff
	if sse.GetLoggerLevel() >= DDEBUG {
		go func() {
			for {
				time.Sleep(10 * time.Second)
				debugchan <- nil
			}
		}()
	}

	sse.wakeForNext()

	sse.Logf(INFO, "state sync is running for identity: %s", sse.self.String())

	for {
		select {
		case r := <-rchan: // received a hello
			sse.processRecv(r)
			break
		case <-sse.tchan: // due to send a hello
			sse.Logf(DDEBUG, "work timer wakeup")
			sse.catchupSync()
			sse.wakeForNext()
			break
		case v := <-echan:
			sse.eventHandler(v)
			break
		case <-debugchan:
			sse.Logf(DDEBUG, "current pool: %v\n", sse.pool)
			sse.Logf(DDEBUG, "current queue: %v\n", sse.queue)
			break
		}
	}
}

////////////////////////
// Unexported methods /
//////////////////////

// this is an important one!  phone home to your parent, setup upward sync
func (sse *StateSyncEngine) callParent(p string) {
	retryCall := func() {
		sse.Logf(INFO, "retrying phone home in 10s to: %s", p)
		time.Sleep(10 * time.Second)
		sse.callParent(p)
	}

	sse.Logf(INFO, "attempting to phone home to: %s", p)
	conn, e := grpc.Dial(p+":"+strconv.Itoa(sse.rpc.Port), grpc.WithInsecure())
	if e != nil {
		sse.Logf(CRITICAL, "phone home to (%s) failed: p, %v", e)
		go retryCall()
		// TODO: retry phone home?
		return
	}
	defer conn.Close()
	c := pb.NewStateSyncClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, e := c.RPCPhoneHome(ctx, &pb.PhoneHomeRequest{Id: sse.self.Binary()})
	if e != nil {
		sse.Logf(CRITICAL, "could not phone home: %v", e)
		go retryCall()
		return
	}
	// ok! we successfully phoned home, now let's setup our parent neighbor.  Also, register our cfg state
	nid := NewNodeIDFromBinary(r.GetPid())
	sse.addNeighbor(nid.String(), true)
	n := sse.pool[nid.String()]
	n.key = r.Key
	rp, e := sse.ssmToNode(r.Msg)
	if e != nil {
		sse.Logf(ERROR, "malformed response from phone home: %v", e)
		return
	}
	if !rp.Node.ID().Equal(sse.self) {
		sse.Logf(CRITICAL, "we phoned home and got info about someone else: %s", rp.Node.ID().String())
		sse.delNeighbor(nid)
		return
	}
	_, e = sse.query.Update(rp.Node)
	if e != nil {
		sse.Log(ERROR, e.Error())
	}

	// we need to create a stub entry for our parent node
	pn := NewNodeWithID(NewNodeIDFromBinary(r.Pid).String())

	// FIXME: this is a lockin to ipv4; also a hack
	/* this isn't necessary as long as the extension is loaded
	ip := &pb.IPv4OverEthernet{}
	pn.AddExtension(ip)
	*/
	pn.SetValue(sse.cfg.AddrURL, reflect.ValueOf([]byte(net.ParseIP(p).To4())))
	sse.query.Create(pn)

	n.recv()
}

func (sse *StateSyncEngine) nodeGetKey(id string) (key []byte, e error) {
	n, ok := sse.pool[id]
	if !ok {
		e = fmt.Errorf("key not found for %s", id)
	}
	key = n.key
	return
}

func (sse *StateSyncEngine) nodeToMessage(to string, n lib.Node) (msg *pb.StateSyncMessage, e error) {
	key, e := sse.nodeGetKey(to)
	if e != nil {
		return
	}
	m := &pb.StateSyncMessage{}
	m.Id = sse.self.Binary()
	m.Message = n.Binary()
	if e != nil {
		return
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(m.Message)
	m.Hmac = mac.Sum(nil)
	return m, nil
}

func (sse *StateSyncEngine) nodeToBinary(to string, n lib.Node) (msg []byte, e error) {
	m, e := sse.nodeToMessage(to, n)
	if e != nil {
		return
	}
	return proto.Marshal(m)
}

func (sse *StateSyncEngine) ssmToNode(m *pb.StateSyncMessage) (rp recvPacket, e error) {
	rp.From = NewNodeIDFromBinary(m.Id)
	if rp.From.Nil() {
		e = fmt.Errorf("could not unmarshal NodeID")
		return
	}
	key, e := sse.nodeGetKey(rp.From.String())
	if e != nil {
		return
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(m.Message)
	nmac := mac.Sum(nil)
	if !hmac.Equal(m.Hmac, nmac) {
		e = fmt.Errorf("HMAC does not match on packet")
		return
	}
	rp.Node = NewNodeFromBinary(m.Message)
	return
}

func (sse *StateSyncEngine) binaryToNode(buf []byte) (rp recvPacket, e error) {
	m := &pb.StateSyncMessage{}
	e = proto.Unmarshal(buf, m)
	if e != nil {
		return
	}
	return sse.ssmToNode(m)
}

func (sse *StateSyncEngine) send(n *stateSyncNeighbor) {
	node, e := sse.query.Read(n.id)
	if e != nil {
		sse.Logf(ERROR, "couldn't get node info, deleting from pool: %v", e)
		sse.delNeighbor(n.id)
		return
	}
	addr, e := node.GetValue(sse.cfg.AddrURL)
	if e != nil {
		sse.Logf(ERROR, "couldn't get node address, deleting from pool: %s, %v\n", n.id.String(), e)
		sse.delNeighbor(n.id)
		return
	}
	// FIXME: @important sending assumes udp4
	ip := net.IPv4(addr.Bytes()[0], addr.Bytes()[1], addr.Bytes()[2], addr.Bytes()[3])
	if n.parent {
		node, e = sse.query.ReadDsc(sse.self)
		if e != nil {
			sse.Logf(CRITICAL, "couldn't get node info on self")
			return
		}
	}
	msg, _ := sse.nodeToBinary(n.id.String(), node)
	cnt, e := sse.conn.WriteTo(msg, &net.UDPAddr{IP: ip, Port: sse.cfg.Port})
	if e != nil {
		sse.Logf(ERROR, "udp write failed: %v", e)
		return
	}
	if cnt != len(msg) {
		sse.Logf(ERROR, "udp write only %d of %d bytes", cnt, len(msg))
		return
	}
	n.sent()
	sse.Logf(DEBUG, "sent sync to: %s", n.id.String())
}

func (sse *StateSyncEngine) sendDiscoverable(id lib.NodeID)  {}
func (sse *StateSyncEngine) sendConfiguration(id lib.NodeID) {}

func (sse *StateSyncEngine) listen(c chan<- recvPacket, conn net.PacketConn) {
	buffer := make([]byte, 9000)
	for {
		cnt, _, e := conn.ReadFrom(buffer)
		buf := buffer[:cnt]
		if e != nil {
			sse.Logf(ERROR, "UDP read error: %s\n", e)
			continue
		}
		rp, e := sse.binaryToNode(buf)
		if e != nil {
			sse.Logf(DEBUG, "node decode failure: %s\n", e)
			continue
		}

		go func(c chan<- recvPacket, rp recvPacket) {
			c <- rp
		}(c, rp)
	}
}

// we implement a bubble sort, because the queue should stay mostly ordered
// this can probably be managed by only moving items when they change
// queue[0] is the next item
func (sse *StateSyncEngine) sortQueue() {
	j := 0
	swapped := true
	for swapped {
		swapped = false
		for i := 1; i < len(sse.queue); i++ {
			if sse.queue[i-1].nextAction().After(sse.queue[i].nextAction()) {
				sse.queue[i], sse.queue[i-1] = sse.queue[i-1], sse.queue[i]
				swapped = true
				j++
			}
		}
	}
}

func (sse *StateSyncEngine) wakeForNext() {
	var next time.Time
	if len(sse.queue) < 1 {
		next = time.Now().Add(sse.cfg.HelloTime)
	} else {
		next = sse.queue[0].nextAction()
	}
	if !next.After(time.Now()) {
		// we need to do work now!
		sse.tchan <- nil
	} else {
		go func() {
			d := time.Until(next)
			sse.Logf(DDEBUG, "next timer due in: %s\n", d.String())
			time.Sleep(d)
			sse.tchan <- nil
		}()
	}
}

func (sse *StateSyncEngine) addNeighbor(id string, parent bool) {
	nid := NewNodeID(id)
	n := &stateSyncNeighbor{
		parent:    parent,
		key:       sse.generateKey(),
		id:        nid,
		helloTime: sse.cfg.HelloTime,
		deadTime:  sse.cfg.DeadTime,
		lastSent:  time.Now(), // we count creation as a sync
		lastRecv:  time.Now(),
	}
	sse.pool[id] = n
	sse.queue = append(sse.queue, n)
	sse.sortQueue()
}

func (sse *StateSyncEngine) delNeighbor(id lib.NodeID) {
	delete(sse.pool, string(id.String()))
	for i, n := range sse.queue {
		if n.id.Equal(id) {
			sse.queue = append(sse.queue[:i], sse.queue[i+1:]...)
			// no sort; shouldn't disrupt queue order
			return
		}
	}
}

func (sse *StateSyncEngine) sync(n *stateSyncNeighbor) {
	if n.dead() {
		if n.parent {
			// this is pretty bad; lost sync with a parent
			sse.Logf(CRITICAL, "lost sync with parent: %s", n.id.String())
			// drop back to INIT status
			sse.query.SetValueDsc(lib.NodeURLJoin(sse.self.String(), "/RunState"), reflect.ValueOf(pb.Node_ERROR))
			sse.delNeighbor(n.id)
		} else {
			// declare this node to be dead
			// we make the declaration, and delete it from our records
			sse.query.SetValueDsc(lib.NodeURLJoin(n.id.String(), "/RunState"), reflect.ValueOf(pb.Node_ERROR))
			sse.delNeighbor(n.id)
			sse.Logf(INFO, "a neighbor died: %s", n.id.String())
		}
	}
	if n.due() {
		sse.Logf(DEBUG, "sending hello: %s", n.id.String())
		sse.send(n)
		sse.sortQueue()
	}
}
func (sse *StateSyncEngine) syncNext() {
	if len(sse.queue) < 1 {
		return
	}
	sse.sync(sse.queue[0])
}

// sync items on queue until we're caught up
func (sse *StateSyncEngine) catchupSync() {
	for len(sse.queue) > 0 && !time.Now().Before(sse.queue[0].nextAction()) {
		sse.syncNext()
	}
	sse.Log(DDEBUG, "caught up on sync work items")
}

func (sse *StateSyncEngine) processRecv(rp recvPacket) {
	n, ok := sse.pool[rp.From.String()]
	if !ok {
		// got sync from something we don't know about, ignore
		sse.Logf(INFO, "got unexpected hello from: %s", rp.From.String())
		return
	}
	n.recv()
	sse.sortQueue()
	sse.Logf(DEBUG, "got a hello from: %s", rp.From.String())
	if n.parent {
		sse.query.Update(rp.Node)
	} else {
		sse.query.UpdateDsc(rp.Node)
	}
}
func eventFilter(v lib.Event) bool {
	sce := v.Data().(*StateChangeEvent)
	switch sce.Type {
	case StateChange_DELETE:
		return true
	case StateChange_UPDATE:
		_, url := lib.NodeURLSplit(sce.URL)
		// we only care about RunState, really
		if url == "/RunState" {
			return true
		}
	case StateChange_CREATE: // don't do anything until /RunState changes
		fallthrough
	case StateChange_READ:
		fallthrough
	default:
		//ignore
	}
	return false
}

func (sse *StateSyncEngine) eventHandler(v lib.Event) {
	sce := v.Data().(*StateChangeEvent)
	switch sce.Type {
	case StateChange_DELETE:
		// node was deleted; we should delete it if we know about it
		sse.delNeighbor(sce.Value.Interface().(lib.Node).ID())
		break
		/*
			case StateChange_UPDATE:
				// we know this means /RunState updated
				// we'll get the value directly though
				node, _ := lib.NodeURLSplit(sce.URL)
				v, e := sse.query.GetValueDsc(lib.NodeURLJoin(node, "/RunState"))
				if e != nil {
					return // ???
				}
				if v.Interface() == pb.Node_SYNC {
					sse.addNeighbor(node, false)
				}
				break
		*/
	default:
	}
}

// no real reason for this to be part of the object...
func (sse *StateSyncEngine) generateKey() (key []byte) {
	c := 16 // TODO: make key size configurable
	b := make([]byte, c)
	rand.Read(b) // should probably check for errors
	return b
}

////////////////////////////
// Passthrough Interfaces /
//////////////////////////

/*
 * Consume Logger
 */
var _ lib.Logger = (*StateSyncEngine)(nil)

func (sse *StateSyncEngine) Log(level lib.LoggerLevel, m string) { sse.log.Log(level, m) }
func (sse *StateSyncEngine) Logf(level lib.LoggerLevel, fmt string, v ...interface{}) {
	sse.log.Logf(level, fmt, v...)
}
func (sse *StateSyncEngine) SetModule(name string)                { sse.log.SetModule(name) }
func (sse *StateSyncEngine) GetModule() string                    { return sse.log.GetModule() }
func (sse *StateSyncEngine) SetLoggerLevel(level lib.LoggerLevel) { sse.log.SetLoggerLevel(level) }
func (sse *StateSyncEngine) GetLoggerLevel() lib.LoggerLevel      { return sse.log.GetLoggerLevel() }
func (sse *StateSyncEngine) IsEnabledFor(level lib.LoggerLevel) bool {
	return sse.log.IsEnabledFor(level)
}

/*
 * Consume an emitter, so we implement EventEmitter directly
 */
var _ lib.EventEmitter = (*StateSyncEngine)(nil)

func (sse *StateSyncEngine) Subscribe(id string, c chan<- []lib.Event) error {
	return sse.em.Subscribe(id, c)
}
func (sse *StateSyncEngine) Unsubscribe(id string) error { return sse.em.Unsubscribe(id) }
func (sse *StateSyncEngine) Emit(v []lib.Event)          { sse.em.Emit(v) }
func (sse *StateSyncEngine) EmitOne(v lib.Event)         { sse.em.EmitOne(v) }
func (sse *StateSyncEngine) EventType() lib.EventType    { return sse.em.EventType() }
