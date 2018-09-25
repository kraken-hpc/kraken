/* StateMutationEngine.go: In many ways, the heart of Kraken, this engine manages state mutations.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
)

///////////////////////
// Auxiliary Objects /
/////////////////////

const (
	MutationEvent_MUTATE    pb.MutationControl_Type = pb.MutationControl_MUTATE
	MutationEvent_INTERRUPT pb.MutationControl_Type = pb.MutationControl_INTERRUPT
)

type MutationEvent struct {
	Type pb.MutationControl_Type
	// strictly speaking, we may only need the Cfg
	// but we generally have this info on hand anyway
	NodeCfg  lib.Node
	NodeDsc  lib.Node
	Mutation [2]string // [0] = module, [1] = mutid
}

type mutationEdge struct {
	cost uint32
	mut  lib.StateMutation
	from *mutationNode
	to   *mutationNode
}

type mutationNode struct {
	spec lib.StateSpec // spec with aggregated require/excludes
	in   []*mutationEdge
	out  []*mutationEdge
}

type mutationPath struct {
	cur    int // where are we currently?
	start  lib.Node
	end    lib.Node
	gstart *mutationNode
	gend   *mutationNode
	chain  []*mutationEdge
	timer  *time.Timer
}

// DefaultRootSpec provides a sensible root StateSpec to build the mutation graph off of
func DefaultRootSpec() lib.StateSpec {
	return NewStateSpec(map[string]reflect.Value{"/PhysState": reflect.ValueOf(pb.Node_PHYS_UNKNOWN)}, map[string]reflect.Value{})
}

////////////////////////////////
// StateMutationEngine Object /
//////////////////////////////

var _ lib.StateMutationEngine = (*StateMutationEngine)(nil)

// A StateMutationEngine listens for state change events and manages mutations to evolve Dsc state into Cfg state
type StateMutationEngine struct {
	muts        []lib.StateMutation
	mutResolver map[lib.StateMutation][2]string // this allows us to lookup module/id pair from mutation
	// stuff we can compute from muts
	mutators    map[string]uint32 // ref count, all URLs that mutate
	requires    map[string]uint32 // ref count, referenced (req/exc) urls that don't mutate
	graph       *mutationNode     // graph start
	nodes       []*mutationNode   // so we can search for matches
	edges       []*mutationEdge
	em          *EventEmitter
	schan       chan<- lib.EventListener // subscription channel
	echan       chan lib.Event
	selist      *EventListener
	run         bool                     // are we running?
	active      map[string]*mutationPath // active mutations
	activeMutex *sync.Mutex              // active needs some synchronization, or we can get in bad places
	query       *QueryEngine
	log         lib.Logger
	self        lib.NodeID
	root        lib.StateSpec
}

// NewStateMutationEngine creates an initialized StateMutationEngine
func NewStateMutationEngine(ctx Context) *StateMutationEngine {
	sme := &StateMutationEngine{
		muts:        []lib.StateMutation{},
		mutResolver: make(map[lib.StateMutation][2]string),
		active:      make(map[string]*mutationPath),
		activeMutex: &sync.Mutex{},
		mutators:    make(map[string]uint32),
		requires:    make(map[string]uint32),
		graph:       &mutationNode{spec: ctx.SME.RootSpec},
		nodes:       []*mutationNode{},
		edges:       []*mutationEdge{},
		em:          NewEventEmitter(lib.Event_STATE_MUTATION),
		run:         false,
		echan:       make(chan lib.Event),
		query:       &ctx.Query,
		schan:       ctx.SubChan,
		log:         &ctx.Logger,
		self:        ctx.Self,
		root:        ctx.SME.RootSpec,
	}
	sme.log.SetModule("StateMutationEngine")
	return sme
}

// RegisterMutation injects new mutaitons into the SME. muts[i] should match callback[i]
// We take a list so that we only call onUpdate once
func (sme *StateMutationEngine) RegisterMutation(module, id string, mut lib.StateMutation) (e error) {
	sme.muts = append(sme.muts, mut)
	sme.mutResolver[mut] = [2]string{module, id}
	sme.onUpdate()
	return
}

// NodeMatch determines how many compatable StateSpecs this node has in the graph
func (sme *StateMutationEngine) NodeMatch(node lib.Node) (i int) {
	ns := sme.nodeSearch(node)
	sme.Logf(DEBUG, "===\nNode:\n%v\n", string(node.JSON()))
	sme.Log(DEBUG, "Matched:\n")
	for _, m := range ns {
		sme.Logf(DEBUG, "Spec:\nreq: %v\nexc: %v\n", m.spec.Requires(), m.spec.Excludes())
	}
	return len(sme.nodeSearch(node))
}

// DumpGraph FIXME: REMOVE -- for debugging
func (sme *StateMutationEngine) DumpGraph() {
	fmt.Printf("\n")
	fmt.Printf("=== START: Mutators URLs ===\n")
	for k, v := range sme.mutators {
		fmt.Printf("%s: %d\n", k, v)
	}
	fmt.Printf("=== END: Mutators URLs ===\n")
	fmt.Printf("=== START: Requires URLs ===\n")
	for k, v := range sme.requires {
		fmt.Printf("%s: %d\n", k, v)
	}
	fmt.Printf("=== END: Requires URLs ===\n")
	fmt.Printf("\n=== START: Node list ===\n")
	for _, m := range sme.nodes {
		fmt.Printf(`
		Node: %p
		 Spec: %p
		  req: %v
		  exc: %v
		 In: %v
		 Out: %v
		 `, m, m.spec, m.spec.Requires(), m.spec.Excludes(), m.in, m.out)
	}
	fmt.Printf("\n=== END: Node list ===\n")
	fmt.Printf("\n=== START: Edge list ===\n")
	for _, m := range sme.edges {
		fmt.Printf(`
		Edge: %p
		 Mutation: %p
		  mut: %v
		  req: %v
		  exc: %v
		 From: %p
		 To: %p
		`, m, m.mut, m.mut.Mutates(), m.mut.Requires(), m.mut.Excludes(), m.from, m.to)
	}
	fmt.Printf("\n=== END: Edge list ===\n")
}

// PathExists returns a boolean indicating whether or not a path exists in the graph between two nodes.
// If the path doesn't exist, it also returns the error.
func (sme *StateMutationEngine) PathExists(start lib.Node, end lib.Node) (r bool, e error) {
	p, e := sme.findPath(start, end)
	if p != nil {
		r = true
	}
	return
}

// Run is a goroutine that listens for state changes and performs StateMutation magic
func (sme *StateMutationEngine) Run() {
	// on run we import all mutations in the registry
	for mod := range Registry.Mutations {
		for id, mut := range Registry.Mutations[mod] {
			sme.muts = append(sme.muts, mut)
			sme.mutResolver[mut] = [2]string{mod, id}
		}
	}
	sme.onUpdate()
	//sme.DumpGraph() // Use this to debug your graph

	// create a listener for state change events we care about
	sme.selist = NewEventListener(
		"StateMutationEngine",
		lib.Event_STATE_CHANGE,
		func(v lib.Event) bool {
			_, url := lib.NodeURLSplit(v.URL())
			for m := range sme.mutators { // NOTE: doesn't fix beginning slashes, etc
				if url == m {
					return true
				}
			}
			if url == "" { // this should mean we got CREATE/DELETE
				return true
			}
			return false
		},
		func(v lib.Event) error { return ChanSender(v, sme.echan) })

	// subscribe our listener
	sme.schan <- sme.selist

	debugchan := make(chan interface{})
	if sme.GetLoggerLevel() >= DDEBUG {
		go func() {
			for {
				time.Sleep(10 * time.Second)
				debugchan <- nil
			}
		}()
	}

	for {
		select {
		case v := <-sme.echan:
			// FIXME: event processing can be expensive;
			// we should make them concurrent with a queue
			sme.handleEvent(v)
			break
		case <-debugchan:
			sme.Logf(DDEBUG, "There are %d active mutations.", len(sme.active))
			break
		}
	}
}

////////////////////////
// Unexported methods /
//////////////////////

func (sme *StateMutationEngine) collectURLs() {
	for _, m := range sme.muts {
		for u := range m.Mutates() {
			if _, ok := sme.mutators[u]; !ok {
				sme.mutators[u] = 0
			}
			sme.mutators[u]++
		}
	}
	// We do this as a separate loop because we don't want mutators in requires
	for _, m := range sme.muts {
		for u := range m.Requires() {
			if _, ok := sme.mutators[u]; ok {
				//skip if we've already registered as a mutator
				continue
			}
			if _, ok := sme.requires[u]; !ok {
				sme.requires[u] = 0
			}
			sme.requires[u]++
		}
		// sme.requires is a bit of a misnomer.
		// really we're interested in any url we depend on to asses, including excludes.
		for u := range m.Excludes() {
			if _, ok := sme.mutators[u]; ok {
				//skip if we've already registered as a mutator
				continue
			}
			if _, ok := sme.requires[u]; !ok {
				sme.requires[u] = 0
			}
			sme.requires[u]++

		}
	}
}

func (sme *StateMutationEngine) remapToNode(root *mutationNode, to *mutationNode) {
	realNode := to
	inEdge := root.in[0]
	inEdge.to = realNode
	realNode.in = append(realNode.in, inEdge)
}

// buildGraph builds the graph of Specs/Mutations.  It is depth-first, recursive.
// TODO: this function may eventually need recursion protection
func (sme *StateMutationEngine) buildGraph(root *mutationNode, seenNode map[lib.StateSpec]*mutationNode, seenMut map[int]*mutationNode, chain []*mutationNode) (nodes []*mutationNode, edges []*mutationEdge) {
	nodes = append(nodes, root)
	edges = []*mutationEdge{}

	// There are two thing that can make a node equal:
	// 1) we have seen this exact node spec...
	for sp, n := range seenNode {
		if sp.Equal(root.spec) {
			// we've seen an identical spec already
			sme.remapToNode(root, n)
			return []*mutationNode{}, []*mutationEdge{}
		}
	}

	for i, m := range sme.muts {
		if m.SpecCompatWithMutators(root.spec, sme.mutators) {
			// ...or, 2) we have hit the same mutation in the same chain.
			if n, ok := seenMut[i]; ok {
				// Ok, I've seen this mutation -> I'm not actually a new node
				// Which node am I? -> seen[i]
				sme.remapToNode(root, n)
				return []*mutationNode{}, []*mutationEdge{}
			}
			nme := &mutationEdge{
				cost: 1,
				mut:  m,
				from: root,
			}
			nn := &mutationNode{
				spec: root.spec.SpecMergeMust(m.After()),
				in:   []*mutationEdge{nme},
				out:  []*mutationEdge{},
			}
			nme.to = nn
			root.out = append(root.out, nme)
			//ineffient, but every chain needs its own copy of seenMut
			newseenMut := make(map[int]*mutationNode)
			for k := range seenMut {
				newseenMut[k] = seenMut[k]
			}
			newseenMut[i] = root
			seenNode[root.spec] = root
			nds, eds := sme.buildGraph(nn, seenNode, newseenMut, append(chain, root))
			edges = append(edges, nme)
			edges = append(edges, eds...)
			nodes = append(nodes, nds...)
		}
	}
	return
}
func (sme *StateMutationEngine) clearGraph() {
	sme.mutators = make(map[string]uint32)
	sme.requires = make(map[string]uint32)
	sme.graph.in = []*mutationEdge{}
	sme.graph.out = []*mutationEdge{}
	sme.graph.spec = sme.root
}

// onUpdate should get called any time a new mutation is registered
func (sme *StateMutationEngine) onUpdate() {
	sme.clearGraph()
	sme.collectURLs()
	sme.nodes, sme.edges = sme.buildGraph(sme.graph, make(map[lib.StateSpec]*mutationNode), make(map[int]*mutationNode), []*mutationNode{})
	sme.Logf(DEBUG, "Built graph [ Mutations: %d Mutation URLs: %d Requires URLs: %d Graph Nodes: %d Graph Edges: %d ]",
		len(sme.muts), len(sme.mutators), len(sme.requires), len(sme.nodes), len(sme.edges))
}

func (sme *StateMutationEngine) nodeSearch(node lib.Node) (mns []*mutationNode) {
	for _, n := range sme.nodes {
		if n.spec.NodeMatch(node) {
			mns = append(mns, n)
		}
	}
	return
}

func (sme *StateMutationEngine) boundarySearch(start lib.Node, end lib.Node) (gstart []*mutationNode, gend []*mutationNode) {
	startMerge := sme.dscNodeMeld(end, start)
	for _, n := range sme.nodes {
		// in general, we don't want the graph root as an option
		if n != sme.graph && n.spec.NodeMatchWithMutators(startMerge, sme.mutators) {
			gstart = append(gstart, n)
		}
		if n != sme.graph && n.spec.NodeMatchWithMutators(end, sme.mutators) {
			gend = append(gend, n)
		}
	}
	// there's one exception: we may be starting on the graph root (if nothing else matched)
	if len(gstart) == 0 {
		gstart = append(gstart, sme.graph)
	}
	return
}

// drijkstra implements the Drijkstra shortest path graph algorithm.
// NOTE: An alternative would be to pre-compute trees for every node
func (sme *StateMutationEngine) drijkstra(gstart *mutationNode, gend *mutationNode) *mutationPath {
	dist := make(map[*mutationNode]uint32)
	prev := make(map[*mutationNode]*mutationEdge)
	queue := make(map[*mutationNode]*mutationNode)

	for _, n := range sme.nodes {
		dist[n] = ^uint32(0) - 1 // max uint32 - 1, a total hack
		prev[n] = nil
		queue[n] = n
	}

	dist[gend] = 0

	for len(queue) > 0 {
		min := ^uint32(0)
		var idx *mutationNode
		for k, v := range queue {
			if dist[v] < min {
				min = dist[v]
				idx = k
			}
		}
		u := queue[idx]

		if u == gstart {
			// found it!
			var chain []*mutationEdge
			i := gstart
			for prev[i] != nil {
				chain = append(chain, prev[i])
				i = prev[i].to
			}
			path := &mutationPath{
				gstart: gstart,
				gend:   gend,
				chain:  chain,
			}
			return path
		}

		delete(queue, idx)

		for _, v := range u.in {
			if _, ok := queue[v.from]; !ok { // v should be in queue
				continue
			}
			alt := dist[u] + v.cost
			if alt < dist[v.from] {
				dist[v.from] = alt
				prev[v.from] = v
			}
		}
	}
	return nil
}

// findPath finds the sequence of edges (if it exists) between two lib.Nodes
func (sme *StateMutationEngine) findPath(start lib.Node, end lib.Node) (path *mutationPath, e error) {
	gs, ge := sme.boundarySearch(start, end)
	if len(gs) < 1 {
		e = fmt.Errorf("could not find path: start not in graph")
	} else if len(gs) > 1 {
		e = fmt.Errorf("could not find path: ambiguous start")
	}
	if len(ge) < 1 {
		e = fmt.Errorf("could not find path: end not in graph")
		fmt.Printf("start: %v, end: %v\n", string(start.JSON()), string(end.JSON()))
		sme.DumpGraph()
	} else if len(ge) > 1 {
		e = fmt.Errorf("could not find path: ambiguous end")
		sme.DumpGraph()
		fmt.Printf("start: %v, end: %v\n", string(start.JSON()), string(end.JSON()))
		fmt.Printf("ends: %v\n", ge)
	}
	if e != nil {
		return
	}
	path = sme.drijkstra(gs[0], ge[0])
	path.start = start
	path.end = end
	path.cur = 0
	if path.chain == nil {
		e = fmt.Errorf("path not found: you can't get there from here")
		path = nil
	}
	return
}

// startNewMutation sees if we need a new mutation
// if we do, it starts it
func (sme *StateMutationEngine) startNewMutation(node string) {
	// we assume it's already been verified that this is *new*
	nid := NewNodeIDFromURL(node)
	start, e := sme.query.ReadDsc(nid)
	if e != nil {
		sme.Log(ERROR, e.Error())
		return
	} // this is bad...
	end, e := sme.query.Read(nid)
	if e != nil {
		sme.Log(ERROR, e.Error())
		return
	}
	p, e := sme.findPath(start, end)
	if e != nil {
		sme.Log(ERROR, e.Error())
		return
	}
	// new mutation, record it, and start it in motion
	sme.activeMutex.Lock()
	sme.active[node] = p
	sme.activeMutex.Unlock()
	sme.Logf(DEBUG, "started new mutation for %s (1/%d).", nid.String(), len(p.chain))
	if sme.mutationInContext(end, p.chain[p.cur].mut) {
		sme.Logf(DDEBUG, "firing mutation in context, timeout %s.", p.chain[p.cur].mut.Timeout().String())
		sme.emitMutation(end, start, p.chain[p.cur].mut)
		if p.chain[p.cur].mut.Timeout() != 0 {
			p.timer = time.AfterFunc(p.chain[p.cur].mut.Timeout(), func() { sme.emitFail(start, p) })
		}
	} else {
		sme.Log(DDEBUG, "mutation is not in our context.")
	}
}

func (sme *StateMutationEngine) emitFail(start lib.Node, p *mutationPath) {
	nid := p.start.ID()
	d := p.chain[p.cur].mut.FailTo()
	sme.Logf(INFO, "mutation timeout for %s, emitting: %s:%s:%s", nid.String(), d[0], d[1], d[2])
	url := lib.NodeURLJoin(nid.String(), d[1])
	dv := NewEvent(
		lib.Event_DISCOVERY,
		url,
		&DiscoveryEvent{
			Module:  d[0],
			URL:     url,
			ValueID: d[2],
		},
	)
	iv := NewEvent(
		lib.Event_STATE_MUTATION,
		url,
		&MutationEvent{
			Type:     pb.MutationControl_INTERRUPT,
			NodeCfg:  p.end,
			NodeDsc:  start,
			Mutation: sme.mutResolver[p.chain[p.cur].mut],
		},
	)
	sme.Emit([]lib.Event{dv, iv})
}

func (sme *StateMutationEngine) updateMutation(node string, url string, val reflect.Value) {
	sme.activeMutex.Lock()
	m, ok := sme.active[node]
	sme.activeMutex.Unlock()
	if !ok {
		// this shouldn't happen
		sme.Log(ERROR, "tried to call updateMutation, but no mutation exists")
		sme.startNewMutation(node)
		return
	}

	// stop any timer clocks
	if m.timer != nil {
		m.timer.Stop()
	}

	// we still query this to make sure it's the Dsc value
	var e error
	val, e = sme.query.GetValueDsc(lib.NodeURLJoin(node, url))
	if e != nil {
		sme.Log(ERROR, e.Error())
		return
	}

	// is this a value change we were expecting?
	cmuts := m.chain[m.cur].mut.Mutates()
	vs, match := cmuts[url]
	if !match {
		// we got an unexpected change!  Recalculating...
		sme.Logf(DEBUG, "node (%s) got an unexpected change of state (%s)\n", node, url)
		sme.activeMutex.Lock()
		delete(sme.active, node)
		sme.activeMutex.Unlock()
		sme.startNewMutation(node)
		return
	}
	// ok, we got an expected URL.  Is this the value we were looking for?
	if val.Interface() == vs[1].Interface() {
		// Ah!  Good, we're mutating as intended.
		m.cur++
		// are we done?
		if len(m.chain) <= m.cur {
			// all done!
			sme.Logf(DEBUG, "mutation chain completed (%d/%d)", m.cur, len(m.chain))
			sme.activeMutex.Lock()
			delete(sme.active, node)
			sme.activeMutex.Unlock()
			return
		}
		sme.Logf(DEBUG, "mutation is progressing as normal, moving to next (%d/%d)", m.cur, len(m.chain))
		// advance
		// TODO: there might be a more clever way that just updates the node we already have?
		n, e := sme.query.ReadDsc(NewNodeID(node))
		if e != nil {
			sme.Logf(ERROR, "couldn't query state of node in active mutation: %v", e)
			return
		}
		if sme.mutationInContext(m.end, m.chain[m.cur].mut) {
			sme.Logf(DDEBUG, "firing mutation in context, timeout %s.", m.chain[m.cur].mut.Timeout().String())
			sme.emitMutation(m.end, n, m.chain[m.cur].mut)
			if m.chain[m.cur].mut.Timeout() != 0 {
				m.timer = time.AfterFunc(m.chain[m.cur].mut.Timeout(), func() { sme.emitFail(n, m) })
			}
		}
	} else if val.Interface() == vs[0].Interface() { // might want to do more with this case later; for now we have to just recalculate
		sme.Logf(DEBUG, "mutation failed to progress, got %v, expected %v\n", val.Interface(), vs[1].Interface())
		sme.activeMutex.Lock()
		delete(sme.active, node)
		sme.activeMutex.Unlock()
		sme.startNewMutation(node)
	} else {
		sme.Logf(DEBUG, "unexpected mutation step, got %v, expected %v\n", val.Interface(), vs[1].Interface())
		// we got something completely unexpected... start over
		sme.activeMutex.Lock()
		delete(sme.active, node)
		sme.activeMutex.Unlock()
		sme.startNewMutation(node)
	}
}

func (sme *StateMutationEngine) mutationInContext(n lib.Node, m lib.StateMutation) (r bool) {
	switch m.Context() {
	case lib.StateMutationContext_SELF:
		if sme.self.Equal(n.ID()) {
			return true
		}
		break
	case lib.StateMutationContext_CHILD:
		if sme.self.Equal(n.ParentID()) {
			return true
		}
		break
	case lib.StateMutationContext_ALL:
		return true
	}
	return
}

func (sme *StateMutationEngine) handleEvent(v lib.Event) {
	sce := v.Data().(*StateChangeEvent)
	node, url := lib.NodeURLSplit(sce.URL)
	sme.activeMutex.Lock()
	_, ok := sme.active[node] // get the active mutation, if there is one
	sme.activeMutex.Unlock()
	switch sce.Type {
	case StateChange_CREATE:
		if ok {
			// what?! how do we have an active mutation for a node that was just created?
			// let's print something, and then pretend it *is* new
			sme.Log(DEBUG, "what?! we got a CREATE event for a node with an existing mutation")
			sme.activeMutex.Lock()
			delete(sme.active, node)
			sme.activeMutex.Unlock()
		}
		sme.startNewMutation(node)
		break
	case StateChange_DELETE:
		if ok {
			sme.activeMutex.Lock()
			delete(sme.active, node)
			sme.activeMutex.Unlock()
		}
		break
	case StateChange_UPDATE:
		if ok {
			// work on an active mutation?
			sme.updateMutation(node, url, sce.Value)
		} else {
			// new mutation?
			sme.startNewMutation(node)
		}
		break
	case StateChange_READ:
		//ignore; shouldn't be created anyway
	default:
	}
}

func (sme *StateMutationEngine) emitMutation(cfg lib.Node, dsc lib.Node, sm lib.StateMutation) {
	smee := &MutationEvent{
		Type:     MutationEvent_MUTATE,
		NodeCfg:  cfg,
		NodeDsc:  dsc,
		Mutation: sme.mutResolver[sm],
	}
	v := NewEvent(
		lib.Event_STATE_MUTATION,
		cfg.ID().String(),
		smee,
	)
	sme.EmitOne(v)
}

// It might be useful to export this
// Also, there's no particular reason it belongs here
// This takes the cfg state and merges only discoverable values from dsc state into it
func (sme *StateMutationEngine) dscNodeMeld(cfg, dsc lib.Node) (r lib.Node) {
	r = NewNodeFromMessage(cfg.Message().(*pb.Node)) // might be a bit expensive
	diff := []string{}
	for m := range Registry.Discoverables {
		for u := range Registry.Discoverables[m] {
			diff = append(diff, u)
		}
	}
	r.MergeDiff(dsc, diff)
	return
}

///////////////////////////
// Passthrough Interface /
/////////////////////////

/*
 * Consume Logger
 */
