/* restapi.go: this module provides a simple ReST API for Kraken
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/restapi.proto

package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib"

	cpb "github.com/hpc/kraken/core/proto"
	pb "github.com/hpc/kraken/modules/restapi/proto"
	wpb "github.com/hpc/kraken/modules/websocket/proto"
)

var _ lib.Module = (*RestAPI)(nil)
var _ lib.ModuleSelfService = (*RestAPI)(nil)
var _ lib.ModuleWithConfig = (*RestAPI)(nil)

type RestAPI struct {
	cfg    *pb.RestAPIConfig
	api    lib.APIClient
	router *mux.Router
	srv    *http.Server
}

type GraphJson struct {
	Nodes []*cpb.MutationNode `json:"nodes"`
	Edges []*cpb.MutationEdge `json:"edges"`
}

func (r *RestAPI) Entry() {
	r.setupRouter()
	for {
		r.startServer()
	}
}

func (r *RestAPI) Stop() { os.Exit(0) }

func (r *RestAPI) Name() string { return "github.com/hpc/kraken/modules/restapi" }

func (r *RestAPI) UpdateConfig(cfg proto.Message) (e error) {
	if rc, ok := cfg.(*pb.RestAPIConfig); ok {
		r.cfg = rc
		if r.srv != nil {
			r.srvStop() // we just stop, entry will (re)start
		}
		return
	}
	return fmt.Errorf("wrong config type")
}

func (r *RestAPI) Init(api lib.APIClient) {
	r.api = api
	if r.cfg == nil {
		r.cfg = r.NewConfig().(*pb.RestAPIConfig)
	}
}

func (r *RestAPI) NewConfig() proto.Message {
	return &pb.RestAPIConfig{
		Addr:      "127.0.0.1",
		Port:      3141,
		WsPortUrl: "type.googleapis.com/proto.RestAPIConfig/Port",
	}
}

func (r *RestAPI) ConfigURL() string {
	a, _ := ptypes.MarshalAny(r.NewConfig())
	return a.GetTypeUrl()
}

func (r *RestAPI) setupRouter() {
	r.router = mux.NewRouter()
	r.router.HandleFunc("/cfg/nodes", r.readAll).Methods("GET")
	r.router.HandleFunc("/cfg/nodes", r.updateMulti).Methods("PUT")
	r.router.HandleFunc("/cfg/nodes", r.createMulti).Methods("POST")
	r.router.HandleFunc("/dsc/nodes", r.readAllDsc).Methods("GET")
	r.router.HandleFunc("/dsc/nodes", r.updateMultiDsc).Methods("PUT")
	r.router.HandleFunc("/cfg/node/{id}", r.readNode).Methods("GET")
	r.router.HandleFunc("/cfg/node", r.createNode).Methods("POST")
	r.router.HandleFunc("/cfg/node/{id}", r.createNode).Methods("POST")
	r.router.HandleFunc("/cfg/node/{id}", r.readNode).Methods("GET")
	r.router.HandleFunc("/cfg/node/{id}", r.deleteNode).Methods("DELETE")
	r.router.HandleFunc("/dsc/node/{id}", r.readNodeDsc).Methods("GET")
	r.router.HandleFunc("/cfg/node", r.updateNode).Methods("PUT")
	r.router.HandleFunc("/cfg/node/{id}", r.updateNode).Methods("PUT")
	r.router.HandleFunc("/dsc/node", r.updateNodeDsc).Methods("PUT")
	r.router.HandleFunc("/dsc/node/{id}", r.updateNodeDsc).Methods("PUT")
	r.router.HandleFunc("/graph/json", r.readGraphJSON).Methods("GET")
	r.router.HandleFunc("/graph/node/{id}/json", r.readNodeGraphJSON).Methods("GET")
	r.router.HandleFunc("/enumerables", r.getAllEnums).Methods("GET")
	r.router.HandleFunc("/ws", r.webSocketRedirect).Methods("GET")
}

func (r *RestAPI) startServer() {
	r.srv = &http.Server{
		Handler: handlers.CORS(
			handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}),
			handlers.AllowedOrigins([]string{"*"}),
			handlers.AllowedMethods([]string{"PUT", "GET", "POST", "DELETE"}),
		)(r.router),
		Addr:         fmt.Sprintf("%s:%d", r.cfg.Addr, r.cfg.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	r.api.Logf(lib.LLINFO, "restapi is listening on: %s\n", r.srv.Addr)
	if e := r.srv.ListenAndServe(); e != nil {
		if e != http.ErrServerClosed {
			r.api.Logf(lib.LLNOTICE, "http stopped: %v\n", e)
		}
	}
	r.api.Log(lib.LLNOTICE, "restapi listener stopped")
}

func (r *RestAPI) srvStop() {
	r.api.Log(lib.LLDEBUG, "restapi is shutting down listener")
	r.srv.Shutdown(context.Background())
}

/*
 * Route handlers
 */

