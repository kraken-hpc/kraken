/* StateSyncEngine.go: implements the binary state synchronization protocol
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I proto/src -I proto --gogo_out=plugins=grpc:proto proto/src/StateSyncMessage.proto

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
	"sync"
	"time"

	pb "github.com/hpc/kraken/core/proto"
	ct "github.com/hpc/kraken/core/proto/customtypes"
	ipv4t "github.com/hpc/kraken/extensions/ipv4/customtypes"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"

	"github.com/gogo/protobuf/proto"
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
	From types.NodeID
	Node types.Node
}

/*
 * stateSyncNeighbor: keeps track of timers on a neighbor
 */

type stateSyncNeighbor struct {
	lock   sync.Mutex
	parent bool // sync up? (or down)
	key    []byte
	id     types.NodeID
	// this allows us to have this configurable per-node in the future
	helloTime time.Duration
	deadTime  time.Duration
	lastSent  time.Time
	lastRecv  time.Time
}

// are we due to send?
func (ssn *stateSyncNeighbor) due() bool {
	ssn.lock.Lock()
	defer ssn.lock.Unlock()
	return time.Since(ssn.lastSent) >= ssn.helloTime
}

// have we passed the dead timer?
func (ssn *stateSyncNeighbor) dead() bool {
	ssn.lock.Lock()
	defer ssn.lock.Unlock()
	return time.Since(ssn.lastRecv) > ssn.deadTime
}

// mark lastRecv as now
func (ssn *stateSyncNeighbor) recv() {
	ssn.lock.Lock()
	defer ssn.lock.Unlock()
	ssn.lastRecv = time.Now()
}

// mark lastSent as now
func (ssn *stateSyncNeighbor) sent() {
	ssn.lock.Lock()
	defer ssn.lock.Unlock()
	ssn.lastSent = time.Now()
}

func (ssn *stateSyncNeighbor) nextAction() time.Time {
	ssn.lock.Lock()
	defer ssn.lock.Unlock()
	d := ssn.lastRecv.Add(ssn.deadTime)
	h := ssn.lastSent.Add(ssn.helloTime)
	if d.Before(h) {
		return d
	}
	return h
}

// locking accessors

func (ssn *stateSyncNeighbor) getKey() []byte {
	ssn.lock.Lock()
	defer ssn.lock.Unlock()
	return ssn.key
}

func (ssn *stateSyncNeighbor) getParent() bool {
	ssn.lock.Lock()
	defer ssn.lock.Unlock()
	return ssn.parent
}

func (ssn *stateSyncNeighbor) getID() types.NodeID {
	ssn.lock.Lock()
	defer ssn.lock.Unlock()
	return ssn.id
}

////////////////////////////
// StateSyncEngine Object /
//////////////////////////

var _ types.StateSyncEngine = (*StateSyncEngine)(nil)

// StateSyncEngine manages tree-style, eventual consistency state synchronization
type StateSyncEngine struct {
	cfg ContextSSE

	lock  sync.RWMutex                  // applies to pool & queue
	pool  map[string]*stateSyncNeighbor // it's useful to keep an index
	queue []*stateSyncNeighbor          // but we need a timing queue too

	query   *QueryEngine
	schan   chan<- types.EventListener
	tchan   chan interface{} // channel tells us when to wake up and do work
	em      *EventEmitter
	log     types.Logger
	self    types.NodeID
	parents []string
	conn    net.PacketConn
	rpc     ContextRPC
}

