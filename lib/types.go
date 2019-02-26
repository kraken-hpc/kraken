/* types.go - Defines core interface types
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package lib

import (
	"os/exec"
	"reflect"
	"time"

	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	pb "github.com/hpc/kraken/core/proto"
)

/*
 * Every node needs a unique ID
 */

// NodeID interface defines what we require of a node identification field
// NodeID methods don't return errors, but may be Nil
type NodeID interface {
	Equal(NodeID) bool
	Binary() []byte
	String() string
	Nil() bool
}

/*
 * Nodes - not literally compute nodes; nodes of the state tree
 *         for now, we don't abstract away ProtoBuf
 */

// ProtoMessage wraps proto.Message with extra functionality we need
// Every root proto.Message should have a ProtoMessage.
// In general, we manipulate values by getting the proto.Message
type ProtoMessage interface {
	GetMessage() proto.Message
	SetMessage(proto.Message)

	// We use these so we can naturally handle things like extensions
	MarshalProto() ([]byte, error)
	UnmarshalProto([]byte) error

	MarshalJSON() ([]byte, error)
	UnmarshalJSON([]byte) error
}

// ExtensibleProtoMessage is a ProtoMessage that can handle extensions
type ExtensibleProtoMessage interface {
	ProtoMessage
	AddExtension(ProtoMessage) error
	DelExtension(url string)
	GetExtension(url string) ProtoMessage
	GetExtensionUrls() []string
}

// Node is a node of the state tree, i.e. any item we store info about
type Node interface {
	ID() NodeID
	ParentID() NodeID

	JSON() []byte
	Binary() []byte
	Message() proto.Message

	// Primary way to access data
	GetValue(url string) (v reflect.Value, e error)
	SetValue(url string, value reflect.Value) (v reflect.Value, e error)
	GetValues(urls []string) (v map[string]reflect.Value)
	SetValues(valmap map[string]reflect.Value) (v map[string]reflect.Value)

	GetExtensionURLs() []string
	AddExtension(proto.Message) error
	DelExtension(url string)
	HasExtension(url string) bool

	AddService(ServiceInstance) error
	DelService(id string)
	GetService(id string) ServiceInstance
	GetServiceIDs() []string
	GetServices() []ServiceInstance
	HasService(id string) bool

	Diff(node Node, prefix string) (diff []string, e error)
	MergeDiff(m Node, diff []string) (changes []string, e error)
	Merge(m Node, prefix string) (changes []string, e error)

	String() string
}

// Indexable can be indexed by multiple values
type Indexable interface {
	GetKey(idx string) (key string)
}

// IndexableNode 's are nodes that can be indexed
type IndexableNode interface {
	Node
	Indexable
}

/*
 * State - interface for storing/retrieving & manipulating collections
 *         of nodes.
 */

// CRUD operations for state
type CRUD interface {
	Create(Node) (Node, error)
	Read(NodeID) (Node, error)
	Update(Node) (Node, error)
	Delete(Node) (Node, error)
	DeleteByID(NodeID) (Node, error)
}

// BulkCRUD does bulk operations, but not queries
type BulkCRUD interface {
	CRUD
	BulkCreate([]Node) ([]Node, error)
	BulkRead([]NodeID) ([]Node, error)
	BulkUpdate([]Node) ([]Node, error)
	BulkDelete([]Node) ([]Node, error)
	BulkDeleteByID([]NodeID) ([]Node, error)

	ReadAll() ([]Node, error)
	DeleteAll() ([]Node, error)
}

// Resolver operations for mapping values to URLs.
// This allows for manipulating single values.
type Resolver interface {
	GetValue(string) (reflect.Value, error)
	SetValue(string, reflect.Value) (reflect.Value, error)
}

// Queryable implementations implement a basic query language
// Querables support bulk operations
type Queryable interface {
	Search(key string, value reflect.Value) []string
	QuerySelect(query string) ([]Node, error)
	QueryUpdate(query string, value reflect.Value) ([]reflect.Value, error)
	QueryDelete(query string) ([]Node, error)
}

