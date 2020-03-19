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

	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib"

	_ "net/http/pprof"

	cpb "github.com/hpc/kraken/core/proto"
	rpb "github.com/hpc/kraken/modules/restapi/proto"
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

	k := core.NewKraken(*idstr, *ip, parents, lib.LoggerLevel(*llevel))
	k.Release()

	qe := core.NewQueryEngine(k.Sde.QueryChan(), k.Sme.QueryChan())
	// we'll modify ourselves a bit to get running
	self, e := qe.Read(k.Ctx.Self)
	if e != nil {
		k.Logf(lib.LLERROR, "couldn't read our own node object: %v", e)
	}

	// if we're full-state, we start restapi automatically
	if len(parents) == 0 {
		conf := &rpb.RestAPIConfig{
			Addr: *ipapi,
			Port: 3141,
		}
		any, _ := ptypes.MarshalAny(conf)
		if _, e := self.SetValue("/Services/restapi/Config", reflect.ValueOf(any)); e != nil {
			k.Logf(lib.LLERROR, "couldn't set value /Services/restapi/Config -> %+v: %v", reflect.ValueOf(any), e)
		}
		if _, e := self.SetValue("/Services/restapi/State", reflect.ValueOf(cpb.ServiceInstance_RUN)); e != nil {
			k.Logf(lib.LLERROR, "couldn't set value /Services/restapi/State -> %+v: %v", reflect.ValueOf(cpb.ServiceInstance_RUN), e)
		}
	}
	qe.Update(self)

	// Thaw if full state
	if len(parents) == 0 {
		k.Sme.Thaw()
	}

	// wait forever
	for {
		select {}
	}
}