var _ lib.Logger = (*StateMutationEngine)(nil)

func (sme *StateMutationEngine) Log(level lib.LoggerLevel, m string) { sme.log.Log(level, m) }
func (sme *StateMutationEngine) Logf(level lib.LoggerLevel, fmt string, v ...interface{}) {
	sme.log.Logf(level, fmt, v...)
}
func (sme *StateMutationEngine) SetModule(name string)                { sme.log.SetModule(name) }
func (sme *StateMutationEngine) GetModule() string                    { return sme.log.GetModule() }
func (sme *StateMutationEngine) SetLoggerLevel(level lib.LoggerLevel) { sme.log.SetLoggerLevel(level) }
func (sme *StateMutationEngine) GetLoggerLevel() lib.LoggerLevel      { return sme.log.GetLoggerLevel() }
func (sme *StateMutationEngine) IsEnabledFor(level lib.LoggerLevel) bool {
	return sme.log.IsEnabledFor(level)
}

/*
 * Consume an emitter, so we implement EventEmitter directly
 */

func (sme *StateMutationEngine) Subscribe(id string, c chan<- []lib.Event) error {
	return sme.em.Subscribe(id, c)
}
func (sme *StateMutationEngine) Unsubscribe(id string) error { return sme.em.Unsubscribe(id) }
func (sme *StateMutationEngine) Emit(v []lib.Event)          { sme.em.Emit(v) }
func (sme *StateMutationEngine) EmitOne(v lib.Event)         { sme.em.EmitOne(v) }
func (sme *StateMutationEngine) EventType() lib.EventType    { return sme.em.EventType() }
