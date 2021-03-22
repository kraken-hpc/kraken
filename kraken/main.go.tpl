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
	"path/filepath"
	"reflect"
	"time"

	"github.com/coreos/go-systemd/daemon"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"
	"github.com/kraken-hpc/go-fork"
	"gopkg.in/yaml.v2"

	_ "net/http/pprof"

	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/ipv4"
	ipv4t "github.com/hpc/kraken/extensions/ipv4/customtypes"
	rpb "github.com/hpc/kraken/modules/restapi"
	uuid "github.com/satori/go.uuid"
)

const Version = "v1.0"

type MutationConfig struct {
	Disable bool          // completely disable the mutation
	Timeout time.Duration // override the timeout
	// we may allow overriding more things eventually
}

type ServiceInstanceConfig struct {
	Disable   bool   // disable the whole instance (for defaults)
	Module    string // name of the module this SI is based on, only needed when defining new instances
	Mutations map[string]MutationConfig
}

// A RuntimeConfig sets overrides for kraken internals once at startup
// Note: we may eventually want to convert this to a protobuf so it could be serialized to neighbors via gRPC
type RuntimeConfig struct {
	Freeze           bool
	ID               uuid.UUID // Our own node ID
	IP               net.IP    // communications IP
	IPAPI            net.IP    // IP the restapi should listen on
	LogLevel         uint
	Parent           net.IP
	NoPrefix         bool   // Disable log prefix?
	SdNotify         bool   // should we notify systemd?
	StateFile        string // Path to state file (json)
	ServiceInstances map[string]ServiceInstanceConfig
}

var flags struct {
	cfgFile  string
	freeze   bool
	idstr    string
	id       string
	ip       string
	ipapi    string
	llevel   uint
	noprefix bool
	parent   string
	prc      bool
	sdnotify bool
	state    string
	version  bool
}

var setFlags map[string]bool

func flagIsSet(f string) bool {
	_, set := setFlags[f]
	return set
}

func usageExit(format string, a ...interface{}) {
	fmt.Printf("Fatal error: "+format+"\n", a...)
	flag.PrintDefaults()
	os.Exit(1)
}

// build and populate a runtime config with defaults
func newRuntimeConfig() (rc *RuntimeConfig) {
	rc = &RuntimeConfig{
		Freeze:           false,
		ID:               uuid.FromStringOrNil("123e4567-e89b-12d3-a456-426655440000"),
		IP:               net.ParseIP("127.0.0.1"),
		IPAPI:            net.ParseIP("127.0.0.1"),
		LogLevel:         5,
		Parent:           nil,
		NoPrefix:         false,
		SdNotify:         false,
		StateFile:        "",
		ServiceInstances: map[string]ServiceInstanceConfig{},
	}
	rc.ServiceInstances["sse"] = ServiceInstanceConfig{
		Disable:   false,
		Module:    "none",
		Mutations: map[string]MutationConfig{},
	}
	// pack the default service instance config in
	for _, sis := range core.Registry.ServiceInstances {
		for name, si := range sis {
			rc.ServiceInstances[name] = ServiceInstanceConfig{
				Disable:   false,
				Module:    si.Module(),
				Mutations: map[string]MutationConfig{},
			}
		}
	}
	for si, muts := range core.Registry.Mutations {
		for name, mut := range muts {
			rc.ServiceInstances[si].Mutations[name] = MutationConfig{
				Disable: false,
				Timeout: mut.Timeout(),
			}
		}
	}
	return
}

func printBuildInfo() {
	fmt.Printf("Kraken version: %s\n%s", Version, core.BuildInfo())
}

func printRuntimeDefaults(rc *RuntimeConfig) {
	d, _ := yaml.Marshal(rc)
	fmt.Printf("---\n")
	fmt.Printf("%s\n", string(d))
}