// State brings it all together
type State interface {
	BulkCRUD
	Resolver
	//Queryable
}

type EventType uint8

const (
	Event_CONTROL EventType = iota
	Event_STATE_CHANGE
	Event_STATE_MUTATION
	Event_STATE_SYNC
	Event_API
	Event_DISCOVERY
)

var EventTypeString = map[EventType]string{
	Event_CONTROL:        "CONTROL",
	Event_STATE_CHANGE:   "STATE_CHANGE",
	Event_STATE_MUTATION: "STATE_MUTATION",
	Event_STATE_SYNC:     "STATE_SYNC",
	Event_API:            "API",
	Event_DISCOVERY:      "DISCOVERY",
}

// Event 's capture a happening's type, location, and optional data
type Event interface {
	Type() EventType   // We may need to handle event types differently
	URL() string       // URL must describe what the event pertains to
	Data() interface{} // consumer should know what we have based on type
}

// EventEmitter 's emit events. They're a firehose; no filtering.
// It's expected that the subscriber will be an event dispatcher
// that will make decisions about where the events need to go.
// An Emitter emits only one EventType.
type EventEmitter interface {
	Subscribe(string, chan<- []Event) error
	Unsubscribe(string) error
	Emit([]Event)
	EmitOne(Event)
	EventType() EventType
}

type Index interface{}

// IndexableState 's are states that maintain indexes of IndexableNodes
/* TODO
type IndexableState interface {
	State
	CreateIndex(i Index) error
	DeleteIndex(key string) error
	RebuildIndex(key string) error
	QueryIndex(key string, value string) ([]IndexableNode, error)
}
*/

// An StateDifferenceEngine is an Emitter that tracks state changes across two States: current & intended
// the two states must maintain identical node structure, so CREATE and DELETE operations
// only happen once.
type StateDifferenceEngine interface {
	EventEmitter
	State // by default, these operations apply to Cfg state, except create/delete which apply to both
	// Discoverable state specific queries
	ReadDsc(nid NodeID) (r Node, e error)
	BulkReadDsc(nids []NodeID) (r []Node, e error)
	UpdateDsc(m Node) (r Node, e error)
	BulkUpdateDsc(m []Node) (r []Node, e error)
	GetValueDsc(url string) (r reflect.Value, e error)
	SetValueDsc(url string, v reflect.Value) (r reflect.Value, e error)
	QueryChan() chan<- Query
	// goroutine that manages engine queries
	Run()
}

// An EventDispatchEngine subscribes to event sources and re-transmits events
// It can filter events for its subscribers
// In the future, it should probably also authorize event listeners
type EventDispatchEngine interface {
	// Direct call to subscribe, or modify a subscription
	AddListener(listener EventListener) error
	// Send an EventListener to subscribe, or modify a subscription
	SubscriptionChan() chan<- EventListener
	EventChan() chan<- []Event
	Run() // goroutine
}

// An EventListener decies if an event should be provided on this subscription.
// It also provides the channel on which it should be provided.
// Name must be unique. It is used to key Listeners for logging and subscription modification.
// Send should call Filter, and should always send iff Filter == true
// Filter is exposed so a Dispatch can know if a message would be sent without sending.
type EventListener interface {
	Name() string
	Filter(Event) bool
	Send(Event) error
	State() EventListenerState
	SetState(EventListenerState)
	Type() EventType
}

type EventListenerState uint8

const (
	EventListener_STOP        EventListenerState = 0
	EventListener_RUN         EventListenerState = 1
	EventListener_UNSUBSCRIBE EventListenerState = 2
)

/*
 * Engine query language
 */

type QueryEngineType uint8

const (
	Query_SDE QueryEngineType = iota
	Query_SME
)

type QueryType uint8

const (
	Query_NIL QueryType = iota
	Query_CREATE
	Query_READ
	Query_UPDATE
	Query_DELETE
	Query_READALL
	Query_DELETEALL
	Query_GETVALUE
	Query_SETVALUE
	Query_RESPONSE
	Query_MUTATIONNODES
	Query_MUTATIONEDGES
	Query_MUTATIONPATH
)

