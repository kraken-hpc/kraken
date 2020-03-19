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
	ip4pb "github.com/hpc/kraken/extensions/IPv4/proto"
	rpb "github.com/hpc/kraken/modules/restapi/proto"
	uuid "github.com/satori/go.uuid"
)

func main() {
	// Argument parsing
	idstr := flag.String("id", "123e4567-e89b-12d3-a456-426655440000", "specify a UUID for this node")
	ip := flag.String("ip", "127.0.0.1", "what is my IP (for communications and listening)")
	ipapi := flag.String("ipapi", "127.0.0.1", "what IP to use for the ReST API")
	parent := flag.String("parent", "", "IP adddress of parent")
	llevel := flag.Int("log", 3, "set the log level (0-9)")
	flag.Parse()

	// Create a new logger interface
	log := &core.WriterLogger{}
	log.RegisterWriter(os.Stderr)
	log.SetModule("main")
	log.SetLoggerLevel(lib.LoggerLevel(*llevel))

	// Launch as base Kraken or module?
	me := filepath.Base(os.Args[0])
	log.Logf(lib.LLNOTICE, "I am: %s", me)
	if me != "kraken" {
		id := os.Getenv("KRAKEN_ID")
		module := os.Getenv("KRAKEN_MODULE")
		sock := os.Getenv("KRAKEN_SOCK")
		if m, ok := core.Registry.Modules[module]; ok {
			if _, ok := m.(lib.ModuleSelfService); ok {
				log.Logf(lib.LLNOTICE, "module entry point: %s", module)
				core.ModuleExecute(id, module, sock)
				return
			}
		}
		log.Logf(lib.LLCRITICAL, "could not start module: %s", module)
		return
	}

	// Check that the parent IP is sane
	parents := []string{}
	if len(*parent) > 0 {
		parents = append(parents, *parent)
		ip := net.ParseIP(*parent)
		if ip == nil {
			fmt.Printf("bad parent IP: %s", *parent)
			flag.PrintDefaults()
			return
		}
	}

	// check that the UUID is sane
	_, e := uuid.FromString(*idstr)
	if e != nil {
		fmt.Printf("bad UUID: %s, %v", *idstr, e)
		flag.PrintDefaults()
		return
	}

	// Build our starting node (CFG) state based on command line arguments
	self := core.NewNodeWithID(*idstr)

	{ // Enable the restapi by default
		conf := &rpb.RestAPIConfig{
			Addr: *ipapi,
			Port: 3141,
		}
		any, _ := ptypes.MarshalAny(conf)
		if _, e := self.SetValue("/Services/restapi/Config", reflect.ValueOf(any)); e != nil {
			log.Logf(lib.LLERROR, "couldn't set value /Services/restapi/Config -> %+v: %v", reflect.ValueOf(any), e)
		}
		if _, e := self.SetValue("/Services/restapi/State", reflect.ValueOf(cpb.ServiceInstance_RUN)); e != nil {
			log.Logf(lib.LLERROR, "couldn't set value /Services/restapi/State -> %+v: %v", reflect.ValueOf(cpb.ServiceInstance_RUN), e)
		}
	}

	{ // Populate interface0 information based on IP
		netIP := net.ParseIP(*ip)
		if netIP == nil {
			log.Logf(lib.LLCRITICAL, "could not parse IP address: %s", *ip)
		}
		iface := net.Interface{}
		network := net.IPNet{}
		ifaces, e := net.Interfaces()
		if e != nil {
			log.Logf(lib.LLCRITICAL, "failed to get system interfaces: %s", e.Error())
			return
		}
		for _, i := range ifaces {
			as, e := i.Addrs()
			if e != nil {
				continue
			}
			for _, a := range as {
				ip, n, _ := net.ParseCIDR(a.String())
				if ip.To4().Equal(netIP) {
					// this is our interface
					iface = i
					network = *n
				}
			}
		}
		if iface.Name == "" {
			log.Logf(lib.LLCRITICAL, "could not find interface for ip %s", *ip)
			return
		}
		log.Logf(lib.LLDEBUG, "using interface: %s", iface.Name)
		pb := &ip4pb.IPv4OverEthernet_ConfiguredInterface{
			Eth: &ip4pb.Ethernet{
				Iface: iface.Name,
				Mac:   iface.HardwareAddr,
				Mtu:   uint32(iface.MTU),
			},
			Ip: &ip4pb.IPv4{
				Ip:     netIP.To4(),
				Subnet: network.Mask,
			},
		}
		self.SetValue("type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0", reflect.ValueOf(pb))
	}

	// Launch Kraken
	k := core.NewKraken(self, parents, log)
	k.Release()

	// Thaw if full state
	if len(parents) == 0 {
		k.Sme.Thaw()
	}

	// wait forever
	for {
		select {}
	}
}