func (r *RestAPI) webSocketRedirect(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	host, _, _ := net.SplitHostPort(req.Host)
	nself, _ := r.api.QueryRead(r.api.Self().String())
	var wsConfig wpb.WebSocketConfig
	if err := proto.Unmarshal(nself.GetService("websocket").Config().GetValue(), &wsConfig); err != nil {
		fmt.Printf("error during marshaling!: %v\n", err)
	} else {
		fmt.Printf("unmarshalling was successful!: %+v\n", wsConfig)
	}
	wsPort := wsConfig.GetPort()
	if wsPort == 0 {
		r.api.Logf(lib.LLERROR, "Could not get WebSocket port. Is websocket module running?")
		return
	}
	json := fmt.Sprintf(`{"websocket": {"host": "%v", "port": "%v", "url": "%v"}}`, host, wsPort, "/ws")
	var resp = []byte(json)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(resp)
	r.api.Logf(lib.LLDEBUG, "Wrote back response of: %v", json)
}

func (r *RestAPI) readAll(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	ns, e := r.api.QueryReadAll()
	if e != nil {
		w.Write([]byte(e.Error()))
		return
	}
	var rsp cpb.NodeList
	for _, n := range ns {
		rsp.Nodes = append(rsp.Nodes, n.Message().(*cpb.Node))
	}
	b, _ := core.MarshalJSON(&rsp)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(b)
}

func (r *RestAPI) getAllEnums(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	nself, e := r.api.QueryRead(r.api.Self().String())
	if e != nil {
		w.Write([]byte(e.Error()))
		return
	}

	type extension struct {
		Name    string           `json:"name"`
		Url     string           `json:"url"`
		Options map[int32]string `json:"options"` // 0:"NONE"
	}
	var extSlice []extension
	extMap := nself.GetExtensions()

	for k, v := range extMap {
		properties := proto.GetProperties(reflect.ValueOf(v).Elem().Type())
		for _, p := range properties.Prop {
			enumValueMap := proto.EnumValueMap(p.Enum)
			if len(enumValueMap) > 0 {
				enumOptions := make(map[int32]string, len(enumValueMap))
				for key, val := range enumValueMap {
					enumOptions[val] = key
				}

				enum := extension{
					Name:    p.Enum,
					Url:     lib.URLPush(k, p.Enum),
					Options: enumOptions,
				}
				extSlice = append(extSlice, enum)
			}
		}
	}

	physState := extension{
		Name:    "PhysState",
		Url:     "physState",
		Options: cpb.Node_PhysState_name,
	}
	extSlice = append(extSlice, physState)

	runState := extension{
		Name:    "RunState",
		Url:     "runState",
		Options: cpb.Node_RunState_name,
	}
	extSlice = append(extSlice, runState)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	jExt, e := json.Marshal(extSlice)
	if e != nil {
		r.api.Logf(lib.LLERROR, "error marshalling json: %v", e)
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}
	w.Write(jExt)
}

func (r *RestAPI) readAllDsc(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	ns, e := r.api.QueryReadAllDsc()
	if e != nil {
		w.Write([]byte(e.Error()))
		return
	}
	var rsp cpb.NodeList
	for _, n := range ns {
		rsp.Nodes = append(rsp.Nodes, n.Message().(*cpb.Node))
	}
	b, _ := core.MarshalJSON(&rsp)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(b)
}

func (r *RestAPI) readNode(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	n, e := r.api.QueryRead(params["id"])
	if e != nil || n == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(n.JSON())
}

func (r *RestAPI) readNodeGraphJSON(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)

	nodes, e := r.api.QueryNodeMutationNodes(params["id"])
	if e != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}
	edges, e := r.api.QueryNodeMutationEdges(params["id"])
	if e != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}

	path, e := r.api.QueryNodeMutationPath(params["id"])
	if e != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}

	// Convert edges and nodes slice to maps
	nodesMap := make(map[string]*cpb.MutationNode)
	edgesMap := make(map[string]*cpb.MutationEdge)
	for _, mn := range nodes.MutationNodeList {
		nodesMap[mn.Id] = mn
	}
	for _, me := range edges.MutationEdgeList {
		edgesMap[me.Id] = me
	}

	red := "#e74c3c"
	lightGreen := "#89CA78"
	darkGreen := "#62a053"
	lightGrey := "#bfbfbf"
	darkGrey := "#848484"

	// Set all nodes and edges to the default grey color first
	dec := &cpb.EdgeColor{
		Color:     darkGrey,
		Highlight: darkGrey,
		Inherit:   false,
	}

	dnc := &cpb.NodeColor{
		Background: lightGrey,
		Border:     darkGrey,
	}

	for _, me := range edgesMap {
		me.Color = dec
	}

	for _, mn := range nodesMap {
		mn.Color = dnc
	}

	// Set special nodes and edges to green or red
	for i, me := range path.Chain {
		if int64(i) != path.Cur {
			ec := &cpb.EdgeColor{
				Color:     lightGreen,
				Highlight: lightGreen,
				Inherit:   false,
			}
			nc := &cpb.NodeColor{
				Background: lightGreen,
				Border:     darkGreen,
			}
			edgesMap[me.Id].Color = ec
			nodesMap[me.To].Color = nc
			nodesMap[me.From].Color = nc
		} else {
			ec := &cpb.EdgeColor{}
			tnc := &cpb.NodeColor{}
			fnc := &cpb.NodeColor{
				Background: lightGreen,
				Border:     darkGreen,
			}
			if path.Cmplt {
				ec = &cpb.EdgeColor{
					Color:     lightGreen,
					Highlight: lightGreen,
					Inherit:   false,
				}
				tnc = &cpb.NodeColor{
					Background: lightGreen,
					Border:     red,
				}
			} else {
				ec = &cpb.EdgeColor{
					Color:     red,
					Highlight: red,
					Inherit:   false,
				}
				tnc = &cpb.NodeColor{
					Background: lightGreen,
					Border:     darkGreen,
				}
			}
			edgesMap[me.Id].Color = ec
			nodesMap[me.To].Color = tnc
			nodesMap[me.From].Color = fnc
		}
	}

	graph := GraphJson{
		Nodes: nodes.MutationNodeList,
		Edges: edges.MutationEdgeList,
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	jsonGraph, e := json.Marshal(graph)
	if e != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}
	r.api.Logf(lib.LLDDDEBUG, "Node filtered graph: %v", string(jsonGraph))
	w.Write([]byte(string(jsonGraph)))
}