var QueryTypeMap = map[QueryType]QueryEngineType{
	Query_CREATE:        Query_SDE,
	Query_READ:          Query_SDE,
	Query_UPDATE:        Query_SDE,
	Query_DELETE:        Query_SDE,
	Query_READALL:       Query_SDE,
	Query_DELETEALL:     Query_SDE,
	Query_GETVALUE:      Query_SDE,
	Query_SETVALUE:      Query_SDE,
	Query_RESPONSE:      Query_SDE,
	Query_MUTATIONNODES: Query_SME,
	Query_MUTATIONEDGES: Query_SME,
	Query_MUTATIONPATH:  Query_SME,
}

type QueryState uint8

const (
	QueryState_BOTH QueryState = iota
	QueryState_CONFIG
	QueryState_DISCOVER
)

type Query interface {
	Type() QueryType
	State() QueryState
	URL() string
	Value() []reflect.Value
	ResponseChan() chan<- QueryResponse
}

type QueryResponse interface {
	Error() error
	Value() []reflect.Value
}

/*
 * State Mutation interfaces
 */

// A StateSpec is essentially a filter that determines if a given state
// falls within the spec or not.  It currently systems of required and excluded
// values for specific URLs.
type StateSpec interface {
	NodeMatch(Node) bool
	SpecCompat(StateSpec) bool
	SpecMerge(StateSpec) (StateSpec, error)
	SpecMergeMust(StateSpec) StateSpec
	Requires() map[string]reflect.Value
	Excludes() map[string]reflect.Value
	Equal(StateSpec) bool
	NodeMatchWithMutators(n Node, muts map[string]uint32) (r bool)  // how we find path starts
	NodeCompatWithMutators(n Node, muts map[string]uint32) (r bool) // how we find path ends
}

// StateMutationContext specifies the context in which a mutation is activated
type StateMutationContext uint8

const (
	StateMutationContext_SELF  StateMutationContext = 0
	StateMutationContext_CHILD StateMutationContext = 1
	StateMutationContext_ALL   StateMutationContext = 2
)

// StateMutation describes a mutation of state.  It does not define how this
// mutation is performed, only the mutation itself.
// It can determine if the mutation will mutate a given node, as well as
// if two mutations can form an edge.
type StateMutation interface {
	Mutates() map[string][2]reflect.Value
	Requires() map[string]reflect.Value
	Excludes() map[string]reflect.Value
	Context() StateMutationContext
	Before() StateSpec
	After() StateSpec
	CanMutateNode(Node) bool
	MutationCompat(StateMutation) bool
	SpecCompat(StateSpec) bool
	SpecCompatWithMutators(StateSpec, map[string]uint32) bool
	Timeout() time.Duration
	FailTo() [3]string // discover address: module:url:value_id
}

type StateMutationEngine interface {
	EventEmitter
	RegisterMutation(module, id string, mut StateMutation) error
	NodeMatch(node Node) int
	PathExists(start Node, end Node) (bool, error)
	Run()
}

/*
 * Logger interface
 */
type LoggerLevel uint8

const (
	LLPANIC    LoggerLevel = iota
	LLFATAL    LoggerLevel = iota
	LLCRITICAL LoggerLevel = iota
	LLERROR    LoggerLevel = iota
	LLWARNING  LoggerLevel = iota
	LLNOTICE   LoggerLevel = iota
	LLINFO     LoggerLevel = iota
	LLDEBUG    LoggerLevel = iota
	LLDDEBUG   LoggerLevel = iota
	LLDDDEBUG  LoggerLevel = iota
)

var LoggerLevels = [...]string{
	"PANIC",
	"FATAL",
	"CRITICAL",
	"ERROR",
	"WARNING",
	"NOTICE",
	"INFO",
	"DEBUG",
	"DDEBUG",
	"DDDEBUG",
}