// construct our runtime config from arguments/config files
func buildRuntimeConfig(rc *RuntimeConfig) {
	var e error

	// was a config file specified?  If so parse it.
	if flagIsSet("config") {
		var d []byte
		ext := filepath.Ext(flags.cfgFile)
		if d, e = ioutil.ReadFile(flags.cfgFile); e != nil {
			usageExit("could not read runtime configuration file: %v", e)
		}
		switch ext {
		case ".yaml", ".yml":
			if e = yaml.Unmarshal(d, rc); e != nil {
				usageExit("failed to decode YAML runtime configuration: %v", e)
			}
		case ".json":
			if e = json.Unmarshal(d, rc); e != nil {
				usageExit("failed to decode JSON runtime configuration: %v", e)
			}
		default:
			usageExit("runtime configuration file must end in .json, .yaml, or .yml, not %s", ext)
		}
	}

	// now overlay anything that was set as a flag
	// -freeze
	if flagIsSet("freeze") {
		rc.Freeze = flags.freeze
	}
	// -id
	if flagIsSet("id") || uuid.Equal(rc.ID, uuid.Nil) {
		if rc.ID, e = uuid.FromString(flags.id); e != nil {
			usageExit("provided ID is not valid: %v", e)
		}
	}
	// -ip
	if flagIsSet("ip") || rc.IP == nil {
		if rc.IP = net.ParseIP(flags.ip); rc.IP == nil {
			usageExit("provided IP is not valid: %s", flags.ip)
		}
	}
	// -ipapi
	if flagIsSet("ipapi") || rc.IPAPI == nil {
		if rc.IPAPI = net.ParseIP(flags.ipapi); rc.IP == nil {
			usageExit("provided API IP is not valid: %s", flags.ipapi)
		}
	}
	// -log
	if flagIsSet("log") || rc.LogLevel == 0 { // note: this means you can't set ll to zero in a config
		rc.LogLevel = flags.llevel
	}
	// -noprefix
	if flagIsSet("noprefix") {
		rc.NoPrefix = flags.noprefix
	}
	// -parent
	if flagIsSet("parent") {
		if rc.Parent = net.ParseIP(flags.parent); rc.Parent == nil {
			usageExit("provided Parent IP is not valid: %s", flags.parent)
		}
	}
	// -sdnotify
	if flagIsSet("sdnotify") {
		rc.SdNotify = flags.sdnotify
	}
	// -state
	if flagIsSet("state") {
		rc.StateFile = flags.state
	}
	return
}

func processSIOverrides(rc *RuntimeConfig) {
	// it's easiest if we build a flat index of service instances first
	simap := map[string]types.ServiceInstance{}
	for _, sis := range core.Registry.ServiceInstances {
		for name, si := range sis {
			simap[name] = si
		}
	}
	for name, sic := range rc.ServiceInstances {
		if si, ok := simap[name]; ok || name == "sse" {
			// si existts
			if sic.Disable {
				if name == "sse" {
					fmt.Printf("WARNING: the sse service instance cannot be disabled. Ignoring.")
				}
				// disable the existing si
				delete(core.Registry.ServiceInstances[si.Module()], name)
				// we also need to delete the mutations
				delete(core.Registry.Mutations, name)
				// it doesn't make sense to do anything else
				continue
			}
			for mutname, mutc := range sic.Mutations {
				if _, ok := core.Registry.Mutations[name]; ok {
					// si has mutations registered
					if mut, ok := core.Registry.Mutations[name][mutname]; ok {
						// si has *this* mutation registered
						if mutc.Disable {
							delete(core.Registry.Mutations[name], mutname)
						}
						mut.SetTimeout(mutc.Timeout)
					} else {
						// si doesn't have this mutaiton registered
						fmt.Printf("WARNING: runtime configuration specified mutation override for non existent mutation: %s/%s. Ignoring.\n", name, mutname)
					}
				} else {
					// si has no mutations registered
					fmt.Printf("WARNING: runtime configuration specified mutation override for non existent mutation: %s/%s. Ignoring.\n", name, mutname)
				}
			}
		} else {
			// si doesn't exist
			fmt.Printf("WARNING: runtime configuration specified a new service instance (%s).  Defining new service instances is not yet supported. Ignoring.\n", name)
		}
	}
}