func (r *RestAPI) readGraphJSON(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	nodes, e := r.api.QueryMutationNodes()
	if e != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}

	edges, e := r.api.QueryMutationEdges()
	if e != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}

	graph := GraphJson{
		Nodes: nodes.MutationNodeList,
		Edges: edges.MutationEdgeList,
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	jsonGraph, e := json.Marshal(graph)
	if e != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}
	r.api.Logf(lib.LLDDDEBUG, "Graph: %v", string(jsonGraph))
	w.Write([]byte(string(jsonGraph)))
}

func (r *RestAPI) readNodeDsc(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	n, e := r.api.QueryReadDsc(params["id"])
	if e != nil || n == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(n.JSON())
}

func (r *RestAPI) updateNode(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	n := core.NewNodeFromJSON(buf.Bytes())
	if n == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	nn, e := r.api.QueryUpdate(n)
	if e != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(e.Error()))
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(nn.JSON())
}

func (r *RestAPI) updateMulti(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	var pbs cpb.NodeList
	e := core.UnmarshalJSON(buf.Bytes(), &pbs)
	if e != nil {
		r.api.Logf(lib.LLERROR, "unmarshal JSON error: %v", e)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var rsp cpb.NodeList
	for _, m := range pbs.GetNodes() {
		n := core.NewNodeFromMessage(m)
		nn, e := r.api.QueryUpdate(n)
		if e == nil {
			rsp.Nodes = append(rsp.Nodes, nn.Message().(*cpb.Node))
		}
	}
	b, _ := core.MarshalJSON(&rsp)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(b)
}

func (r *RestAPI) updateNodeDsc(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	n := core.NewNodeFromJSON(buf.Bytes())
	if n == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("n is nil"))
		return
	}
	nn, e := r.api.QueryUpdateDsc(n)
	if e != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(e.Error()))
		return
	}
	// w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(nn.JSON())
}

func (r *RestAPI) updateMultiDsc(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	var pbs cpb.NodeList
	e := core.UnmarshalJSON(buf.Bytes(), &pbs)
	if e != nil {
		r.api.Logf(lib.LLERROR, "unmarshal JSON error: %v", e)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var rsp cpb.NodeList
	for _, m := range pbs.GetNodes() {
		n := core.NewNodeFromMessage(m)
		nn, e := r.api.QueryUpdateDsc(n)
		if e == nil {
			rsp.Nodes = append(rsp.Nodes, nn.Message().(*cpb.Node))
		}
	}
	b, _ := core.MarshalJSON(&rsp)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(b)
}

func (r *RestAPI) deleteNode(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	n, e := r.api.QueryDelete(params["id"])
	if e != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(e.Error()))
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(n.JSON())
}

func (r *RestAPI) createNode(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	n := core.NewNodeFromJSON(buf.Bytes())
	if n == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	nn, e := r.api.QueryCreate(n)
	if e != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(e.Error()))
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(nn.JSON())
}

func (r *RestAPI) createMulti(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	var pbs cpb.NodeList
	e := core.UnmarshalJSON(buf.Bytes(), &pbs)
	if e != nil {
		r.api.Logf(lib.LLERROR, "unmarshal JSON error: %v", e)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var rsp cpb.NodeList
	for _, m := range pbs.GetNodes() {
		n := core.NewNodeFromMessage(m)
		nn, e := r.api.QueryCreate(n)
		if e == nil {
			rsp.Nodes = append(rsp.Nodes, nn.Message().(*cpb.Node))
		}
	}
	b, _ := core.MarshalJSON(&rsp)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(b)
}

func init() {
	module := &RestAPI{}
	core.Registry.RegisterModule(module)
	si := core.NewServiceInstance(
		"restapi",
		module.Name(),
		module.Entry,
		nil,
	)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{
		si.ID(): si,
	})
}
