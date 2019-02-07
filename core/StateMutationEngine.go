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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	gv "github.com/awalterschulze/gographviz"
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

var MutationEventString = map[pb.MutationControl_Type]string{
	MutationEvent_MUTATE:    "MUTATE",
	MutationEvent_INTERRUPT: "INTERRUPT",
}

type MutationEvent struct {
	Type pb.MutationControl_Type
	// strictly speaking, we may only need the Cfg
	// but we generally have this info on hand anyway
	NodeCfg  lib.Node
	NodeDsc  lib.Node
	Mutation [2]string // [0] = module, [1] = mutid
}

func (me *MutationEvent) String() string {
	return fmt.Sprintf("(%s) %s : %s -> %s", MutationEventString[me.Type], me.NodeCfg.ID().String(), me.Mutation[0], me.Mutation[1])
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
	mutex *sync.Mutex
	cur   int // where are we currently?
	// curSeen is a slice of URLs that we've seen (correct) changes in the current mut
	// 	This is important to keep track of muts that change more than one URL
	curSeen []string
	start   lib.Node
	end     lib.Node
	gstart  *mutationNode
	gend    *mutationNode
	chain   []*mutationEdge
	timer   *time.Timer
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
	graphMutex  *sync.RWMutex
	nodes       []*mutationNode // so we can search for matches
	edges       []*mutationEdge
	em          *EventEmitter
	qc          chan lib.Query
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
	freeze      bool
}

// NewStateMutationEngine creates an initialized StateMutationEngine
func NewStateMutationEngine(ctx Context, qc chan lib.Query) *StateMutationEngine {
	sme := &StateMutationEngine{
		muts:        []lib.StateMutation{},
		mutResolver: make(map[lib.StateMutation][2]string),
		active:      make(map[string]*mutationPath),
		activeMutex: &sync.Mutex{},
		mutators:    make(map[string]uint32),
		requires:    make(map[string]uint32),
		graph:       &mutationNode{spec: ctx.SME.RootSpec},
		graphMutex:  &sync.RWMutex{},
		nodes:       []*mutationNode{},
		edges:       []*mutationEdge{},
		em:          NewEventEmitter(lib.Event_STATE_MUTATION),
		qc:          qc,
		run:         false,
		echan:       make(chan lib.Event),
		query:       &ctx.Query,
		schan:       ctx.SubChan,
		log:         &ctx.Logger,
		self:        ctx.Self,
		root:        ctx.SME.RootSpec,
		freeze:      true,
	}
	sme.log.SetModule("StateMutationEngine")
	return sme
}

