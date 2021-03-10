/* StateMutationEngine.go: In many ways, the heart of Kraken, this engine manages state mutations.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	pb "github.com/hpc/kraken/core/proto"
	ct "github.com/hpc/kraken/core/proto/customtypes"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
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
	NodeCfg  types.Node
	NodeDsc  types.Node
	Mutation [2]string // [0] = module, [1] = mutid
}

func (me *MutationEvent) String() string {
	return fmt.Sprintf("(%s) %s : %s -> %s", MutationEventString[me.Type], me.NodeCfg.ID().String(), me.Mutation[0], me.Mutation[1])
}

type mutationEdge struct {
	cost uint32
	mut  types.StateMutation
	from *mutationNode
	to   *mutationNode
}

// consider equal if mut is the same (pointer), and to and from are the same
func (m *mutationEdge) Equal(b *mutationEdge) bool {
	if m.to != b.to {
		return false
	}
	if m.from != b.from {
		return false
	}
	if m.mut != b.mut {
		return false
	}
	return true
}

type mutationNode struct {
	spec types.StateSpec // spec with aggregated require/excludes
	in   []*mutationEdge
	out  []*mutationEdge
}

type mutationPath struct {
	mutex *sync.Mutex
	cur   int // where are we currently?
	cmplt bool
	// curSeen is a slice of URLs that we've seen (correct) changes in the current mut
	// 	This is important to keep track of muts that change more than one URL
	curSeen    []string
	start      types.Node
	end        types.Node
	gstart     *mutationNode
	gend       *mutationNode
	chain      []*mutationEdge
	timer      *time.Timer
	waitingFor string // the SI we're currently waiting for
}

// DefaultRootSpec provides a sensible root StateSpec to build the mutation graph off of
func DefaultRootSpec() types.StateSpec {
	return NewStateSpec(map[string]reflect.Value{"/PhysState": reflect.ValueOf(pb.Node_PHYS_UNKNOWN)}, map[string]reflect.Value{})
}

////////////////////////////////
// StateMutationEngine Object /
//////////////////////////////

var _ types.StateMutationEngine = (*StateMutationEngine)(nil)

// A StateMutationEngine listens for state change events and manages mutations to evolve Dsc state into Cfg state
type StateMutationEngine struct {
	muts        []types.StateMutation
	mutResolver map[types.StateMutation][2]string // this allows us to lookup module/id pair from mutation
	// stuff we can compute from muts
	mutators    map[string]uint32 // ref count, all URLs that mutate
	requires    map[string]uint32 // ref count, referenced (req/exc) urls that don't mutate
	graph       *mutationNode     // graph start
	graphMutex  *sync.RWMutex
	nodes       []*mutationNode // so we can search for matches
	edges       []*mutationEdge
	em          *EventEmitter
	qc          chan types.Query
	schan       chan<- types.EventListener // subscription channel
	echan       chan types.Event
	sichan      chan types.Event
	selist      *EventListener
	silist      *EventListener
	run         bool                     // are we running?
	active      map[string]*mutationPath // active mutations
	waiting     map[string][]*mutationPath
	activeMutex *sync.Mutex // active (and waiting) needs some synchronization, or we can get in bad places
	query       *QueryEngine
	log         types.Logger
	self        types.NodeID
	root        types.StateSpec
	freeze      bool
}

// NewStateMutationEngine creates an initialized StateMutationEngine
func NewStateMutationEngine(ctx Context, qc chan types.Query) *StateMutationEngine {
	sme := &StateMutationEngine{
		muts:        []types.StateMutation{},
		mutResolver: make(map[types.StateMutation][2]string),
		active:      make(map[string]*mutationPath),
		waiting:     make(map[string][]*mutationPath),
		activeMutex: &sync.Mutex{},
		mutators:    make(map[string]uint32),
		requires:    make(map[string]uint32),
		graph:       &mutationNode{spec: ctx.SME.RootSpec},
		graphMutex:  &sync.RWMutex{},
		nodes:       []*mutationNode{},
		edges:       []*mutationEdge{},
		em:          NewEventEmitter(types.Event_STATE_MUTATION),
		qc:          qc,
		run:         false,
		echan:       make(chan types.Event),
		sichan:      make(chan types.Event),
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
func (sme *StateMutationEngine) RegisterMutation(si, id string, mut types.StateMutation) (e error) {
	sme.graphMutex.Lock()
	sme.muts = append(sme.muts, mut)
	sme.mutResolver[mut] = [2]string{si, id}
	sme.graphMutex.Unlock()
	sme.onUpdate()
	return
}

// NodeMatch determines how many compatable StateSpecs this node has in the graph
func (sme *StateMutationEngine) NodeMatch(node types.Node) (i int) {
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
		s += fmt.Sprintf("%s: %s, ", k, util.ValueToString(m[k]))
	}
	return
}

func (sme *StateMutationEngine) dumpMutMap(m map[string][2]reflect.Value) (s string) {
	for k := range m {
		s += fmt.Sprintf("%s: %s -> %s, ", k, util.ValueToString(m[k][0]), util.ValueToString(m[k][1]))
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

// DumpJSONGraph for debugging the graph
// !!!IMPORTANT!!!
// DumpJSONGraph assumes you already hold a lock
func (sme *StateMutationEngine) DumpJSONGraph(nodes []*mutationNode, edges []*mutationEdge) {
	nl := mutationNodesToProto(nodes)
	el := mutationEdgesToProto(edges)

	graph := struct {
		Nodes []*pb.MutationNode `json:"nodes"`
		Edges []*pb.MutationEdge `json:"edges"`
	}{
		Nodes: nl.MutationNodeList,
		Edges: el.MutationEdgeList,
	}

	jsonGraph, e := json.Marshal(graph)
	if e != nil {
		fmt.Printf("error getting json graph\n")
		return
	}
	fmt.Printf("JSON Graph: \n%v\n", string(jsonGraph))
}

// Converts a slice of sme mutation nodes to a protobuf MutationNodeList
func mutationNodesToProto(nodes []*mutationNode) (r pb.MutationNodeList) {
	for _, mn := range nodes {
		var nmn pb.MutationNode
		nmn.Id = fmt.Sprintf("%p", mn)
		label := ""

		var reqKeys []string
		var reqs = mn.spec.Requires()
		for k := range reqs {
			reqKeys = append(reqKeys, k)
		}
		sort.Strings(reqKeys)

		for _, reqKey := range reqKeys {
			reqValue := reqs[reqKey]
			// Add req to label
			trimKey := strings.Replace(reqKey, "type.googleapis.com", "", -1)
			trimKey = strings.Replace(trimKey, "/", "", -1)
			if label == "" {
				label = fmt.Sprintf("%s: %s", trimKey, util.ValueToString(reqValue))
			} else {
				label = fmt.Sprintf("%s\n%s: %s", label, trimKey, util.ValueToString(reqValue))
			}
		}

		nmn.Label = label
		r.MutationNodeList = append(r.MutationNodeList, &nmn)
	}
	return
}

// Converts a slice of sme mutation edges to a protobuf MutationEdgeList
func mutationEdgesToProto(edges []*mutationEdge) (r pb.MutationEdgeList) {
	for _, me := range edges {
		var nme pb.MutationEdge
		nme.From = fmt.Sprintf("%p", me.from)
		nme.To = fmt.Sprintf("%p", me.to)
		nme.Id = fmt.Sprintf("%p", me)

		r.MutationEdgeList = append(r.MutationEdgeList, &nme)
	}
	return
}

// Converts an sme mutation path to a protobuf MutationPath
// LOCKS: path.mutex
func mutationPathToProto(path *mutationPath) (r pb.MutationPath, e error) {
	path.mutex.Lock()
	defer path.mutex.Unlock()
	if path != nil {
		r.Cur = int64(path.cur)
		r.Cmplt = path.cmplt
		for _, me := range path.chain {
			var nme pb.MutationEdge
			nme.From = fmt.Sprintf("%p", me.from)
			nme.To = fmt.Sprintf("%p", me.to)
			nme.Id = fmt.Sprintf("%p", me)

			r.Chain = append(r.Chain, &nme)
		}
	} else {
		e = fmt.Errorf("Mutation path is nil")
	}

	return
}

// Returns the mutation nodes that have correlating reqs and execs for a given nodeID
// LOCKS: activeMutex; path.mutex
func (sme *StateMutationEngine) filterMutNodesFromNode(n ct.NodeID) (r []*mutationNode, e error) {
	// Get node from path
	sme.activeMutex.Lock()
	mp := sme.active[n.String()]
	sme.activeMutex.Unlock()
	if mp != nil {
		mp.mutex.Lock()
		node := mp.end
		mp.mutex.Unlock()

		// Combine discoverables and mutators into discoverables map
		discoverables := make(map[string]string)
		for _, siMap := range Registry.Discoverables {
			for key := range siMap {
				discoverables[key] = ""
			}
		}
		sme.graphMutex.RLock()
		for key := range sme.mutators {
			discoverables[key] = ""
		}
		sme.graphMutex.RUnlock()

		filteredNodes := make(map[*mutationNode]string)

		for _, mn := range sme.nodes {
			filteredNodes[mn] = ""
			for reqKey, reqVal := range mn.spec.Requires() {
				// if reqkey is not in discoverables
				if _, ok := discoverables[reqKey]; !ok {
					// if physical node has the reqkey as a value, check if it doesn't match
					if nodeVal, err := node.GetValue(reqKey); err == nil {
						// if it doesn't match, remove mn from final nodes
						if nodeVal.String() != reqVal.String() {
							delete(filteredNodes, mn)
						}
					}
				}
			}

			for excKey, excVal := range mn.spec.Excludes() {
				// if excKey is in discoverables, move on
				if _, ok := discoverables[excKey]; ok {
					break
				}
				// if physical node has the exckey as a value, check if it does match
				if nodeVal, err := node.GetValue(excKey); err == nil {
					// if it doesn't match, remove mn from final nodes
					if nodeVal == excVal {
						delete(filteredNodes, mn)
					}
				}
			}
		}

		for mn := range filteredNodes {
			r = append(r, mn)
		}
		sme.Logf(DDDEBUG, "Final filtered nodes from SME: %v", r)

	} else {
		e = fmt.Errorf("Can't get node info because mutation path is nil")
	}

	return
}

// Returns the mutation edges that match the filtered nodes from filterMutNodesFromNode
// LOCKS: activeMutex via filterMutNodesFromNode; path.mutex via filterMutNodesFromNode
func (sme *StateMutationEngine) filterMutEdgesFromNode(n ct.NodeID) (r []*mutationEdge, e error) {
	nodes, e := sme.filterMutNodesFromNode(n)
	filteredEdges := make(map[*mutationEdge]string)

	for _, mn := range nodes {
		for _, me := range mn.in {
			filteredEdges[me] = ""
		}
		for _, me := range mn.out {
			filteredEdges[me] = ""
		}
	}

	for me := range filteredEdges {
		r = append(r, me)
	}

	return
}

// PathExists returns a boolean indicating whether or not a path exists in the graph between two nodes.
// If the path doesn't exist, it also returns the error.
// LOCKS: graphMutex (R) via findPath
func (sme *StateMutationEngine) PathExists(start types.Node, end types.Node) (r bool, e error) {
	p, e := sme.findPath(start, end)
	if p != nil {
		r = true
	}
	return
}

// goroutine
func (sme *StateMutationEngine) sendQueryResponse(qr types.QueryResponse, r chan<- types.QueryResponse) {
	r <- qr
}

// QueryChan returns a chanel that Queries can be sent on
func (sme *StateMutationEngine) QueryChan() chan<- types.Query {
	return sme.qc
}

// Run is a goroutine that listens for state changes and performs StateMutation magic
// LOCKS: all
func (sme *StateMutationEngine) Run(ready chan<- interface{}) {
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
		sme.graphMutex.RLock()
		sme.DumpJSONGraph(sme.nodes, sme.edges) // Use this to debug your graph
		sme.graphMutex.RUnlock()

	}

	// create a listener for state change events we care about
	sme.selist = NewEventListener(
		"StateMutationEngine",
		types.Event_STATE_CHANGE,
		func(v types.Event) bool {
			_, url := util.NodeURLSplit(v.URL())
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
		func(v types.Event) error { return ChanSender(v, sme.echan) })

	// subscribe our listener
	sme.schan <- sme.selist

	smurl := regexp.MustCompile(`^\/?Services\/`)
	sme.silist = NewEventListener(
		"StateMutationEngine-SI",
		types.Event_STATE_CHANGE,
		func(v types.Event) bool {
			node, url := util.NodeURLSplit(v.URL())
			if !ct.NewNodeID(node).EqualTo(sme.self) {
				return false
			}
			if smurl.MatchString(url) {
				return true
			}
			return false
		},
		func(v types.Event) error { return ChanSender(v, sme.sichan) },
	)
	sme.schan <- sme.silist

	debugchan := make(chan interface{})
	if sme.GetLoggerLevel() >= DDEBUG {
		go func() {
			for {
				time.Sleep(10 * time.Second)
				debugchan <- nil
			}
		}()
	}

	ready <- nil
	for {
		select {
		case q := <-sme.qc:
			switch q.Type() {
			case types.Query_MUTATIONNODES:
				_, u := util.NodeURLSplit(q.URL())
				var e error
				// If url is empty then assume we want all mutation nodes
				if u == "" {
					sme.graphMutex.RLock()
					v := mutationNodesToProto(sme.nodes)
					sme.graphMutex.RUnlock()
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				} else {
					n := ct.NewNodeIDFromURL(q.URL())
					sme.graphMutex.RLock()
					fmn, e := sme.filterMutNodesFromNode(*n)
					mnl := mutationNodesToProto(fmn)
					sme.graphMutex.RUnlock()
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{reflect.ValueOf(mnl)}, e), q.ResponseChan())
				}
				break
			case types.Query_MUTATIONEDGES:
				_, u := util.NodeURLSplit(q.URL())
				var e error
				// If url is empty then assume we want all mutation edges
				if u == "" {
					sme.graphMutex.RLock()
					v := mutationEdgesToProto(sme.edges)
					sme.graphMutex.RUnlock()
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{reflect.ValueOf(v)}, e), q.ResponseChan())
				} else {
					n := ct.NewNodeIDFromURL(q.URL())
					sme.graphMutex.RLock()
					fme, e := sme.filterMutEdgesFromNode(*n)
					mel := mutationEdgesToProto(fme)
					sme.graphMutex.RUnlock()
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{reflect.ValueOf(mel)}, e), q.ResponseChan())
				}
				break
			case types.Query_MUTATIONPATH:
				n := ct.NewNodeIDFromURL(q.URL())
				sme.activeMutex.Lock()
				mp := sme.active[n.String()]
				sme.activeMutex.Unlock()
				pmp, e := mutationPathToProto(mp)
				go sme.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(pmp)}, e), q.ResponseChan())
				break
			case types.Query_FREEZE:
				sme.Freeze()
				if sme.Frozen() {
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{}, nil), q.ResponseChan())
				} else {
					e := fmt.Errorf("sme failed to freeze")
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{}, e), q.ResponseChan())
				}
				break
			case types.Query_THAW:
				sme.Thaw()
				if !sme.Frozen() {
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{}, nil), q.ResponseChan())
				} else {
					e := fmt.Errorf("sme failed to thaw")
					go sme.sendQueryResponse(NewQueryResponse(
						[]reflect.Value{}, e), q.ResponseChan())
				}
				break
			case types.Query_FROZEN:
				f := sme.Frozen()
				go sme.sendQueryResponse(NewQueryResponse(
					[]reflect.Value{reflect.ValueOf(f)}, nil), q.ResponseChan())
				break
			default:
				sme.Logf(DEBUG, "unsupported query type: %d", q.Type())
			}
			break
		case v := <-sme.echan:
			// FIXME: event processing can be expensive;
			// we should make them concurrent with a queue
			if !sme.Frozen() {
				sme.handleEvent(v)
			}
			break
		case v := <-sme.sichan:
			// Got a service change
			sme.handleServiceEvent(v.Data().(*StateChangeEvent))
		case <-debugchan:
			sme.Logf(DDEBUG, "There are %d active mutations.", len(sme.active))
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

func (sme *StateMutationEngine) remapToNode(root *mutationNode, to *mutationNode, mutOnly bool) []*mutationEdge {
	var mutEqual func(*mutationEdge, *mutationEdge) bool

	if mutOnly {
		mutEqual = func(a *mutationEdge, b *mutationEdge) bool { return a.mut == b.mut }
	} else {
		mutEqual = func(a *mutationEdge, b *mutationEdge) bool { return a.Equal(b) }
	}

	inSlice := func(s []*mutationEdge, n *mutationEdge) bool {
		for _, mn := range s {
			if mutEqual(mn, n) {
				return true
			}
		}
		return false
	}

	rmEdges := []*mutationEdge{}
	// we perform a union on in/out
	for _, in := range root.in {
		if !inSlice(to.in, in) {
			in.to = to
			to.in = append(to.in, in)
		} else {
			rmEdges = append(rmEdges, in)
		}
	}
	for _, out := range root.out {
		if !inSlice(to.out, out) {
			out.from = to
			to.out = append(to.out, out)
		} else {
			rmEdges = append(rmEdges, out)
		}
	}
	return rmEdges
}

func nodeMerge(list []int, nodes []*mutationNode) (*mutationNode, []*mutationEdge) {
	// build a new node from a merge of multiple nodes
	// use least-common specification
	// note: this will choke on a zero length list, but that shouldn't happen
	node := nodes[list[0]] // build off of the first node
	for i := 1; i < len(list); i++ {
		// remap edges
		inode := nodes[list[i]]
		for _, e := range inode.in {
			if !edgeInEdges(e, node.in) { // don't add if we already have this
				e.to = node
				node.in = append(node.in, e)
			}
		}
		for _, e := range inode.out {
			if !edgeInEdges(e, node.out) {
				e.from = node
				node.out = append(node.out, e)
			}
		}
		// prune spec
		node.spec.LeastCommon(inode.spec)
	}
	return node, node.out
}

// buildGraphStage1 builds a fully expanded set of paths branching from a starting root
// this will build a very verbose, probably unusable graph
// it is expected that later graph stages will clean it up and make it more sane
// root: current node, edge: edge that got us here (or nil), seenNodes: map of nodes we've seen
func (sme *StateMutationEngine) buildGraphStage1(root *mutationNode, edge *mutationEdge, seenNodes map[types.StateSpec]*mutationNode) ([]*mutationNode, []*mutationEdge) {
	// for this algorithm, it just complicates things to track nodes & edges at the same time.  We build the edge list at the end
	nodes := []*mutationNode{}
	edges := []*mutationEdge{}

	// is this node equal to one we've already seen?
	for sp, n := range seenNodes {
		if sp.ReqsEqual(root.spec) {
			// yes, we've seen this node, so we're done processing this chain.  Merge the nodes.
			sme.remapToNode(root, n, true)
			n.spec.LeastCommon(root.spec)
			return nodes, edges
		}
	}

	nodes = append(nodes, root)
	seenNodes[root.spec] = root

	// find connecting mutations
OUTER:
	for _, m := range sme.muts {
		// do we have a valid out arrow?
		if m.SpecCompatOut(root.spec, sme.mutators) {
			// m is a valid out arrow, create an edge for the out arrow
			// first, do we already know about this one?
			for _, edge := range root.out {
				if m == edge.mut {
					continue OUTER
				}
			}
			newEdge := &mutationEdge{
				cost: 1,
				mut:  m,
				from: root,
			}
			// ...and construct the new node that it connects to
			newNode := &mutationNode{
				spec: root.spec.SpecMergeMust(m.After()), // a combination of the current spec + the changes the mation creates
				in:   []*mutationEdge{newEdge},           // we know we have this in at least
				out:  []*mutationEdge{},                  // for now, out is empty
			}
			newEdge.to = newNode
			root.out = append(root.out, newEdge)
			// ready to recurse
			ns, _ := sme.buildGraphStage1(newNode, newEdge, seenNodes)
			nodes = append(nodes, ns...)
		} else if m.SpecCompatIn(root.spec, sme.mutators) {
			// note: it doesn't make sense for the same mutator to be both in/out.  This would be a meaningless nop.
			// m is a valid in arrow, similar to out with some subtle differences
			for _, edge := range root.in {
				if m == edge.mut {
					continue OUTER
				}
			}
			newEdge := &mutationEdge{
				cost: 1,
				mut:  m,
				to:   root,
			}
			newNode := &mutationNode{
				spec: root.spec.SpecMergeMust(m.Before()),
				in:   []*mutationEdge{},
				out:  []*mutationEdge{newEdge},
			}
			newEdge.from = newNode
			root.in = append(root.in, newEdge)
			ns, _ := sme.buildGraphStage1(newNode, newEdge, seenNodes)
			nodes = append(nodes, ns...)
		}
	}
	// build edges list
	for _, n := range nodes {
		edges = append(edges, n.out...)
	}
	return nodes, edges
}

// some useful util functions for graph building
func edgeInEdges(m *mutationEdge, es []*mutationEdge) bool {
	for _, e := range es {
		if m.Equal(e) {
			return true
		}
	}
	return false
}

func mutInEdges(m *mutationEdge, es []*mutationEdge) bool {
	for _, e := range es {
		if m.mut == e.mut {
			return true
		}
	}
	return false
}

// buildGraphReduceNodes remaps any nodes with identical edges to be the same node
// This is currently unused but may be used as a component of subgraph creation in the future
func (sme *StateMutationEngine) buildGraphReduceNodes(nodes []*mutationNode, edges []*mutationEdge) ([]*mutationNode, []*mutationEdge) {
	// some tests we'll use

	nodeEqual := func(a *mutationNode, b *mutationNode) bool {
		if len(a.in) != len(b.in) || len(a.out) != len(b.out) {
			return false
		}
		for _, e := range a.in {
			if !mutInEdges(e, b.in) {
				return false
			}
		}
		for _, e := range a.out {
			if !mutInEdges(e, b.out) {
				return false
			}
		}
		return true
	}

	mergeList := [][]int{}
	merged := map[int]bool{}
	for i := range nodes {
		if _, ok := merged[i]; ok { // skip if already merged
			continue
		}
		list := []int{i}
		for j := i + 1; j < len(nodes); j++ {
			if _, ok := merged[j]; ok { // skip if already merged
				continue
			}
			if nodeEqual(nodes[i], nodes[j]) {
				list = append(list, j)
				merged[j] = true
			}
		}
		mergeList = append(mergeList, list)
	}
	newNodes := []*mutationNode{}
	newEdges := []*mutationEdge{}
	for _, list := range mergeList {
		n, e := nodeMerge(list, nodes)
		newNodes = append(newNodes, n)
		newEdges = append(newEdges, e...)
	}
	return newNodes, newEdges
}

// buildGraphDiscoverDepends calculates the dependencies of discoveries (any mutation that goes from zero to non-zero)
// it returns a map[state_url]spec_of_dependencies
func (sme *StateMutationEngine) buildGraphDiscoverDepends(edges []*mutationEdge) map[string]types.StateSpec {
	clone := func(from map[string]reflect.Value) (to map[string]reflect.Value) {
		to = make(map[string]reflect.Value)
		for k, v := range from {
			to[k] = v
		}
		return
	}

	isDiscoverFor := func(u string, e *mutationEdge) bool {
		for mu, mvs := range e.mut.Mutates() {
			if mu == u && mvs[0].IsZero() {
				return true
			}
		}
		return false
	}

	deps := make(map[string]types.StateSpec)
	for url := range sme.mutators { // do we need to cast a wider net than mutators?
		var spec types.StateSpec
		for _, e := range edges { // find edges
			if isDiscoverFor(url, e) { // this is one of our discovery edges
				if spec == nil { // we need to start with a new, but populated spec
					reqs := clone(e.from.spec.Requires())
					excs := clone(e.from.spec.Excludes())
					// we can't require something of our own url
					// anything co-mutating should have mutation target as a requires
					// should we also remove non-discoverables?
					// FIXME is this correct?
					for k, v := range e.mut.Mutates() {
						if k == url {
							delete(reqs, k)
						} else {
							reqs[k] = v[1]
						}
					}
					delete(excs, url)
					spec = NewStateSpec(reqs, excs)
				} else {
					spec.LeastCommon(e.from.spec)
				}
			}
		}
		if spec == nil {
			sme.Logf(types.LLERROR, "failed to get discovery dependencies for: %s", url)
			continue
		}
		deps[url] = spec
	}
	return deps
}

func (sme *StateMutationEngine) nodeViolatesDeps(deps map[string]types.StateSpec, node *mutationNode) (reqs []string, excs []string) {
	// excludes don't really make sense in this context
	for u, v := range node.spec.Requires() {
		if ds, ok := deps[u]; ok { // this is an url with dependencies
			for req, val := range ds.Requires() { // for each requirement of this dep
				if nval, ok := node.spec.Requires()[req]; ok { // if the requirement is set
					if nval.Interface() != val.Interface() { // if the values aren't equal
						reqs = append(reqs, u) // we can't know this url in this node because a requirement is unequal
						sme.Logf(types.LLDDEBUG, "strip %s == %s because %s %s != %s\n", u, util.ValueToString(v), req, util.ValueToString(nval), util.ValueToString(val))
					}
				} else {
					reqs = append(reqs, u) // we can't know this url in this node because a requirement wasn't set
					sme.Logf(types.LLDDEBUG, "strip %s == %s because req %s missing\n", u, util.ValueToString(v), req)
				}
			}
			for exc, val := range ds.Excludes() { // for each exclude of this dep
				if nval, ok := node.spec.Requires()[exc]; ok { // if the exclude is set
					if nval.Interface() == val.Interface() {
						reqs = append(reqs, u) // we can't know this url because an exclude is violated
						sme.Logf(types.LLDDEBUG, "strip %s == %s because %s %s == %s\n", u, util.ValueToString(v), exc, util.ValueToString(nval), util.ValueToString(val))
					}
				}
			}
		}
	}
	return
}

func (sme *StateMutationEngine) printDeps(deps map[string]types.StateSpec) {
	// print some nice log messages documenting our dependencies
	for u := range deps {
		msg := fmt.Sprintf("Dependencies for %s: ", u)
		msg += "requires ("
		for k, v := range deps[u].Requires() {
			msg += fmt.Sprintf("%s == %s , ", k, util.ValueToString(v))
		}
		msg += "), excludes ("
		for k, v := range deps[u].Excludes() {
			msg += fmt.Sprintf("%s == %s , ", k, util.ValueToString(v))
		}
		msg += ")"
		sme.Log(types.LLDEBUG, msg)
	}
}

func (sme *StateMutationEngine) graphIsSane(nodes []*mutationNode, edges []*mutationEdge) bool {
	ret := true
	nodeEdges := map[*mutationEdge]uint{}
	edgeNodes := map[*mutationNode]uint{}
	for _, n := range nodes {
		for _, e := range n.in {
			nodeEdges[e]++
		}
		for _, e := range n.out {
			nodeEdges[e]++
		}
	}
	for _, e := range edges {
		edgeNodes[e.from]++
		edgeNodes[e.to]++
	}
	fmt.Println("=== BEGIN: Graph sanity check ===")
	// sanity checks
	// 1. Equal number of nodeEdges as edges?
	if len(nodeEdges) != len(edges) {
		fmt.Printf("len(nodeEdges) != len(edges) : %d != %d\n", len(nodeEdges), len(edges))
		ret = false
	}
	// 2. Equal number of edgeNodes as nodes?
	if len(edgeNodes) != len(nodes) {
		fmt.Printf("len(edgeNodes) != len(nodes) : %d != %d\n", len(edgeNodes), len(nodes))
		ret = false
	}
	// 2. nodeEdges should have ref count 2
	bad := []*mutationEdge{}
	for e, c := range nodeEdges {
		if c != 2 {
			bad = append(bad, e)
		}
	}
	if len(bad) > 0 {
		fmt.Printf("%d edges have ref count != 2: %v\n", len(bad), bad)
		ret = false
	}

	fmt.Println("=== END: Graph sanity check ===")
	return ret
}

// buildGraphStripState takes discovery dependencies into account
// it uses this info to simplify node state specs so they don't proclaim to know things they can't know anymore
// it then reduces nodes that became the same after forgetting extra state info
// note: we can't really know discoverable dependencies for sure until we did stage1 build
func (sme *StateMutationEngine) buildGraphStripState(nodes []*mutationNode, edges []*mutationEdge) ([]*mutationNode, []*mutationEdge) {
	deps := sme.buildGraphDiscoverDepends(edges)
	sme.printDeps(deps) // print debugging output for deps

	rmEdge := func(edges []*mutationEdge, edge *mutationEdge) []*mutationEdge {
		for i := range edges {
			if edges[i] == edge {
				edges = append(edges[:i], edges[i+1:]...)
				return edges
			}
		}
		return edges
	}

	// 1. iterate through the nodes
	//    - remove dependecy violating info
	//    - remove zero values
	//    - remap newly redundant nodes
	newNodes := []*mutationNode{}
OUTER_NODE:
	for _, n := range nodes { // for all nodes
		nr := n.spec.Requires()
		ne := n.spec.Excludes()
		vr, ve := sme.nodeViolatesDeps(deps, n) // get list of violating urls
		for _, r := range vr {
			delete(nr, r)
		}
		for _, e := range ve {
			delete(ne, e)
		}
		for u := range nr {
			if nr[u].IsZero() {
				delete(nr, u)
			}
		}
		for u := range ne {
			if ne[u].IsZero() {
				delete(ne, u)
			}
		}
		n.spec = NewStateSpec(nr, ne)
		// Now, has this node become redundant?  If so, remap it.
		for _, nn := range newNodes {
			// We only care that the requirements are the same
			// It's OK if the excludes get broader
			if n.spec.ReqsEqual(nn.spec) { // duplicate node
				dead := sme.remapToNode(n, nn, false)
				for _, e := range dead {
					rmEdge(edges, e)
				}
				nn.spec.LeastCommon(n.spec) // strip extra excludes
				continue OUTER_NODE
			}
		}
		newNodes = append(newNodes, n)
	}

	// 2. Now we need to remove edges that have become violations

	newEdges := []*mutationEdge{}
OUTER_EDGE:
	for _, e := range edges {
		// first, skip this edge if we've already seen one equal to it
		if edgeInEdges(e, newEdges) {
			e.from.out = rmEdge(e.from.out, e)
			e.to.in = rmEdge(e.to.in, e)
			continue
		}
		imp := e.from.spec.SpecMergeMust(e.mut.After())
		if len(imp.Requires()) < len(e.to.spec.Requires()) { // we're not allowed to gain extra mutation information
			for u := range e.to.spec.Requires() {
				if _, ok := imp.Requires()[u]; !ok { // outlier
					if _, ok := sme.mutators[u]; ok { // and a mutator, delete
						e.from.out = rmEdge(e.from.out, e)
						e.to.in = rmEdge(e.to.in, e)
						continue OUTER_EDGE
					}
				}
			}
		}
		if !e.mut.SpecCompatIn(e.to.spec, sme.mutators) || !e.mut.SpecCompatOut(e.from.spec, sme.mutators) { // this edge is no longer compatible
			e.from.out = rmEdge(e.from.out, e)
			e.to.in = rmEdge(e.to.in, e)
			continue
		}
		newEdges = append(newEdges, e)
	}

	return newNodes, newEdges
}

// buildGraph builds the graph of Specs/Mutations.  It is depth-first, recursive.
// TODO: this function may eventually need recursion protection
// !!!IMPORTANT!!!
// buildGraph assumes you already hold a lock
// currently only used in onUpdate
func (sme *StateMutationEngine) buildGraph(root *mutationNode) (nodes []*mutationNode, edges []*mutationEdge) {
	nodes, edges = sme.buildGraphStage1(root, nil, map[types.StateSpec]*mutationNode{})
	if sme.log.GetLoggerLevel() > types.LLDEBUG {
		sme.graphIsSane(nodes, edges)
	}

	if sme.log.GetLoggerLevel() > types.LLDDEBUG {
		sme.DumpJSONGraph(nodes, edges)
	}

	nodes, edges = sme.buildGraphStripState(nodes, edges)
	if sme.log.GetLoggerLevel() > types.LLDEBUG {
		sme.graphIsSane(nodes, edges)
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
	sme.nodes, sme.edges = sme.buildGraph(sme.graph)
	sme.Logf(DEBUG, "Built graph [ Mutations: %d Mutation URLs: %d Requires URLs: %d Graph Nodes: %d Graph Edges: %d ]",
		len(sme.muts), len(sme.mutators), len(sme.requires), len(sme.nodes), len(sme.edges))
	sme.graphMutex.Unlock()
}

// LOCKS: graphMutex (R)
func (sme *StateMutationEngine) nodeSearch(node types.Node) (mns []*mutationNode) {
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
func (sme *StateMutationEngine) boundarySearch(start types.Node, end types.Node) (gstart []*mutationNode, gend []*mutationNode) {
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
				cmplt:   false,
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

// findPath finds the sequence of edges (if it exists) between two types.Nodes
// LOCKS: graphMutex (R) via boundarySearch, drijkstra
func (sme *StateMutationEngine) findPath(start types.Node, end types.Node) (path *mutationPath, e error) {
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
			cmplt:   true,
			curSeen: []string{},
			chain:   []*mutationEdge{},
		}
		return
	}
	gs, ge := sme.boundarySearch(start, end)
	if len(gs) < 1 {
		e = fmt.Errorf("could not find path: start not in graph")
	}
	if len(ge) < 1 {
		e = fmt.Errorf("could not find path: end not in graph")
		if sme.GetLoggerLevel() >= DDEBUG {
			fmt.Printf("start: %v, end: %v\n", string(start.JSON()), string(end.JSON()))
			sme.DumpGraph()
			sme.graphMutex.RLock()
			sme.DumpJSONGraph(sme.nodes, sme.edges) // Use this to debug your graph
			sme.graphMutex.RUnlock()
		}
	}
	if e != nil {
		return
	}
	// try starts until we get a path (or fail)
	for _, st := range gs {
		path = sme.drijkstra(st, ge) // we require a unique start, but not a unique end
		if path.chain != nil {
			break
		}
	}
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
	nid := ct.NewNodeIDFromURL(node)
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
		if sme.waitForServices(p) {
			return
		}
		sme.Logf(DDEBUG, "firing mutation in context, timeout %s.", p.chain[p.cur].mut.Timeout().String())
		sme.emitMutation(end, start, p.chain[p.cur].mut)
		if p.chain[p.cur].mut.Timeout() != 0 {
			if p.timer != nil {
				// Stop old timer if it exists
				p.timer.Stop()
			}
			p.timer = time.AfterFunc(p.chain[p.cur].mut.Timeout(), func() { sme.emitFail(start, p) })
		}
	} else {
		sme.Log(DDEBUG, "mutation is not in our context.")
	}
}

// Assumes that path is already locked
// LOCKS: activeMutex
func (sme *StateMutationEngine) waitForServices(p *mutationPath) (wait bool) {
	m, _ := sme.mutResolver[p.chain[p.cur].mut] // what ServiceInstance do we depend on?
	// is there a current waitFor queue for this SI?
	si := m[0]

	// we don't wait for core mutations
	if si == "core" {
		return
	}

	sme.activeMutex.Lock()
	defer sme.activeMutex.Unlock()
	if _, ok := sme.waiting[si]; ok {
		// queue already exists, just add ourselves to it
		sme.waiting[si] = append(sme.waiting[si], p)
		p.waitingFor = si
		sme.Logf(INFO, "%s is waiting for service %s", p.end.ID().String(), si)
		return true
	}

	// queue doesn't already exist, is service running?  Is this too expensive?
	url := util.NodeURLJoin(sme.self.String(), util.URLPush(util.URLPush("/Services", si), "State"))
	v, e := sme.query.GetValueDsc(url)
	if e != nil {
		sme.Logf(ERROR, "waitForServices could not lookup service state (%s): %v", url, e)
		return
	}
	if pb.ServiceInstance_ServiceState(v.Int()) != pb.ServiceInstance_RUN {
		// Mutation was requested for a service that isn't running yet
		// 1. Set it to run
		if _, e := sme.query.SetValue(url, reflect.ValueOf(pb.ServiceInstance_RUN)); e != nil {
			sme.Logf(ERROR, "waitForServices failed to set service state (%s): %v", url, e)
			// we still continue
		}
		// 3. Create a waitlist of this SI
		sme.waiting[si] = []*mutationPath{p}
		p.waitingFor = si
		sme.Logf(INFO, "%s is waiting for service %s", p.end.ID().String(), si)
		return true
	}
	return
}

// unwaitForService clears waiting status for path
// assumes activeMutex is already locked
func (sme *StateMutationEngine) unwaitForService(p *mutationPath) {
	if p.waitingFor != "" {
		queue := sme.waiting[p.waitingFor]
		for i := range queue {
			if queue[i] == p {
				// order isn't important
				queue[i] = queue[len(queue)-1]
				sme.waiting[p.waitingFor] = queue[:len(queue)-1]
				break
			}
		}
		p.waitingFor = ""
	}
}

func (sme *StateMutationEngine) handleServiceEvent(v *StateChangeEvent) {
	if v.Type != StateChange_UPDATE {
		return
	} // DSC update

	_, url := util.NodeURLSplit(v.URL)
	us := util.URLToSlice(url)
	if us[len(us)-1] != "State" {
		// not a change in service state
		return
	}

	if v.Value.Kind() != reflect.TypeOf(pb.ServiceInstance_RUN).Kind() {
		// it looks like we weren't actually passed the state value
		// this shouldn't happen, but if we don't check we could panic
		return
	}

	if pb.ServiceInstance_ServiceState(v.Value.Int()) != pb.ServiceInstance_RUN {
		return
	} // service discovered run state

	// get SI
	si := ""
	// this makes sure we don't get tripped up by leading slashes
	for i := range us {
		if us[i] == "Services" {
			si = us[i+1]
		}
	}
	if si == "" {
		sme.Logf(types.LLDEBUG, "failed to parse URL for /Services state change: %s", v.URL)
		return
	}

	// OK, let's resume any waiting chains
	sme.activeMutex.Lock()
	queue, ok := sme.waiting[si]
	if ok {
		delete(sme.waiting, si)
	} else {
		queue = []*mutationPath{}
	}
	sme.activeMutex.Unlock()
	for _, p := range queue {
		p.mutex.Lock()
		p.waitingFor = ""
		p.cur-- // we have to rewind one to advance.  This could even mean we go negative
		sme.advanceMutation(p.end.ID().String(), p)
		p.mutex.Unlock()
	}
}

// LOCKS: graphMutex (R); path.mutex
func (sme *StateMutationEngine) emitFail(start types.Node, p *mutationPath) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	nid := p.start.ID()
	d := p.chain[p.cur].mut.FailTo()
	sme.Logf(INFO, "mutation timeout for %s, emitting: %s:%s:%s", nid.String(), d[0], d[1], d[2])

	// try devolve first
	val, ok := Registry.Discoverables[d[0]][d[1]][d[2]]
	if !ok {
		sme.Logf(ERROR, "could not find value %v:%v:%v in discoverables registry", d[0], d[1], d[2])
		return
	}

	rewind, i, err := sme.devolve(p, d[1], val)

	n, e := sme.query.ReadDsc(nid)
	if e != nil {
		// ok, I give up.  The node has just disappeared.
		sme.Logf(ERROR, "%s node unexpectly disappeared", nid.String())
		return
	}
	nc, e := sme.query.Read(nid)
	if e != nil {
		// ok, I give up.  The node has just disappeared.
		sme.Logf(ERROR, "%s node unexpectly disappeared", nid.String())
		return
	}

	// this is a devolution
	if err == nil {
		// ok, let's devolve
		sme.Logf(DEBUG, "%s is devolving back %d steps due to an unexpected regression", nid, p.cur-i)

		n.SetValues(rewind)
		p.chain = append(p.chain[:p.cur+1], p.chain[i:]...)
		sme.advanceMutation(nid.String(), p)
		return
	}

	// We couldn't devolve so...
	// Create fake types.node with our failto and all non-mutators (sme.requires).
	// These non-mutators do not exist in the dsc (platform for example) so we have to pull from the cfg node
	fn := NewNodeWithID(n.ID().String())
	sme.graphMutex.RLock()
	for r := range sme.requires {
		if r == d[1] {
			continue
		}
		v, _ := n.GetValue(r)
		if v.Interface() == reflect.Zero(v.Type()).Interface() {
			v, _ = nc.GetValue(r)
		}
		fn.SetValue(r, v)
	}
	sme.graphMutex.RUnlock()
	fn.SetValue(d[1], val)

	// get all possible mutation nodes for this fake types.node
	pns := sme.nodeSearch(fn)

	if len(pns) == 0 {
		// we didn't find any possible mutation nodes so let's give up and set all mutators to zero
		sme.Logf(DEBUG, "failed to find possible mutation node for %s. Resetting all mutators to zero", nid)

		// reset all mutators to zero, except the failure mutator
		// FIXME: setting things without discovery isn't very polite
		node, _ := sme.query.ReadDsc(nid)
		sme.graphMutex.RLock()
		for m := range sme.mutators {
			if m == d[1] {
				continue
			}
			v, _ := node.GetValue(m)
			sme.Logf(DDEBUG, "setting %s:%s to zero", node.ID().String(), m)
			sme.query.SetValueDsc(util.NodeURLJoin(node.ID().String(), m), reflect.Zero(v.Type()))
		}
		sme.graphMutex.RUnlock()

	} else {
		// we found some possible mutation nodes for our fake types.node. Lets just take the first one and force our types.node to match it.
		sme.Logf(DEBUG, "found a matching mutation node for %s. Setting mutators to equal mutation node's requires: %v", nid, pns[0].spec.Requires())

		// loop through all the mutators
		// if the mutator is the failto, skip
		// if the mutator is in the requires of this mutation node, set it to whatever it requires
		// if the mutator isn't either of those, set it to zero
		sme.graphMutex.RLock()
		for m := range sme.mutators {
			if m == d[1] {
				continue
			}
			if val, ok := pns[0].spec.Requires()[m]; ok {
				sme.Logf(DDEBUG, "setting %s:%s to %v", n.ID().String(), m, val.Interface())
				sme.query.SetValueDsc(util.NodeURLJoin(n.ID().String(), m), val)
			} else {
				v, _ := n.GetValue(m)
				sme.Logf(DDEBUG, "setting %s:%s to zero", n.ID().String(), m)
				sme.query.SetValueDsc(util.NodeURLJoin(n.ID().String(), m), reflect.Zero(v.Type()))
			}
		}
		sme.graphMutex.RUnlock()
	}

	// now send a discover to whatever failed state
	url := util.NodeURLJoin(nid.String(), d[1])
	dv := NewEvent(
		types.Event_DISCOVERY,
		url,
		&DiscoveryEvent{
			ID:      d[0],
			URL:     url,
			ValueID: d[2],
		},
	)

	// send a mutation interrupt
	iv := NewEvent(
		types.Event_STATE_MUTATION,
		url,
		&MutationEvent{
			Type:     pb.MutationControl_INTERRUPT,
			NodeCfg:  p.end,
			NodeDsc:  start,
			Mutation: sme.mutResolver[p.chain[p.cur].mut],
		},
	)
	sme.Emit([]types.Event{dv, iv})
}

// advanceMutation fires off the next mutation in the chain
// does *not* check to make sure there is one
// assumes m.mutex is locked by surrounding func
func (sme *StateMutationEngine) advanceMutation(node string, m *mutationPath) {
	nid := ct.NewNodeIDFromURL(node)
	m.cur++
	m.curSeen = []string{}
	sme.Logf(DEBUG, "resuming mutation for %s (%d/%d).", nid.String(), m.cur+1, len(m.chain))
	if sme.mutationInContext(m.end, m.chain[m.cur].mut) {
		if sme.waitForServices(m) {
			return
		}
		sme.Logf(DDEBUG, "firing mutation in context, timeout %s.", m.chain[m.cur].mut.Timeout().String())
		sme.emitMutation(m.end, m.start, m.chain[m.cur].mut)
		if m.chain[m.cur].mut.Timeout() != 0 {
			if m.timer != nil {
				// Stop old timer if it exists
				m.timer.Stop()
			}
			m.timer = time.AfterFunc(m.chain[m.cur].mut.Timeout(), func() { sme.emitFail(m.start, m) })
		}
	} else {
		sme.Logf(DDEBUG, "node (%s) mutation is not in our context", node)
	}
}

// devolve will reverse through a mutation path until it gets to the desired url and val.
// If it succeeds, it will return a map of urls to values that need to be set to devolve and the index of the devolve point in the mutation chain.
// If it fails to devolve, it will return an error.
// LOCKS: This assumes mutation path has been locked
func (sme *StateMutationEngine) devolve(m *mutationPath, url string, val reflect.Value) (map[string]reflect.Value, int, error) {
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
				rewind[murl] = mvs[0]
			}
		}
		if found {
			break
		}
	}

	if found {
		return rewind, i, nil
	}
	e := fmt.Errorf("could not find desired url in mutation path for devolve")
	return nil, 0, e
}

// handleUnexpected deals with unexpected events in updateMutation
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
	m.cmplt = false

	// this is a bit bad.  We don't want to get our own state changes, so we change the node directly
	nid := ct.NewNodeIDFromURL(node)
	n, e := sme.query.ReadDsc(nid)
	if e != nil {
		// ok, I give up.  The node has just disappeared.
		sme.Logf(ERROR, "%s node unexpectly disappeared", node)
		return
	}

	// can we find a path?
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
		m.chain = append(m.chain[:m.cur+1], p.chain...)
		sme.advanceMutation(node, m)
		return
	}

	sme.Logf(DEBUG, "%s could neither find a path, nor devolve.  We're lost.", node)
}

// updateMutation attempts to progress along an existing mutation chain
// LOCKS: activeMutex; path.mutex; graphMutex (R) via startNewMutation
func (sme *StateMutationEngine) updateMutation(node string, url string, val reflect.Value) {
	sme.activeMutex.Lock()
	m, ok := sme.active[node]
	if !ok {
		// this shouldn't happen
		sme.Logf(DDEBUG, "call to updateMutation, but no mutation exists %s", node)
		sme.startNewMutation(node)
		sme.activeMutex.Unlock()
		return
	}
	// we should reset waiting status
	sme.unwaitForService(m)
	sme.activeMutex.Unlock()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// stop any timer clocks
	if m.timer != nil {
		m.timer.Stop()
	}

	// we still query this to make sure it's the Dsc value
	var e error
	val, e = sme.query.GetValueDsc(util.NodeURLJoin(node, url))
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
			m.cmplt = true
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
func (sme *StateMutationEngine) mutationInContext(n types.Node, m types.StateMutation) (r bool) {
	switch m.Context() {
	case types.StateMutationContext_SELF:
		if sme.self.EqualTo(n.ID()) {
			return true
		}
		break
	case types.StateMutationContext_CHILD:
		if sme.self.EqualTo(n.ParentID()) {
			return true
		}
		break
	case types.StateMutationContext_ALL:
		return true
	}
	return
}

func (sme *StateMutationEngine) handleEvent(v types.Event) {
	sce := v.Data().(*StateChangeEvent)
	node, url := util.NodeURLSplit(sce.URL)
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
			m := sme.active[node]
			m.mutex.Lock()
			if m.timer != nil {
				m.timer.Stop()
			}
			sme.unwaitForService(m)
			delete(sme.active, node)
			m.mutex.Unlock()
			sme.activeMutex.Unlock()
		}
		sme.startNewMutation(node)
	case StateChange_DELETE:
		if ok {
			sme.activeMutex.Lock()
			m := sme.active[node]
			m.mutex.Lock()
			if m.timer != nil {
				m.timer.Stop()
			}
			sme.unwaitForService(m)
			delete(sme.active, node)
			m.mutex.Unlock()
			sme.activeMutex.Unlock()
		}
	case StateChange_UPDATE:
		sme.updateMutation(node, url, sce.Value)
	case StateChange_CFG_UPDATE:
		// for a cfg update, we need to create a new chain
		if ok {
			sme.activeMutex.Lock()
			m := sme.active[node]
			m.mutex.Lock()
			if m.timer != nil {
				m.timer.Stop()
			}
			sme.unwaitForService(m)
			delete(sme.active, node)
			m.mutex.Unlock()
			sme.activeMutex.Unlock()
		}
		sme.Logf(DEBUG, "our cfg has changed, creating new mutaiton path: %s:%s", node, url)
		sme.startNewMutation(node)
	default:
	}
}

// LOCKS: graphMutex (R)
func (sme *StateMutationEngine) emitMutation(cfg types.Node, dsc types.Node, sm types.StateMutation) {
	sme.graphMutex.RLock()
	smee := &MutationEvent{
		Type:     MutationEvent_MUTATE,
		NodeCfg:  cfg,
		NodeDsc:  dsc,
		Mutation: sme.mutResolver[sm],
	}
	sme.graphMutex.RUnlock()
	v := NewEvent(
		types.Event_STATE_MUTATION,
		cfg.ID().String(),
		smee,
	)
	sme.EmitOne(v)
}

// It might be useful to export this
// Also, there's no particular reason it belongs here
// This takes the cfg state and merges only discoverable values from dsc state into it
func (sme *StateMutationEngine) dscNodeMeld(cfg, dsc types.Node) (r types.Node) {
	r = NewNodeFromMessage(cfg.Message().(*pb.Node)) // might be a bit expensive
	diff := []string{}
	for si := range Registry.Discoverables {
		for u := range Registry.Discoverables[si] {
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
var _ types.Logger = (*StateMutationEngine)(nil)

func (sme *StateMutationEngine) Log(level types.LoggerLevel, m string) { sme.log.Log(level, m) }
func (sme *StateMutationEngine) Logf(level types.LoggerLevel, fmt string, v ...interface{}) {
	sme.log.Logf(level, fmt, v...)
}
func (sme *StateMutationEngine) SetModule(name string) { sme.log.SetModule(name) }
func (sme *StateMutationEngine) GetModule() string     { return sme.log.GetModule() }
func (sme *StateMutationEngine) SetLoggerLevel(level types.LoggerLevel) {
	sme.log.SetLoggerLevel(level)
}
func (sme *StateMutationEngine) GetLoggerLevel() types.LoggerLevel { return sme.log.GetLoggerLevel() }
func (sme *StateMutationEngine) IsEnabledFor(level types.LoggerLevel) bool {
	return sme.log.IsEnabledFor(level)
}

/*
 * Consume an emitter, so we implement EventEmitter directly
 */

func (sme *StateMutationEngine) Subscribe(id string, c chan<- []types.Event) error {
	return sme.em.Subscribe(id, c)
}
func (sme *StateMutationEngine) Unsubscribe(id string) error { return sme.em.Unsubscribe(id) }
func (sme *StateMutationEngine) Emit(v []types.Event)        { sme.em.Emit(v) }
func (sme *StateMutationEngine) EmitOne(v types.Event)       { sme.em.EmitOne(v) }
func (sme *StateMutationEngine) EventType() types.EventType  { return sme.em.EventType() }