type Logger interface {
	Log(level LoggerLevel, m string)
	Logf(level LoggerLevel, fmt string, v ...interface{})

	SetModule(name string)
	GetModule() string

	SetLoggerLevel(LoggerLevel)
	GetLoggerLevel() LoggerLevel
	IsEnabledFor(LoggerLevel) bool
}

/*
 * StateSync
 */

type StateSyncEngine interface {
	EventEmitter
}

/*
 * Service infrastructure
 */
type ServiceState pb.ServiceInstance_ServiceState

const (
	Service_UNKNOWN ServiceState = ServiceState(pb.ServiceInstance_UNKNOWN)
	Service_STOP    ServiceState = ServiceState(pb.ServiceInstance_STOP)
	Service_INIT    ServiceState = ServiceState(pb.ServiceInstance_INIT)
	Service_RUN     ServiceState = ServiceState(pb.ServiceInstance_RUN)
	Service_ERROR   ServiceState = ServiceState(pb.ServiceInstance_ERROR)
)

// consume these so the client doesn't need to import the protobuf
type ServiceControl_Command pb.ServiceControl_Command

const (
	ServiceControl_STOP   ServiceControl_Command = ServiceControl_Command(pb.ServiceControl_STOP)
	ServiceControl_UPDATE ServiceControl_Command = ServiceControl_Command(pb.ServiceControl_UPDATE)
	ServiceControl_INIT   ServiceControl_Command = ServiceControl_Command(pb.ServiceControl_INIT)
)

type ServiceControl struct {
	Command ServiceControl_Command
	Config  *any.Any
}
type ServiceInstance interface {
	ID() string
	State() ServiceState
	SetState(ServiceState)
	GetState() ServiceState // GetState tries to discover the real state on a node running the service
	Module() string
	Exe() string // services take two arguments - connect string, and instance ID
	Cmd() *exec.Cmd
	SetCmd(*exec.Cmd)
	Stop()
	SetCtl(chan<- ServiceControl)
	Config() *any.Any
	UpdateConfig(*any.Any)
	Message() *pb.ServiceInstance
}

// A ServiceManager handles the lifecycle of external services
type ServiceManager interface {
	AddService(ServiceInstance) error
	AddServiceByModule(id string, name string, cfg *any.Any) error
	DelService(name string) error
	GetServiceIDs() []string

	Service(id string) ServiceInstance // returns a service by ID, nil if not found

	RunService(id string) error
	StopService(id string) error

	SyncNode(n Node) map[string]ServiceState
}

/*
 * Extensions & Modules
 */

type Extension interface {
	New() proto.Message // should return a proto.Message object with initialized default values
	Name() string       // this needs to be a name unique to all extensions; used as a map key
}

type Module interface {
	Name() string
}

type ModuleSelfService interface {
	Module
	Entry()
	Init(api APIClient)
	//Execute()
	Stop()
}

type ModuleWithConfig interface {
	Module
	NewConfig() proto.Message
	UpdateConfig(proto.Message) error
	ConfigURL() string
}

type ModuleWithMutations interface {
	Module
	SetMutationChan(<-chan Event)
}

type ModuleWithDiscovery interface {
	Module
	SetDiscoveryChan(chan<- Event)
}

type APIClient interface {
	Logger
	Self() NodeID
	QueryCreate(Node) (Node, error)
	QueryRead(string) (Node, error)
	QueryReadDsc(string) (Node, error)
	QueryUpdate(Node) (Node, error)
	QueryUpdateDsc(Node) (Node, error)
	QueryDelete(string) (Node, error)
	QueryReadAll() ([]Node, error)
	QueryReadAllDsc() ([]Node, error)
	QueryMutationNodes() (pb.MutationNodeList, error)
	QueryMutationEdges() (pb.MutationEdgeList, error)
	QueryNodeMutationNodes(string) (pb.MutationNodeList, error)
	QueryNodeMutationEdges(string) (pb.MutationEdgeList, error)
	QueryNodeMutationPath(string) (pb.MutationPath, error)
	QueryDeleteAll() ([]Node, error)
	ServiceInit(string, string) (<-chan ServiceControl, error)
}