// NewStateSyncEngine creates a new initialized StateSyncEngine
func NewStateSyncEngine(ctx Context) *StateSyncEngine {
	sse := &StateSyncEngine{
		cfg:     ctx.SSE,
		lock:    sync.RWMutex{},
		pool:    make(map[string]*stateSyncNeighbor),
		queue:   []*stateSyncNeighbor{},
		em:      NewEventEmitter(types.Event_STATE_SYNC),
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

// implement types.ServiceInstance
// this is a bit of a hack
func (sse *StateSyncEngine) ID() string                               { return sse.Name() }
func (sse *StateSyncEngine) Module() string                           { return sse.Name() }
func (sse *StateSyncEngine) Start()                                   {} //NOP
func (sse *StateSyncEngine) Stop()                                    {} //NOP
func (sse *StateSyncEngine) GetState() types.ServiceState             { return types.Service_RUN }
func (sse *StateSyncEngine) UpdateConfig()                            {} //NOP
func (sse *StateSyncEngine) Watch(chan<- types.ServiceInstanceUpdate) {} //NOP
func (sse *StateSyncEngine) SetCtl(chan<- types.ServiceControl)       {} //NOP
func (sse *StateSyncEngine) SetSock(string)                           {} //NOP

// implement types.Module
func (*StateSyncEngine) Name() string { return "sse" }

var _ pb.StateSyncServer = (*StateSyncEngine)(nil)

// RPCPhoneHome is a gRPC call.  It establishes state sync properties with a child.
func (sse *StateSyncEngine) RPCPhoneHome(ctx context.Context, in *pb.PhoneHomeRequest) (out *pb.PhoneHomeReply, e error) {
	// we don't really have any way to make sure this is the right client but timing right now
	id := ct.NewNodeIDFromBinary(in.GetId())
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
	v, e := sse.query.GetValueDsc(util.NodeURLJoin(id.String(), "/RunState"))
	if e != nil {
		return
	}
	if v.Interface() != pb.Node_INIT {
		e = fmt.Errorf("attempted phone home out-of-turn: %s is %v", id.String(), v.Interface())
		sse.Logf(NOTICE, "attempted phone home out-of-turn: %s is %v", id.String(), v.Interface())
		return
	}
	// ok, proceed
	//_, e = sse.query.SetValue(util.NodeURLJoin(id.String(), "/RunState"), reflect.ValueOf(pb.Node_SYNC))
	// Node_SYNC should probably be propagated up?  But something needs to keep other nodes from interjecting themselves perhaps.
	// We do this as a discovery instead...
	url := util.NodeURLJoin(id.String(), "/RunState")
	ev := NewEvent(
		types.Event_DISCOVERY,
		url,
		&DiscoveryEvent{
			ID:      "sse",
			URL:     url,
			ValueID: "SYNC",
		},
	)
	sse.EmitOne(ev)
	_, ok := sse.getNeighbor(id)
	if ok {
		sse.Logf(DEBUG, "deleting stale neighbor: %s", id.String())
		sse.delNeighbor(id)
	}
	ssn := sse.addNeighbor(id.String(), false)
	cfg, e := sse.nodeToMessage(n.ID(), n)
	if e != nil {
		return
	}
	// This is redundant with the discovery above, but we need to make sure it's set before we send it
	// But we also want to be a good citizen and send a discovery
	_, e = sse.query.SetValueDsc(util.NodeURLJoin(id.String(), "/RunState"), reflect.ValueOf(pb.Node_SYNC))

	nd, _ := sse.query.ReadDsc(id)

	// the following check *should* be unnecessary, and maybe can be removed some day
	rs, _ := nd.GetValue("/RunState")
	if rs.Interface() != pb.Node_SYNC {
		sse.Logf(DEBUG, "sending a phone home reply, but /RunState != SYNC, this shouldn't happen: %s", id.String())
	}

	dsc, e := sse.nodeToMessage(n.ID(), nd)
	if e != nil {
		return
	}
	sse.Logf(DEBUG, "successful phone home for: %s", id.String())
	return &pb.PhoneHomeReply{Pid: sse.self.Bytes(), Key: ssn.getKey(), Cfg: cfg, Dsc: dsc}, nil
}

// Run is a goroutine that makes StateSyncEngine active
func (sse *StateSyncEngine) Run(ready chan<- interface{}) {
	rchan := make(chan recvPacket)  // receive chan
	echan := make(chan types.Event) // event chan
	sse.Log(INFO, "starting StateSyncEngine")

	elist := NewEventListener(
		"StateSyncEngine",
		types.Event_STATE_CHANGE,
		eventFilter,
		func(v types.Event) error {
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

	ready <- nil
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
			sse.lock.RLock()
			sse.Logf(DDEBUG, "current pool: %v\n", sse.pool)
			sse.Logf(DDEBUG, "current queue: %v\n", sse.queue)
			sse.lock.RUnlock()
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
	r, e := c.RPCPhoneHome(ctx, &pb.PhoneHomeRequest{Id: sse.self.Bytes()})
	if e != nil {
		sse.Logf(CRITICAL, "could not phone home: %v", e)
		go retryCall()
		return
	}
	// ok! we successfully phoned home, now let's setup our parent neighbor.  Also, register our cfg state
	nid := ct.NewNodeIDFromBinary(r.GetPid())
	n := sse.addNeighbor(nid.String(), true)
	n.lock.Lock()
	n.key = r.Key
	n.lock.Unlock()
	rp, e := sse.ssmToNode(r.Cfg)
	if e != nil {
		sse.Logf(ERROR, "malformed response from phone home: %v", e)
		return
	}
	rpd, e := sse.ssmToNode(r.Dsc)
	if e != nil {
		sse.Logf(ERROR, "malformed response from phone home: %v", e)
		return
	}
	if !rp.Node.ID().EqualTo(sse.self) {
		sse.Logf(CRITICAL, "we phoned home and got info about someone else: %s", rp.Node.ID().String())
		sse.delNeighbor(nid)
		return
	}
	_, e = sse.query.UpdateDsc(rpd.Node)
	if e != nil {
		sse.Log(ERROR, e.Error())
	}
	_, e = sse.query.Update(rp.Node)
	if e != nil {
		sse.Log(ERROR, e.Error())
	}

	// we need to create a stub entry for our parent node
	pn := NewNodeWithID(ct.NewNodeIDFromBinary(r.Pid).String())

	// FIXME: this is a lockin to ipv4; also a hack
	/* this isn't necessary as long as the extension is loaded
	ip := &pb.IPv4OverEthernet{}
	pn.AddExtension(ip)
	*/
	pn.SetValue(sse.cfg.AddrURL, reflect.ValueOf(ipv4t.IP{IP: net.ParseIP(p)}))
	sse.query.Create(pn)

	n.recv()
	e = sse.query.Thaw()
	if e != nil {
		sse.Log(ERROR, e.Error())
	}
}

func (sse *StateSyncEngine) nodeGetKey(id types.NodeID) (key []byte, e error) {
	n, ok := sse.getNeighbor(id)
	if !ok {
		e = fmt.Errorf("key not found for %s", id.String())
		return
	}
	key = n.getKey()
	return
}

func (sse *StateSyncEngine) nodeToMessage(to types.NodeID, n types.Node) (msg *pb.StateSyncMessage, e error) {
	key, e := sse.nodeGetKey(to)
	if e != nil {
		return
	}
	m := &pb.StateSyncMessage{}
	m.Id = sse.self.Bytes()
	m.Message = n.Binary()
	if e != nil {
		return
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(m.Message)
	m.Hmac = mac.Sum(nil)
	return m, nil
}

func (sse *StateSyncEngine) nodeToBinary(to types.NodeID, n types.Node) (msg []byte, e error) {
	m, e := sse.nodeToMessage(to, n)
	if e != nil {
		return
	}
	return proto.Marshal(m)
}

func (sse *StateSyncEngine) ssmToNode(m *pb.StateSyncMessage) (rp recvPacket, e error) {
	rp.From = ct.NewNodeIDFromBinary(m.Id)
	if rp.From.Nil() {
		e = fmt.Errorf("could not unmarshal NodeID")
		return
	}
	key, e := sse.nodeGetKey(rp.From)
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
	node, e := sse.query.Read(n.getID())
	if e != nil {
		sse.Logf(ERROR, "couldn't get node info, deleting from pool: %v", e)
		sse.delNeighbor(n.getID())
		return
	}
	addr, e := node.GetValue(sse.cfg.AddrURL)
	if e != nil {
		sse.Logf(ERROR, "couldn't get node address, deleting from pool: %s, %v\n", n.getID().String(), e)
		sse.delNeighbor(n.getID())
		return
	}
	// FIXME: @important sending assumes udp4
	ip := addr.Interface().(ipv4t.IP)
	if n.getParent() {
		node, e = sse.query.ReadDsc(sse.self)
		if e != nil {
			sse.Logf(CRITICAL, "couldn't get node info on self")
			return
		}
	}
	msg, _ := sse.nodeToBinary(n.getID(), node)
	cnt, e := sse.conn.WriteTo(msg, &net.UDPAddr{IP: ip.IP, Port: sse.cfg.Port})
	if e != nil {
		sse.Logf(ERROR, "udp write failed: %v", e)
		return
	}
	if cnt != len(msg) {
		sse.Logf(ERROR, "udp write only %d of %d bytes", cnt, len(msg))
		return
	}
	n.sent()
}

func (sse *StateSyncEngine) sendDiscoverable(id types.NodeID)  {}
func (sse *StateSyncEngine) sendConfiguration(id types.NodeID) {}

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
	sse.lock.Lock()
	defer sse.lock.Unlock()
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
	sse.lock.RLock()
	if len(sse.queue) < 1 {
		next = time.Now().Add(sse.cfg.HelloTime)
	} else {
		next = sse.queue[0].nextAction()
	}
	sse.lock.RUnlock()
	d := time.Until(next)
	if !next.After(time.Now()) {
		// we need to do work now!
		d = 0
	}
	// we *must* make this a goroutine, or we introduce deadlocks
	go func(d time.Duration) {
		sse.Logf(DDEBUG, "next timer due in: %s\n", d.String())
		time.Sleep(d)
		sse.tchan <- nil
	}(d)
}

func (sse *StateSyncEngine) addNeighbor(id string, parent bool) *stateSyncNeighbor {
	nid := ct.NewNodeID(id)
	n := &stateSyncNeighbor{
		lock:      sync.Mutex{},
		parent:    parent,
		key:       sse.generateKey(),
		id:        nid,
		helloTime: sse.cfg.HelloTime,
		deadTime:  sse.cfg.DeadTime,
		lastSent:  time.Now(), // we count creation as a sync
		lastRecv:  time.Now(),
	}
	sse.lock.Lock()
	sse.pool[id] = n
	sse.queue = append(sse.queue, n)
	sse.lock.Unlock()

	sse.sortQueue()
	return n
}

func (sse *StateSyncEngine) delNeighbor(id types.NodeID) {
	sse.lock.Lock()
	defer sse.lock.Unlock()
	n := sse.pool[id.String()]
	n.lock.Lock()
	defer n.lock.Unlock()
	delete(sse.pool, string(id.String()))
	for i, n := range sse.queue {
		if n.id.EqualTo(id) {
			sse.queue = append(sse.queue[:i], sse.queue[i+1:]...)
			// no sort; shouldn't disrupt queue order
			return
		}
	}
}

// just a map query, but with locking
func (sse *StateSyncEngine) getNeighbor(id types.NodeID) (n *stateSyncNeighbor, ok bool) {
	sse.lock.RLock()
	defer sse.lock.RUnlock()
	n, ok = sse.pool[id.String()]
	return
}

func (sse *StateSyncEngine) getNeighborMust(id types.NodeID) (n *stateSyncNeighbor) {
	var ok bool
	n, ok = sse.getNeighbor(id)
	if ok {
		return n
	}
	return nil
}

func (sse *StateSyncEngine) sync(n *stateSyncNeighbor) {
	if n.dead() {
		if n.getParent() {
			// this is pretty bad; lost sync with a parent
			sse.Logf(CRITICAL, "lost sync with parent: %s", n.getID().String())
			// drop back to INIT status
			//sse.query.SetValueDsc(util.NodeURLJoin(sse.self.String(), "/RunState"), reflect.ValueOf(pb.Node_ERROR))
			url := util.NodeURLJoin(sse.self.String(), "/RunState")
			ev := NewEvent(
				types.Event_DISCOVERY,
				url,
				&DiscoveryEvent{
					ID:      "sse",
					URL:     url,
					ValueID: "ERROR",
				},
			)
			sse.EmitOne(ev)
			sse.delNeighbor(n.getID())
		} else {

			// before we assume this node went to error, make sure it's actually in SYNC
			// this can happen if, e.g. we got an unexpected event and devolved but SSE didn't notice
			cur, e := sse.query.GetValueDsc(util.NodeURLJoin(n.getID().String(), "/RunState"))
			if e != nil {
				sse.Logf(INFO, "lost sync on a non-existent node?: %s, %v", n.getID().String(), e)
			}

			if cur.Interface() == pb.Node_SYNC {
				// ok, we thought we were in sync; a neighbor died
				// declare this node to be dead
				// we make the declaration, and delete it from our records
				url := util.NodeURLJoin(n.getID().String(), "/RunState")
				ev := NewEvent(
					types.Event_DISCOVERY,
					url,
					&DiscoveryEvent{
						ID:      "sse",
						URL:     url,
						ValueID: "ERROR",
					},
				)
				sse.EmitOne(ev)
				sse.Logf(INFO, "a neighbor died: %s", n.getID().String())
			} else {
				// we actually didn't think we were in SYNC anyway
				sse.Logf(DEBUG, "lost sync on a node that wasn't in SYNC: %s", n.getID().String())
			}
			sse.delNeighbor(n.getID()) // in all cases, we need to delete this neighbor
		}
	}
	if n.due() {
		sse.Logf(DEBUG, "sending hello: %s", n.getID().String())
		sse.send(n)
		sse.sortQueue()
	}
}

// returns nil if queue is empty
func (sse *StateSyncEngine) queueGetNext() (n *stateSyncNeighbor) {
	sse.lock.RLock()
	defer sse.lock.RUnlock()
	if len(sse.queue) < 1 {
		return
	}
	return sse.queue[0]
}

// sync items on queue until we're caught up
func (sse *StateSyncEngine) catchupSync() {
	n := sse.queueGetNext()
	for n != nil && !time.Now().Before(n.nextAction()) {
		sse.sync(n)
		n = sse.queueGetNext()
	}
	sse.Log(DDEBUG, "caught up on sync work items")
}

func (sse *StateSyncEngine) processRecv(rp recvPacket) {
	n, ok := sse.getNeighbor(rp.From)
	if !ok {
		// got sync from something we don't know about, ignore
		sse.Logf(INFO, "got unexpected hello from: %s", rp.From.String())
		return
	}
	n.recv()
	sse.sortQueue()
	sse.Logf(DEBUG, "got a hello from: %s", rp.From.String())
	if n.getParent() {
		_, e := sse.query.Update(rp.Node)
		if e != nil {
			sse.log.Logf(types.LLERROR, "Received Error while updating cfg: %v", e)
		}
	} else {
		_, e := sse.query.UpdateDsc(rp.Node)
		if e != nil {
			sse.log.Logf(types.LLERROR, "Received Error while updating dsc for %s: %v", rp.From.String(), e)
		}
	}
}

func eventFilter(v types.Event) bool {
	sce := v.Data().(*StateChangeEvent)
	switch sce.Type {
	case StateChange_DELETE:
		return true
	case StateChange_UPDATE:
		_, url := util.NodeURLSplit(sce.URL)
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

func (sse *StateSyncEngine) eventHandler(v types.Event) {
	sce := v.Data().(*StateChangeEvent)
	switch sce.Type {
	case StateChange_DELETE:
		// node was deleted; we should delete it if we know about it
		sse.delNeighbor(sce.Value.Interface().(types.Node).ID())
		break
		/*
			case StateChange_UPDATE:
				// we know this means /RunState updated
				// we'll get the value directly though
				node, _ := util.NodeURLSplit(sce.URL)
				v, e := sse.query.GetValueDsc(util.NodeURLJoin(node, "/RunState"))
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
var _ types.Logger = (*StateSyncEngine)(nil)

func (sse *StateSyncEngine) Log(level types.LoggerLevel, m string) { sse.log.Log(level, m) }
func (sse *StateSyncEngine) Logf(level types.LoggerLevel, fmt string, v ...interface{}) {
	sse.log.Logf(level, fmt, v...)
}
func (sse *StateSyncEngine) SetModule(name string)                  { sse.log.SetModule(name) }
func (sse *StateSyncEngine) GetModule() string                      { return sse.log.GetModule() }
func (sse *StateSyncEngine) SetLoggerLevel(level types.LoggerLevel) { sse.log.SetLoggerLevel(level) }
func (sse *StateSyncEngine) GetLoggerLevel() types.LoggerLevel      { return sse.log.GetLoggerLevel() }
func (sse *StateSyncEngine) IsEnabledFor(level types.LoggerLevel) bool {
	return sse.log.IsEnabledFor(level)
}

/*
 * Consume an emitter, so we implement EventEmitter directly
 */
var _ types.EventEmitter = (*StateSyncEngine)(nil)

func (sse *StateSyncEngine) Subscribe(id string, c chan<- []types.Event) error {
	return sse.em.Subscribe(id, c)
}
func (sse *StateSyncEngine) Unsubscribe(id string) error { return sse.em.Unsubscribe(id) }
func (sse *StateSyncEngine) Emit(v []types.Event)        { sse.em.Emit(v) }
func (sse *StateSyncEngine) EmitOne(v types.Event)       { sse.em.EmitOne(v) }
func (sse *StateSyncEngine) EventType() types.EventType  { return sse.em.EventType() }

//////////
// Init /
////////

// we need to declare a couple of mutations & discoveries
func init() {
	discoverables := map[string]map[string]reflect.Value{
		"/RunState": {
			"INIT":  reflect.ValueOf(pb.Node_INIT),
			"SYNC":  reflect.ValueOf(pb.Node_SYNC),
			"ERROR": reflect.ValueOf(pb.Node_ERROR),
		},
		"/PhysState": {
			"HANG": reflect.ValueOf(pb.Node_PHYS_HANG),
		},
	}
	mutations := map[string]types.StateMutation{
		"INITtoSYNC": NewStateMutation(
			map[string][2]reflect.Value{
				"/RunState": {
					reflect.ValueOf(pb.Node_INIT),
					reflect.ValueOf(pb.Node_SYNC),
				},
			},
			map[string]reflect.Value{
				"/PhysState": reflect.ValueOf(pb.Node_POWER_ON),
			},
			map[string]reflect.Value{},
			types.StateMutationContext_CHILD,
			time.Second*90, // FIXME: don't hardcode values
			[3]string{"sse", "/PhysState", "HANG"},
		),
	}
	Registry.RegisterDiscoverable(&StateSyncEngine{}, discoverables)
	Registry.RegisterMutations(&StateSyncEngine{}, mutations)
}