// RegisterMutation injects new mutaitons into the SME. muts[i] should match callback[i]
// We take a list so that we only call onUpdate once
// LOCKS: graphMutex (RW)
func (sme *StateMutationEngine) RegisterMutation(module, id string, mut lib.StateMutation) (e error) {
	sme.graphMutex.Lock()
	sme.muts = append(sme.muts, mut)
	sme.mutResolver[mut] = [2]string{module, id}
	sme.graphMutex.Unlock()
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

func (sme *StateMutationEngine) dumpMapOfValues(m map[string]reflect.Value) (s string) {
	for k := range m {
		s += fmt.Sprintf("%s: %s, ", k, lib.ValueToString(m[k]))
	}
	return
}

func (sme *StateMutationEngine) dumpMutMap(m map[string][2]reflect.Value) (s string) {
	for k := range m {
		s += fmt.Sprintf("%s: %s -> %s, ", k, lib.ValueToString(m[k][0]), lib.ValueToString(m[k][1]))
	}
	return
}

// DumpGraph FIXME: REMOVE -- for debugging
// LOCKS: graphMutex (R)
func (sme *StateMutationEngine) DumpGraph() {
	sme.graphMutex.RLock()
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
		  req: %s
		  exc: %s
		 In: %v
		 Out: %v
		 `, m, m.spec, sme.dumpMapOfValues(m.spec.Requires()), sme.dumpMapOfValues(m.spec.Excludes()), m.in, m.out)
	}
	fmt.Printf("\n=== END: Node list ===\n")
	fmt.Printf("\n=== START: Edge list ===\n")
	for _, m := range sme.edges {
		fmt.Printf(`
		Edge: %p
		 Mutation: %p
		  mut: %s
		  req: %s
		  exc: %s
		 From: %p
		 To: %p
		`, m, m.mut, sme.dumpMutMap(m.mut.Mutates()), sme.dumpMapOfValues(m.mut.Requires()), sme.dumpMapOfValues(m.mut.Excludes()), m.from, m.to)
	}
	fmt.Printf("\n=== END: Edge list ===\n")
	sme.graphMutex.RUnlock()
}

func MutationNodesToProto(nodes []*mutationNode) (r pb.MutationNodeList) {
	for _, mn := range nodes {
		var nmn pb.MutationNode
		nmn.Id = fmt.Sprintf("%p", mn)
		reqsMap := mn.spec.Requires()
		excsMap := mn.spec.Excludes()
		for k := range reqsMap {
			newPair := &pb.Pair{
				Key:   k,
				Value: lib.ValueToString(reqsMap[k]),
			}
			nmn.Reqs = append(nmn.Reqs, newPair)
		}
		for k := range excsMap {
			newPair := &pb.Pair{
				Key:   k,
				Value: lib.ValueToString(excsMap[k]),
			}
			nmn.Excs = append(nmn.Excs, newPair)
		}
		for _, me := range mn.in {
			newEdge := &pb.MutationEdge{
				Cost: me.cost,
				From: fmt.Sprintf("%p", me.from),
				To:   fmt.Sprintf("%p", me.to),
			}
			nmn.In = append(nmn.In, newEdge)
		}
		for _, me := range mn.out {
			newEdge := &pb.MutationEdge{
				Cost: me.cost,
				From: fmt.Sprintf("%p", me.from),
				To:   fmt.Sprintf("%p", me.to),
			}
			nmn.Out = append(nmn.Out, newEdge)
		}
		r.MutationNodeList = append(r.MutationNodeList, &nmn)
	}
	return
}

func MutationEdgesToProto(edges []*mutationEdge) (r pb.MutationEdgeList) {
	for _, me := range edges {
		var nme pb.MutationEdge
		nme.Cost = me.cost
		nme.From = fmt.Sprintf("%p", me.from)
		nme.To = fmt.Sprintf("%p", me.to)

		r.MutationEdgeList = append(r.MutationEdgeList, &nme)
	}
	return
}

func MutationPathToProto(path *mutationPath) (r pb.MutationPath, e error) {
	if path != nil {
		r.Cur = int64(path.cur)
		for _, me := range path.chain {
			var nme pb.MutationEdge
			nme.Cost = me.cost
			nme.From = fmt.Sprintf("%p", me.from)
			nme.To = fmt.Sprintf("%p", me.to)

			r.Chain = append(r.Chain, &nme)
		}
	} else {
		e = fmt.Errorf("Mutation path is nil")
	}

	return
}

func (sme *StateMutationEngine) filterMutationNodesForNode(n NodeID) (r []*mutationNode, e error) {
	// Get node from path
	sme.Logf(lib.LLDEBUG, "got to filtered nodes method")
	mp := sme.active[n.String()]
	if mp != nil {
		jmp, _ := json.Marshal(mp)
		sme.Logf(lib.LLDEBUG, string(jmp))
		node := mp.end
		plat, e := node.GetValue("/Platform")
		sme.Logf(lib.LLDEBUG, "plat: %v err: %v", plat, e)

		sme.Logf(lib.LLDEBUG, "Mutators: %v", sme.mutators)
		sme.Logf(lib.LLDEBUG, "Discoverables: %v", Registry.Discoverables)
		for _, mn := range sme.nodes {
			sme.Logf(lib.LLDEBUG, "node reqs: %v", mn.spec.Requires())
		}

	} else {
		e = fmt.Errorf("Can't get node info because mutation path is nil")
	}

	return
}

//GenDotString returns a DOT formatted string of the mutation graph
func (sme *StateMutationEngine) GenDotString() string {
	g := gv.NewGraph()
	g.SetName("MutGraph")
	g.SetDir(true) //indicates that the graph is directed
	for _, e := range sme.edges {
		if g.IsNode(fmt.Sprintf("%p", e.to)) == false {
			var attributes map[string]string
			attributes = make(map[string]string)
			g.AddNode("MutGraph", fmt.Sprintf("%p", e.to), attributes)
		}

		if g.IsNode(fmt.Sprintf("%p", e.from)) == false {
			var attributes map[string]string
			attributes = make(map[string]string)
			g.AddNode("MutGraph", fmt.Sprintf("%p", e.from), attributes)
		}

		var attributes map[string]string
		g.AddEdge(fmt.Sprintf("%p", e.from), fmt.Sprintf("%p", e.to), true, attributes)
	}
	return g.String()
}

// GetDotGraph returns the dot graph
func (sme *StateMutationEngine) GetDotGraph(n lib.Node) (r string, e error) {
	graphAst, _ := gv.ParseString(`graph G {}`)
	graph := gv.NewGraph()
	if err := gv.Analyse(graphAst, graph); err != nil {
		return "", err
	}

	platform, _ := n.GetValue("/Platform")
	arch, _ := n.GetValue("/Arch")

	platformString := lib.ValueToString(platform)
	archString := lib.ValueToString(arch)

	var nodes []string

	// Add nodes to graph
	for _, m := range sme.nodes {
		label := ""
		rm := m.spec.Requires()
		if lib.ValueToString(rm["/Platform"]) == platformString && lib.ValueToString(rm["/Arch"]) == archString {
			if rm["/PhysState"].IsValid() {
				label += lib.ValueToString(rm["/PhysState"])
			}
			if rm["/RunState"].IsValid() {
				label += lib.ValueToString(rm["/RunState"])
			}
			if rm["type.googleapis.com/proto.RPi3/Pxe"].IsValid() {
				label += lib.ValueToString(rm["type.googleapis.com/proto.RPi3/Pxe"])
			}
		} else if !rm["/Platform"].IsValid() && !rm["/Platform"].IsValid() {
			if rm["/PhysState"].IsValid() {
				label += lib.ValueToString(rm["/PhysState"])
			}
		}
		label = strings.Replace(label, ",", "", -1)
		label = strings.Replace(label, " ", "", -1)
		if label == "" {
			graph.AddNode("G", fmt.Sprintf("%p", m), nil)
		} else {
			graph.AddNode("G", fmt.Sprintf("%p", m), map[string]string{"label": label})
		}
		nodes = append(nodes, fmt.Sprintf("%p", m))
	}

	// Add edges to graph
	for _, m := range sme.edges {
		from := fmt.Sprintf("%p", m.from)
		to := fmt.Sprintf("%p", m.to)

		for _, n := range nodes {
			if from == n {
				graph.AddEdge(from, to, true, nil)
			}
		}
	}

	output := graph.String()
	return output, nil
}

// PathExists returns a boolean indicating whether or not a path exists in the graph between two nodes.
// If the path doesn't exist, it also returns the error.
// LOCKS: graphMutex (R) via findPath
func (sme *StateMutationEngine) PathExists(start lib.Node, end lib.Node) (r bool, e error) {
	p, e := sme.findPath(start, end)
	if p != nil {
		r = true
	}
	return
}

// goroutine
func (sme *StateMutationEngine) sendQueryResponse(qr lib.QueryResponse, r chan<- lib.QueryResponse) {
	r <- qr
}

// QueryChan returns a chanel that Queries can be sent on
func (sme *StateMutationEngine) QueryChan() chan<- lib.Query {
	return sme.qc
}

// Run is a goroutine that listens for state changes and performs StateMutation magic
// LOCKS: all
func (sme *StateMutationEngine) Run() {
	// on run we import all mutations in the registry
	sme.graphMutex.Lock()
	for mod := range Registry.Mutations {
		for id, mut := range Registry.Mutations[mod] {
			sme.muts = append(sme.muts, mut)
			sme.mutResolver[mut] = [2]string{mod, id}
		}
	}
	sme.graphMutex.Unlock()
	sme.onUpdate()
	if sme.GetLoggerLevel() >= DDEBUG {
		sme.DumpGraph() // Use this to debug your graph
	}

	// create a listener for state change events we care about
	sme.selist = NewEventListener(
		"StateMutationEngine",
		lib.Event_STATE_CHANGE,
		func(v lib.Event) bool {
			_, url := lib.NodeURLSplit(v.URL())
			sme.graphMutex.RLock()
			defer sme.graphMutex.RUnlock()
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
		case q := <-sme.qc:
			switch q.Type() {
			case lib.Query_READDOT:
				var v string
				var e error
				v = sme.GenDotString()
				go sme.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			case lib.Query_MUTATIONNODES:
				_, u := lib.NodeURLSplit(q.URL())
				var e error
				if u == "" {
					v := MutationNodesToProto(sme.nodes)
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				} else {
					sme.Logf(lib.LLDEBUG, "getting filtered nodes")
					n := NewNodeIDFromURL(q.URL())
					r, e := sme.filterMutationNodesForNode(*n)
					v := MutationNodesToProto(r)
					sme.Logf(lib.LLDEBUG, "filtered nodes: %v", v)
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				}
				break
			case lib.Query_MUTATIONEDGES:
				n, u := lib.NodeURLSplit(q.URL())
				sme.Logf(lib.LLDEBUG, "node: %v url: %v", n, u)
				var e error
				if u == "" {
					v := MutationEdgesToProto(sme.edges)
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				} else {

				}
				break
			case lib.Query_MUTATIONPATH:
				n, _ := lib.NodeURLSplit(q.URL())
				var e error
				v, e := MutationPathToProto(sme.active[n])
				go sme.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				break
			default:
				sme.Logf(lib.LLDEBUG, "unsupported query type: %d", q.Type())
			}
			break
		case v := <-sme.echan:
			// FIXME: event processing can be expensive;
			// we should make them concurrent with a queue
			if !sme.Frozen() {
				sme.handleEvent(v)
			}
			break
		case <-debugchan:
			sme.Logf(lib.LLDDEBUG, "There are %d active mutations.", len(sme.active))
			break
		}
	}
}

func (sme *StateMutationEngine) Frozen() bool {
	sme.activeMutex.Lock()
	defer sme.activeMutex.Unlock()
	return sme.freeze
}

func (sme *StateMutationEngine) Freeze() {
	sme.Log(INFO, "freezing")
	sme.activeMutex.Lock()
	sme.freeze = true
	sme.activeMutex.Unlock()
}

func (sme *StateMutationEngine) Thaw() {
	sme.Log(INFO, "thawing")
	sme.activeMutex.Lock()
	sme.active = make(map[string]*mutationPath)
	sme.freeze = false
	sme.activeMutex.Unlock()
	ns, _ := sme.query.ReadAll()
	for _, n := range ns {
		sme.startNewMutation(n.ID().String())
	}
}

////////////////////////
// Unexported methods /
//////////////////////

// !!!IMPORTANT!!!
// collectURLs assumes you already hold a lock
// currently only used in onUpdate
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
// !!!IMPORTANT!!!
// buildGraph assumes you already hold a lock
// currently only used in onUpdate
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

// !!!IMPORTANT!!!
// clearGraph assumes you already hold a lock
// currently only used in onUpdate
func (sme *StateMutationEngine) clearGraph() {
	sme.mutators = make(map[string]uint32)
	sme.requires = make(map[string]uint32)
	sme.graph.in = []*mutationEdge{}
	sme.graph.out = []*mutationEdge{}
	sme.graph.spec = sme.root
}

// onUpdate should get called any time a new mutation is registered
// onUpdate gets a graphMutex around everything, so it's important that it doesn't
// call anything that tries to get it's own lock, or it will deadlock
// LOCKS: graphMutex (RW)
// FIXME: We should re-compute active mutations?
func (sme *StateMutationEngine) onUpdate() {
	sme.graphMutex.Lock()
	sme.clearGraph()
	sme.collectURLs()
	sme.nodes, sme.edges = sme.buildGraph(sme.graph, make(map[lib.StateSpec]*mutationNode), make(map[int]*mutationNode), []*mutationNode{})
	sme.Logf(DEBUG, "Built graph [ Mutations: %d Mutation URLs: %d Requires URLs: %d Graph Nodes: %d Graph Edges: %d ]",
		len(sme.muts), len(sme.mutators), len(sme.requires), len(sme.nodes), len(sme.edges))
	sme.graphMutex.Unlock()
}

// LOCKS: graphMutex (R)
func (sme *StateMutationEngine) nodeSearch(node lib.Node) (mns []*mutationNode) {
	sme.graphMutex.RLock()
	defer sme.graphMutex.RUnlock()
	for _, n := range sme.nodes {
		if n.spec.NodeMatch(node) {
			mns = append(mns, n)
		}
	}
	return
}

// LOCKS: graphMutex (R)
func (sme *StateMutationEngine) boundarySearch(start lib.Node, end lib.Node) (gstart []*mutationNode, gend []*mutationNode) {
	startMerge := sme.dscNodeMeld(end, start)
	sme.graphMutex.RLock()
	for _, n := range sme.nodes {
		// in general, we don't want the graph root as an option
		if n != sme.graph && n.spec.NodeMatchWithMutators(startMerge, sme.mutators) {
			gstart = append(gstart, n)
		}
		if n != sme.graph && n.spec.NodeCompatWithMutators(end, sme.mutators) { // ends can be more lenient
			gend = append(gend, n)
		}
	}
	sme.graphMutex.RUnlock()
	// there's one exception: we may be starting on the graph root (if nothing else matched)
	if len(gstart) == 0 {
		gstart = append(gstart, sme.graph)
	}
	return
}

// drijkstra implements the Drijkstra shortest path graph algorithm.
// NOTE: An alternative would be to pre-compute trees for every node
// LOCKS: graphMutex (R)
func (sme *StateMutationEngine) drijkstra(gstart *mutationNode, gend []*mutationNode) *mutationPath {
	sme.graphMutex.RLock()
	defer sme.graphMutex.RUnlock()
	isEnd := func(i *mutationNode) (r bool) {
		for _, j := range gend {
			if i == j {
				return true
			}
		}
		return
	}

	dist := make(map[*mutationNode]uint32)
	prev := make(map[*mutationNode]*mutationEdge)
	queue := make(map[*mutationNode]*mutationNode)

	for _, n := range sme.nodes {
		dist[n] = ^uint32(0) - 1 // max uint32 - 1, a total hack
		prev[n] = nil
		queue[n] = n
	}

	dist[gstart] = 0

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

		if isEnd(u) {
			// found it!
			var chain []*mutationEdge
			i := u
			for prev[i] != nil {
				chain = append([]*mutationEdge{prev[i]}, chain...)
				i = prev[i].from
			}
			path := &mutationPath{
				mutex:   &sync.Mutex{},
				gstart:  gstart,
				gend:    u,
				chain:   chain,
				curSeen: []string{},
			}
			return path
		}

		delete(queue, idx)

		for _, v := range u.out {
			if _, ok := queue[v.to]; !ok { // v should be in queue
				continue
			}
			alt := dist[u] + v.cost
			if alt < dist[v.to] {
				dist[v.to] = alt
				prev[v.to] = v
			}
		}
	}
	return nil
}

// findPath finds the sequence of edges (if it exists) between two lib.Nodes
// LOCKS: graphMutex (R) via boundarySearch, drijkstra
func (sme *StateMutationEngine) findPath(start lib.Node, end lib.Node) (path *mutationPath, e error) {
	same := true
	for m := range sme.mutators {
		sv, _ := start.GetValue(m)
		ev, _ := end.GetValue(m)
		if sv.Interface() != ev.Interface() {
			same = false
			break
		}
	}
	if same {
		path = &mutationPath{
			mutex:   &sync.Mutex{},
			start:   start,
			end:     end,
			cur:     0,
			curSeen: []string{},
			chain:   []*mutationEdge{},
		}
		return
	}
	gs, ge := sme.boundarySearch(start, end)
	if len(gs) < 1 {
		e = fmt.Errorf("could not find path: start not in graph")
	} else if len(gs) > 1 {
		e = fmt.Errorf("could not find path: ambiguous start")
	}
	if len(ge) < 1 {
		e = fmt.Errorf("could not find path: end not in graph")
		if sme.GetLoggerLevel() >= DDEBUG {
			fmt.Printf("start: %v, end: %v\n", string(start.JSON()), string(end.JSON()))
			sme.DumpGraph()
		}
	}
	if e != nil {
		return
	}
	// If start is contained in end, we're already where we want to be
	// In this case, we return a valid mutationPath with a zero length chain
	/* this should be caught by the test above already
	for _, gend := range ge {
		if gend == gs[0] {
			path = &mutationPath{
				mutex:   &sync.Mutex{},
				start:   start,
				end:     end,
				cur:     0,
				curSeen: []string{},
				chain:   []*mutationEdge{},
			}
			return
		}
	}
	*/
	path = sme.drijkstra(gs[0], ge) // we require a unique start, but not a unique end
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
// if we don't already have a mutation object, it creates it
// LOCKS: graphMutex (R) via findPath; activeMutex; path.mutex
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
	if len(p.chain) == 0 { // we're already there
		sme.Logf(DEBUG, "%s discovered that we're already where we want to be", nid.String())
		return
	}
	// new mutation, record it, and start it in motion

	// we need to hold the path mutex for the rest of this function
	p.mutex.Lock()
	defer p.mutex.Unlock()

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

// LOCKS: graphMutex (R); path.mutex
func (sme *StateMutationEngine) emitFail(start lib.Node, p *mutationPath) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	nid := p.start.ID()
	d := p.chain[p.cur].mut.FailTo()
	sme.Logf(INFO, "mutation timeout for %s, emitting: %s:%s:%s", nid.String(), d[0], d[1], d[2])

	// reset all mutators to zero, except the failure mutator
	// FIXME: setting things without discovery isn't very polite
	node, _ := sme.query.ReadDsc(nid)
	sme.graphMutex.RLock()
	for m := range sme.mutators {
		if m == d[1] {
			continue
		}
		v, _ := node.GetValue(m)
		node.SetValue(m, reflect.Zero(v.Type()))
	}
	sme.graphMutex.RUnlock()
	sme.query.UpdateDsc(node)

	// now send a discover to whatever failed state
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

	// send a mutation interrupt
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

// advanceMutation fires off the next mutation in the chain
// does *not* check to make sure there is one
// assumes m.mutex is locked by surrounding func
func (sme *StateMutationEngine) advanceMutation(node string, m *mutationPath) {
	nid := NewNodeIDFromURL(node)
	m.cur++
	m.curSeen = []string{}
	sme.Logf(DEBUG, "resuming mutation for %s (%d/%d).", nid.String(), m.cur+1, len(m.chain))
	if sme.mutationInContext(m.end, m.chain[m.cur].mut) {
		sme.Logf(DDEBUG, "firing mutation in context, timeout %s.", m.chain[m.cur].mut.Timeout().String())
		sme.emitMutation(m.end, m.start, m.chain[m.cur].mut)
		if m.chain[m.cur].mut.Timeout() != 0 {
			m.timer = time.AfterFunc(m.chain[m.cur].mut.Timeout(), func() { sme.emitFail(m.start, m) })
		}
	} else {
		sme.Logf(DDEBUG, "node (%s) mutation is not in our context", node)
	}
}

// handleUnexpected deals with unexpected events in updateMutaion
// Logic:
// 1) did we regress to a previous point in the chain?  devolve.
// 2) no? can we find a direct path?
// 3) no? give up, declare everything (except the unexpected discovery) unknown
// LOCKS: !!! this does *not* lock the mutationPath, it assumes it is already locked by the calling function
// (generally updateMutation)
func (sme *StateMutationEngine) handleUnexpected(node, url string, val reflect.Value) {
	sme.activeMutex.Lock()
	m, ok := sme.active[node]
	sme.activeMutex.Unlock()
	if !ok {
		// there's no existing mutation chain
		// shouldn't really happen
		sme.startNewMutation(node)
		return
	}

	rewind := make(map[string]reflect.Value)
	// starting from the current position, look backwards in the chain
	// have we seen this value before?  Maybe we need to reset to that point...
	found := false
	var i int
	for i = m.cur; i >= 0; i-- {
		// is there a mutation with this url?
		for murl, mvs := range m.chain[i].mut.Mutates() {
			if murl == url {
				// this mutation deals with the url of interest
				if mvs[0].Interface() == val.Interface() {
					// this is our rewind point, but we need the rest of this loop
					found = true
				}
			} else {
				// add to our rewind
				rewind[url] = mvs[0]
			}
		}
		if found {
			break
		}
	}

	// this is a bit bad.  We don't want to get our own state changes, so we change the node directly
	nid := NewNodeIDFromURL(node)
	n, e := sme.query.ReadDsc(nid)
	if e != nil {
		// ok, I give up.  The node has just disappeared.
		sme.Logf(ERROR, "%s node unexpectly disappeared", node)
		return
	}

	// this is a devolution
	if found {
		// ok, let's devolve
		sme.Logf(DEBUG, "%s is devolving back %d steps due to an unexpected regression", node, m.cur-i)

		n.SetValues(rewind)
		m.chain = append(m.chain[:m.cur], m.chain[i-1:]...)
		sme.advanceMutation(node, m)
		return
	}

	// not a devolution, can we find a path?
	end, e := sme.query.Read(nid)
	if e != nil {
		sme.Log(ERROR, e.Error())
		return
	}
	p, e := sme.findPath(n, end)
	if e == nil {
		if len(p.chain) == 0 { // we're already there
			sme.Logf(DEBUG, "%s discovered that we're already where we want to be", nid.String())
			return
		}
		// update the chain & increment
		sme.Logf(DEBUG, "%s found a new path", node)
		m.chain = append(m.chain[:m.cur], p.chain...)
		sme.advanceMutation(node, m)
		return
	}

	sme.Logf(DEBUG, "%s could neither find a path, nor devolve.  We're lost.", node)

	sme.graphMutex.RLock()
	defer sme.graphMutex.RUnlock()
	// set everything to unknown except the value we were given
	for u := range sme.mutators {
		if u == url {
			// don't reset the unexpected value...
			continue
		}
		v, _ := n.GetValue(u)
		n.SetValue(u, reflect.Zero(v.Type()))
	}
}

// updateMutation attempts to progress along an existing mutation chain
// LOCKS: activeMutex; path.mutex; graphMutex (R) via startNewMutation
func (sme *StateMutationEngine) updateMutation(node string, url string, val reflect.Value) {
	sme.activeMutex.Lock()
	m, ok := sme.active[node]
	sme.activeMutex.Unlock()
	if !ok {
		// this shouldn't happen
		sme.Logf(DDEBUG, "call to updateMutation, but no mutation exists %s", node)
		sme.startNewMutation(node)
		return
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()

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

	// this is a discovery on a completed chain
	if m.cur >= len(m.chain) {
		sme.Logf(DEBUG, "node (%s) got a discovery on a completed chain (%s)", node, url)
		sme.handleUnexpected(node, url, val)
		return
	}

	// is this a value change we were expecting?
	cmuts := m.chain[m.cur].mut.Mutates()
	vs, match := cmuts[url]
	if !match {
		// we got an unexpected change!  Recalculating...
		sme.Logf(DEBUG, "node (%s) got an unexpected change of state (%s)", node, url)
		sme.handleUnexpected(node, url, val)
		return
	}

	// ok, we got an expected URL.  Is this the value we were looking for?
	if val.Interface() == vs[1].Interface() {
		// Ah!  Good, we're mutating as intended.
		m.curSeen = append(m.curSeen, url)
		// Ok, everything checks out, but maybe we have more things to discover before progressing?
		// TODO: more efficient way to do this for large numbers of URL changes/mut?
		for url := range cmuts {
			got := false
			for _, seen := range m.curSeen {
				if url == seen { // ok, we got this one
					got = true
					break
				}
			}
			if !got {
				// ok, we haven't seen all of the URL's discovered
				sme.Logf(DEBUG, "mutation chain for %s progressing as normal, but this mutation isn't complete yet. Still need: %s", node, url)
				return
			}
		}
		m.curSeen = []string{} // possibly redundant
		m.timer.Stop()
		// are we done?
		if len(m.chain) == m.cur+1 {
			// all done!
			sme.Logf(DEBUG, "mutation chain completed for %s (%d/%d)", node, m.cur+1, len(m.chain))
			return
		}
		sme.Logf(DEBUG, "mutation for %s progressing as normal, moving to next (%d/%d)", node, m.cur+1, len(m.chain))
		// advance
		sme.advanceMutation(node, m)
	} else if val.Interface() == vs[0].Interface() { // might want to do more with this case later; for now we have to just recalculate
		sme.Logf(DEBUG, "mutation for %s failed to progress, got %v, expected %v\n", node, val.Interface(), vs[1].Interface())
		sme.handleUnexpected(node, url, val)
	} else {
		sme.Logf(DEBUG, "unexpected mutation step for %s, got %v, expected %v\n", node, val.Interface(), vs[1].Interface())
		// we got something completely unexpected... start over
		sme.handleUnexpected(node, url, val)
	}
}

// Assumes you already hold a lock
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
			m := sme.active[node]
			m.mutex.Lock()
			delete(sme.active, node)
			m.mutex.Unlock()
			sme.activeMutex.Unlock()
		}
		break
	case StateChange_UPDATE:
		sme.updateMutation(node, url, sce.Value)
		break
	case StateChange_READ:
		//ignore; shouldn't be created anyway
	default:
	}
}

// LOCKS: graphMutex (R)
func (sme *StateMutationEngine) emitMutation(cfg lib.Node, dsc lib.Node, sm lib.StateMutation) {
	sme.graphMutex.RLock()
	smee := &MutationEvent{
		Type:     MutationEvent_MUTATE,
		NodeCfg:  cfg,
		NodeDsc:  dsc,
		Mutation: sme.mutResolver[sm],
	}
	sme.graphMutex.RUnlock()
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
