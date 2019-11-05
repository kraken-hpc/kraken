/* main.go: provides the main entry-point for Kraken
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"

	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib"

	_ "net/http/pprof"

	"github.com/golang/protobuf/ptypes"
	pb "github.com/hpc/kraken/core/proto"
	pbr "github.com/hpc/kraken/modules/restapi/proto"
	uuid "github.com/satori/go.uuid"
)

func main() {
	me := filepath.Base(os.Args[0])
	fmt.Printf("I am: %s\n", me)
	if me != "kraken" {
		id := os.Getenv("KRAKEN_ID")
		module := os.Getenv("KRAKEN_MODULE")
		sock := os.Getenv("KRAKEN_SOCK")
		if m, ok := core.Registry.Modules[module]; ok {
			if _, ok := m.(lib.ModuleSelfService); ok {
				fmt.Printf("module entry point: %s\n", module)
				core.ModuleExecute(id, module, sock)
				return
			}
		}
		fmt.Printf("could not start module: %s\n", module)
		return
	}
	idstr := flag.String("id", "123e4567-e89b-12d3-a456-426655440000", "specify a UUID for this node")
	ip := flag.String("ip", "127.0.0.1", "what is my IP (for communications and listening)")
	ipapi := flag.String("ipapi", "127.0.0.1", "what IP to use for the ReST API")
	parent := flag.String("parent", "", "IP adddress of parent")
	llevel := flag.Int("log", 3, "set the log level (0-9)")
	flag.Parse()

	parents := []string{}
	if len(*parent) > 0 {
		parents = append(parents, *parent)
		ip := net.ParseIP(*parent)
		if ip == nil {
			fmt.Printf("bad parent IP: %s\n", *parent)
			flag.PrintDefaults()
			return
		}
	}

	_, e := uuid.FromString(*idstr)
	if e != nil {
		fmt.Printf("bad UUID: %s, %v\n", *idstr, e)
		flag.PrintDefaults()
		return
	}

	muts := make(map[string]lib.StateMutation)
	discs := make(map[string]map[string]reflect.Value)

	k := core.NewKraken(*idstr, *ip, parents, lib.LoggerLevel(*llevel))

	// inject service instances
	// & declare mutations/discoveries for each
	// note: this can be overridden by a parent
	for i := range core.Registry.ServiceInstances {
		for _, si := range core.Registry.ServiceInstances[i] {

			stateStr := lib.URLPush(lib.URLPush("/Services", si.ID()), "State")

			makeMut := func(a pb.ServiceInstance_ServiceState, b pb.ServiceInstance_ServiceState) lib.StateMutation {
				return core.NewStateMutation(
					map[string][2]reflect.Value{
						stateStr: [2]reflect.Value{
							reflect.ValueOf(a),
							reflect.ValueOf(b),
						},
					},
					map[string]reflect.Value{
						"/RunState": reflect.ValueOf(pb.Node_SYNC),
						"/PhysState": reflect.ValueOf(pb.Node_POWER_ON),
					},
					map[string]reflect.Value{},
					lib.StateMutationContext_SELF,
					0,
					[3]string{},
				)
			}
			mutUrls := make(map[string]bool)
			for i := range core.Registry.Mutations {
				for _, m := range core.Registry.Mutations[i] {
					for u := range m.Mutates() {
						mutUrls[u] = true
					}
					for u := range m.Requires() {
						mutUrls[u] = true
					}
					for u := range m.Excludes() {
						mutUrls[u] = true
					}
				}
			}
			if _, ok := mutUrls[stateStr]; ok {
				muts[si.ID()+"UkToInit"] = makeMut(pb.ServiceInstance_UNKNOWN, pb.ServiceInstance_INIT)
				muts[si.ID()+"StopToInit"] = makeMut(pb.ServiceInstance_STOP, pb.ServiceInstance_INIT)
				muts[si.ID()+"InitToRun"] = makeMut(pb.ServiceInstance_INIT, pb.ServiceInstance_RUN)
				muts[si.ID()+"RunToStop"] = makeMut(pb.ServiceInstance_RUN, pb.ServiceInstance_STOP)
			}
			discs[stateStr] = map[string]reflect.Value{
				"Stop": reflect.ValueOf(pb.ServiceInstance_STOP),
				"Init": reflect.ValueOf(pb.ServiceInstance_INIT),
				"Run":  reflect.ValueOf(pb.ServiceInstance_RUN),
			}
		}
	}
	core.Registry.RegisterMutations(k, muts)
	core.Registry.RegisterDiscoverable(k, discs)
	stateToVID := map[lib.ServiceState]string{
		lib.Service_UNKNOWN: "Unknown",
		lib.Service_STOP:    "Stop",
		lib.Service_RUN:     "Run",
	}
	emitDiscoveries := func(ds map[string]lib.ServiceState) {
		vs := []lib.Event{}
		for d := range ds {
			stateStr := lib.URLPush(lib.URLPush("/Services", d), "State")
			url := lib.NodeURLJoin(k.Ctx.Self.String(), stateStr)
			v := core.NewEvent(
				lib.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{
					ID:      k.ID(),
					URL:     url,
					ValueID: stateToVID[ds[d]],
				},
			)
			vs = append(vs, v)
		}
		k.Emit(vs)
	}

	k.Release()
	qe := core.NewQueryEngine(k.Sde.QueryChan(), k.Sme.QueryChan())
	// we'll modify ourselves a bit to get running
	self, _ := qe.Read(k.Ctx.Self)

	// if we're full-state, we start restapi automatically
	if len(parents) == 0 {
		restapi := self.GetService("restapi")
		cfg := &pbr.RestAPIConfig{
			Addr: *ipapi,
			Port: 3141,
		}
		any, _ := ptypes.MarshalAny(cfg)
		restapi.SetState(lib.Service_RUN)
		restapi.UpdateConfig(any)
	}
	qe.Update(self)
	self, _ = qe.Read(k.Ctx.Self)
	emitDiscoveries(k.Ctx.Services.SyncNode(self))

	echan := make(chan lib.Event)
	sclist_re, _ := regexp.Compile(".*Services.*")
	// OK, we need to manage services now
	// We make an event listener for service changes for us
	sclist := core.NewEventListener(
		"Kraken",
		lib.Event_STATE_CHANGE,
		func(v lib.Event) bool {
			node, url := lib.NodeURLSplit(v.URL())
			if !core.NewNodeID(node).Equal(k.Ctx.Self) {
				// we only care about ourself
				return false
			}
			if sclist_re.Match([]byte(url)) {
				return true
			}
			return false
		},
		func(v lib.Event) error {
			return core.ChanSender(v, echan)
		})

	// subscribe our listener
	k.Ctx.SubChan <- sclist

	// Thaw if full state
	if len(parents) == 0 {
		k.Sme.Thaw()
	}

	// wait forever
	for {
		select {
		case v := <-echan:
			_, url := lib.NodeURLSplit(v.URL())
			if v.Type() == lib.Event_STATE_CHANGE && sclist_re.Match([]byte(url)) {
				k.Logf(lib.LLDEBUG, "got event, need to update services: %v\n", v)
				me, e := qe.Read(k.Ctx.Self)
				if e != nil {
					k.Logf(lib.LLERROR, "I seem to have forgotten myself!")
					panic("amnesia")
				}
				emitDiscoveries(k.Ctx.Services.SyncNode(me))
			}
			break
		}
	}
}
