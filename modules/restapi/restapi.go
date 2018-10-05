/* restapi.go: this module provides a simple ReST API for Kraken
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/restapi.proto

package restapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/gorilla/mux"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib"

	cpb "github.com/hpc/kraken/core/proto"
	pb "github.com/hpc/kraken/modules/restapi/proto"
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

func (r *RestAPI) Entry() {
	r.setupRouter()
	r.startServer()
}

func (r *RestAPI) Stop() { os.Exit(0) }

func (r *RestAPI) Name() string { return "github.com/hpc/kraken/modules/restapi" }

func (r *RestAPI) UpdateConfig(cfg proto.Message) (e error) {
	if rc, ok := cfg.(*pb.RestAPIConfig); ok {
		r.cfg = rc
		/* this is broken
		r.srvStop() // we just restart everything
		r.Entry()
		*/
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
		Addr: "127.0.0.1",
		Port: 3141,
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
}

func (r *RestAPI) startServer() {
	r.srv = &http.Server{
		Handler:      r.router,
		Addr:         fmt.Sprintf("%s:%d", r.cfg.Addr, r.cfg.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	r.api.Logf(lib.LLINFO, "restapi is listening on: %s\n", r.srv.Addr)
	if e := r.srv.ListenAndServe(); e != nil {
		r.api.Logf(lib.LLERROR, "%v\n", e)
	}
	r.api.Log(lib.LLNOTICE, "restapi listener stopped")
}

func (r *RestAPI) srvStop() {
	r.api.Log(lib.LLNOTICE, "restapi is shutting down listener")
	r.srv.Shutdown(context.Background())
}

/*
 * Route handlers
 */

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
	w.Write(b)
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
	w.Write(n.JSON())
}

func (r *RestAPI) readNodeDsc(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	n, e := r.api.QueryReadDsc(params["id"])
	if e != nil || n == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
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
	w.Write(b)
}

func (r *RestAPI) updateNodeDsc(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	n := core.NewNodeFromJSON(buf.Bytes())
	if n == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	nn, e := r.api.QueryUpdateDsc(n)
	if e != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(e.Error()))
		return
	}
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
