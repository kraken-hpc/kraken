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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"reflect"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"

	_ "net/http/pprof"

	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/IPv4"
	rpb "github.com/hpc/kraken/modules/restapi"
	uuid "github.com/satori/go.uuid"
)

func main() {
	// Argument parsing
	idstr := flag.String("id", "123e4567-e89b-12d3-a456-426655440000", "specify a UUID for this node")
	ip := flag.String("ip", "127.0.0.1", "what is my IP (for communications and listening)")
	ipapi := flag.String("ipapi", "127.0.0.1", "what IP to use for the ReST API")
	parent := flag.String("parent", "", "IP adddress of parent")
	llevel := flag.Int("log", 3, "set the log level (0-9)")
	sdnotify := flag.Bool("sdnotify", false, "notify systemd when kraken is initialized")
	noprefix := flag.Bool("noprefix", true, "don't prefix log messages with timestamps")
	cfg := flag.String("cfg", "", "path to a JSON file containing initial configuration state to load")
	freeze := flag.Bool("freeze", false, "start the SME frozen (i.e. don't try to mutate any states at startup)")
	flag.Parse()

	// This gives us an easy way to distinguesh when a flag happened to be set to its default
	// And when a flag wasn't specified.  We give those two things differen precedence.
	// If the flag was set, it will override -cfg values, even if it's the default.
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	// Create a new logger interface
	log := &core.WriterLogger{}
	log.RegisterWriter(os.Stderr)
	log.SetModule("main")
	log.SetLoggerLevel(types.LoggerLevel(*llevel))
	if *noprefix {
		log.DisablePrefix = true
	}

	// Launch as base Kraken or module?
	//me := filepath.Base(os.Args[0])

	id := os.Getenv("KRAKEN_ID")
	if id == "" {
		id = "kraken"
	}
	log.Logf(types.LLNOTICE, "I am: %s", id)

	if id != "kraken" {
		module := os.Getenv("KRAKEN_MODULE")
		sock := os.Getenv("KRAKEN_SOCK")
		if m, ok := core.Registry.Modules[module]; ok {
			if _, ok := m.(types.ModuleSelfService); ok {
				log.Logf(types.LLNOTICE, "module entry point: %s", module)
				core.ModuleExecute(id, module, sock)
				return
			}
		}
		log.Logf(types.LLCRITICAL, "could not start module: %s", module)
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
	selfDsc := core.NewNodeWithID(*idstr)

	// Set some defaults if we're a full state node
	if len(parents) == 0 {
		// Enable the restapi by default
		conf := &rpb.RestAPIConfig{
			Addr: *ipapi,
			Port: 3141,
		}
		any, _ := ptypes.MarshalAny(conf)
		if _, e := self.SetValue("/Services/restapi/Config", reflect.ValueOf(any)); e != nil {
			log.Logf(types.LLERROR, "couldn't set value /Services/restapi/Config -> %+v: %v", reflect.ValueOf(any), e)
		}
		if _, e := self.SetValue("/Services/restapi/State", reflect.ValueOf(cpb.ServiceInstance_RUN)); e != nil {
			log.Logf(types.LLERROR, "couldn't set value /Services/restapi/State -> %+v: %v", reflect.ValueOf(cpb.ServiceInstance_RUN), e)
		}

		// Set our run/phys states.  If we're full state these are implicit
		self.SetValue("/PhysState", reflect.ValueOf(cpb.Node_POWER_ON))
		selfDsc.SetValue("/PhysState", reflect.ValueOf(cpb.Node_POWER_ON))
		self.SetValue("/RunState", reflect.ValueOf(cpb.Node_SYNC))
		selfDsc.SetValue("/RunState", reflect.ValueOf(cpb.Node_SYNC))
	}

	nodes := []types.Node{}

	// Parse -cfg file
	if *cfg != "" {
		log.Logf(types.LLINFO, "loading initial configuration state from: %s", *cfg)
		data, e := ioutil.ReadFile(*cfg)
		if e != nil {
			fmt.Printf("failed to read cfg state file: %s, %v", *cfg, e)
			flag.PrintDefaults()
			return
		}
		var pbs cpb.NodeList
		if e = json.Unmarshal(data, &pbs); e != nil {
			fmt.Printf("could not parse cfg state file: %s, %v", *cfg, e)
			flag.PrintDefaults()
			return
		}
		log.Logf(types.LLDEBUG, "found initial state information for %d nodes", len(pbs.Nodes))
		for _, m := range pbs.GetNodes() {
			n := core.NewNodeFromMessage(m)
			log.Logf(types.LLDDDEBUG, "got node state for node: %s", n.ID().String())
			if n.ID().Equal(self.ID()) {
				// we found ourself
				self.Merge(n, "")
			} else {
				// note: duplicates will cause a later failure
				nodes = append(nodes, n)
			}
		}
	}

	// Populate interface0 information based on IP (iff cfg wasn't specified, or ip was explicitly specified)
	if ok, _ := setFlags["ip"]; *cfg == "" || ok {
		netIP := net.ParseIP(*ip)
		if netIP == nil {
			log.Logf(types.LLCRITICAL, "could not parse IP address: %s", *ip)
		}
		iface := net.Interface{}
		network := net.IPNet{}
		ifaces, e := net.Interfaces()
		if e != nil {
			log.Logf(types.LLCRITICAL, "failed to get system interfaces: %s", e.Error())
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
			log.Logf(types.LLCRITICAL, "could not find interface for ip %s", *ip)
			return
		}
		log.Logf(types.LLDEBUG, "using interface: %s", iface.Name)
		pb := &IPv4.IPv4OverEthernet_ConfiguredInterface{
			Eth: &IPv4.Ethernet{
				Iface: iface.Name,
				Mac:   iface.HardwareAddr,
				Mtu:   uint32(iface.MTU),
			},
			Ip: &IPv4.IPv4{
				Ip:     netIP.To4(),
				Subnet: network.Mask,
			},
		}
		self.SetValue("type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/0", reflect.ValueOf(pb))
	}

	// Launch Kraken
	k := core.NewKraken(self, parents, log)
	if len(nodes) > 0 {
		k.Ctx.SDE.InitialCfg = append(k.Ctx.SDE.InitialCfg, nodes...)
	}
	k.Ctx.SDE.InitialDsc = []types.Node{selfDsc}
	k.Release()

	// Thaw if full state and not told to freeze
	if len(parents) == 0 || !*freeze {
		k.Sme.Thaw()
	}

	// notify systemd?
	if *sdnotify {
		sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
		if err != nil {
			log.Logf(types.LLCRITICAL, "failed to send sd_notify: %v", err)
		} else if !sent {
			log.Logf(types.LLWARNING, "sdnotify was requested, but notification is not supported")
		} else {
			log.Logf(types.LLNOTICE, "successfuly sent sdnotify ready")
		}
	}

	// wait forever
	for {
		select {}
	}
}