func main() {
	rc := newRuntimeConfig()
	// Flags not considered part of RunningConfig
	flag.BoolVar(&flags.version, "version", false, "print version and build information and exit")
	flag.StringVar(&flags.cfgFile, "config", "", "path to a runtime configuration file")
	flag.BoolVar(&flags.prc, "printrc", false, "print the runtime config and exit (can be used as a basis for a new config)")
	// RunningConfig flags
	flag.BoolVar(&flags.freeze, "freeze", rc.Freeze, "start the SME frozen (i.e. don't try to mutate any states at startup)")
	flag.StringVar(&flags.id, "id", rc.ID.String(), "specify a UUID for this node")
	flag.StringVar(&flags.ip, "ip", rc.IP.String(), "what is my IP (for communications and listening)")
	flag.StringVar(&flags.ipapi, "ipapi", rc.IPAPI.String(), "what IP to use for the ReST API")
	flag.UintVar(&flags.llevel, "log", rc.LogLevel, "set the log level (0-9)")
	flag.BoolVar(&flags.noprefix, "noprefix", rc.NoPrefix, "don't prefix log messages with timestamps")
	flag.StringVar(&flags.parent, "parent", "", "IP adddress of parent")
	flag.BoolVar(&flags.sdnotify, "sdnotify", rc.SdNotify, "notify systemd when kraken is initialized")
	flag.StringVar(&flags.state, "state", rc.StateFile, "path to a JSON file containing initial configuration state to load")
	flag.Parse()

	// This gives us an easy way to distinguesh when a flag happened to be set to its default
	// And when a flag wasn't specified.  We give those two things differen precedence.
	// If the flag was set, it will override -cfg values, even if it's the default.
	setFlags = make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	if flags.version {
		printBuildInfo()
		os.Exit(0)
	}

	buildRuntimeConfig(rc)

	// we do this before flags.prc so any warnings get printed
	processSIOverrides(rc)

	// process info options
	if flags.prc {
		printRuntimeDefaults(rc)
		os.Exit(0)
	}

	fullStateNode := true
	if rc.Parent != nil {
		fullStateNode = false
	}

	// Create a new logger interface
	log := &core.WriterLogger{}
	log.RegisterWriter(os.Stderr)
	log.SetModule("main")
	log.SetLoggerLevel(types.LoggerLevel(rc.LogLevel))
	if rc.NoPrefix {
		log.DisablePrefix = true
	}

	// Launch as base Kraken or module?
	fork.Init()

	// Past this point we know we're not a module
	// Let's report some things about ourself
	log.Logf(types.LLNOTICE, "I am: kraken")
	log.Logf(types.LLDDEBUG, "runtime configuration: %s", func() string { d, _ := json.Marshal(rc); return string(d) }())
	log.Logf(types.LLNOTICE, "running as a %s node", func() string {
		if fullStateNode {
			return "full-state (parent)"
		}
		return "partial-state (child)"
	}())

	// Build our starting node (CFG) state based on command line arguments
	self := core.NewNodeWithID(rc.ID.String())
	selfDsc := core.NewNodeWithID(rc.ID.String())

	// Set some state defaults if we're a full state node
	if fullStateNode {
		// Enable the restapi by default
		conf := &rpb.Config{
			Addr: rc.IPAPI.String(),
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

	// Parse -state file
	if rc.StateFile != "" {
		log.Logf(types.LLINFO, "loading initial configuration state from: %s", rc.StateFile)
		data, e := ioutil.ReadFile(rc.StateFile)
		if e != nil {
			usageExit("failed to read cfg state file: %s, %v", rc.StateFile, e)
		}
		var pbs cpb.NodeList
		if e = json.Unmarshal(data, &pbs); e != nil {
			usageExit("could not parse cfg state file: %s, %v", rc.StateFile, e)
		}
		log.Logf(types.LLDEBUG, "found initial state information for %d nodes", len(pbs.Nodes))
		for _, m := range pbs.GetNodes() {
			n := core.NewNodeFromMessage(m)
			log.Logf(types.LLDDDEBUG, "got node state for node: %s", n.ID().String())
			if n.ID().EqualTo(self.ID()) {
				// we found ourself
				self.Merge(n, "")
			} else {
				// note: duplicates will cause a later failure
				nodes = append(nodes, n)
			}
		}
	}

	// Populate interface0 information based on IP (if it doesn't already exist from the state file)
	if _, e := self.GetValue("type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Ip/Ip"); e != nil {
		iface := net.Interface{}
		network := net.IPNet{}
		ifaces, e := net.Interfaces()
		if e != nil {
			usageExit("failed to get system interfaces: %s", e.Error())
		}
		for _, i := range ifaces {
			as, e := i.Addrs()
			if e != nil {
				continue
			}
			for _, a := range as {
				ip, n, _ := net.ParseCIDR(a.String())
				if ip.To4().Equal(rc.IP) {
					// this is our interface
					iface = i
					network = *n
				}
			}
		}
		if iface.Name == "" {
			usageExit("could not find interface for ip %s", rc.IP.String())
		}
		log.Logf(types.LLDEBUG, "using interface: %s", iface.Name)
		pb := &ipv4.IPv4OverEthernet_ConfiguredInterface{
			Eth: &ipv4.Ethernet{
				Iface: iface.Name,
				Mac:   &ipv4t.MAC{HardwareAddr: iface.HardwareAddr},
				Mtu:   uint32(iface.MTU),
			},
			Ip: &ipv4.IPv4{
				Ip:     &ipv4t.IP{IP: rc.IP.To4()},
				Subnet: &ipv4t.IP{IP: net.IP(network.Mask)},
			},
		}
		self.SetValue("type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken", reflect.ValueOf(pb))
	}

	parents := []string{}
	if !fullStateNode {
		parents = append(parents, rc.Parent.String())
	}
	// Launch Kraken
	k := core.NewKraken(self, parents, log)
	if len(nodes) > 0 {
		k.Ctx.SDE.InitialCfg = append(k.Ctx.SDE.InitialCfg, nodes...)
	}
	k.Ctx.SDE.InitialDsc = []types.Node{selfDsc}
	k.Release()

	// Thaw if full state and not told to freeze
	if fullStateNode && !rc.Freeze {
		k.Sme.Thaw()
	}

	// notify systemd?
	if rc.SdNotify {
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
