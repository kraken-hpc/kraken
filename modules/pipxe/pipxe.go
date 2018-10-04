/* pipxe.go: provides PXE-boot capabilities for Raspberry Pis
 *           this manages both DHCP and TFTP services.
 *           It incorperates some hacks to get the Rpi3B to boot consistently.
 *			 If <file> doesn't exist, but <file>.tpl does, tftp will fill it as as template.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/pipxe.proto

package pipxe

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/krolaw/dhcp4"
	"github.com/krolaw/dhcp4/conn"
	"github.com/mdlayher/arp"
	"github.com/pin/tftp"

	"github.com/golang/protobuf/proto"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/IPv4"
	rpipb "github.com/hpc/kraken/extensions/RPi3/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/pipxe/proto"
)

const (
	PxeURL      = "type.googleapis.com/proto.RPi3/Pxe"
	SrvStateURL = "/Services/pipxe/State"
)

type pxmut struct {
	f       rpipb.RPi3_PXE
	t       rpipb.RPi3_PXE
	reqs    map[string]reflect.Value
	timeout string
}

var muts = map[string]pxmut{
	"NONEtoWAIT": pxmut{
		f:       rpipb.RPi3_NONE,
		t:       rpipb.RPi3_WAIT,
		reqs:    reqs,
		timeout: "3s",
	},
	/* special case has to be made manually b/c it's a double mutation
	"WAITtoINIT": pxmut{ // this one is doesn't do any work, but provides a timeout
		f:       rpipb.RPi3_WAIT,
		t:       rpipb.RPi3_INIT,
		reqs:    reqs,
		timeout: "20s",
	}, */
	"INITtoCOMP": pxmut{
		f: rpipb.RPi3_INIT,
		t: rpipb.RPi3_COMP,
		reqs: map[string]reflect.Value{
			"/Arch":      reflect.ValueOf("aarch64"),
			"/Platform":  reflect.ValueOf("rpi3"),
			"/PhysState": reflect.ValueOf(cpb.Node_POWER_ON),
			"/RunState":  reflect.ValueOf(cpb.Node_SYNC),
		},
		timeout: "30s",
	},
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Arch":      reflect.ValueOf("aarch64"),
	"/Platform":  reflect.ValueOf("rpi3"),
	"/PhysState": reflect.ValueOf(cpb.Node_POWER_ON),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

/* we use channels and a node manager rather than locking
   to make our node store safe.  This is a simpple query
   language for that service */

type nodeQueryBy string

const (
	queryByIP  nodeQueryBy = "IP"
	queryByMAC nodeQueryBy = "MAC"
)

//////////////////
// PiPXE Object /
////////////////

// PiPXE provides PXE-boot capabilities for Raspberry Pis
type PiPXE struct {
	api   lib.APIClient
	cfg   *pb.PiPXEConfig
	mchan <-chan lib.Event
	dchan chan<- lib.Event

	selfIP net.IP

	// for maintaining our list of currently booting nodes

	mutex  sync.RWMutex
	nodeBy map[nodeQueryBy]map[string]lib.Node
}

/*
 * service starters
 */

// ServeDHCP is the main handler for new DHCP packets
func (px *PiPXE) ServeDHCP(p dhcp4.Packet, t dhcp4.MessageType, o dhcp4.Options) (d dhcp4.Packet) {
	// ignore if this doesn't appear to be a Pi
	if string([]rune(strings.ToLower(p.CHAddr().String())[0:8])) != "b8:26:eb" {
		return
	}
	switch t {
	case dhcp4.Discover:
		hardwareAddr := p.CHAddr()
		fmt.Printf("got DHCP discover from %s\n", hardwareAddr.String())
		n := px.NodeGet(queryByMAC, hardwareAddr.String())
		if n == nil {
			fmt.Printf("ignoring DHCP discover from unknown %s\n", hardwareAddr.String())
			return
		}
		v, e := n.GetValue(px.cfg.IpUrl)
		if e != nil {
			fmt.Printf("node does not have an IP in state %s\n", hardwareAddr.String())
			return
		}
		ip := IPv4.BytesToIP(v.Bytes())
		fmt.Printf("sending DHCP offer of %s to %s\n", ip.String(), hardwareAddr.String())
		d = dhcp4.ReplyPacket(
			p,
			dhcp4.Offer,
			px.selfIP.To4(),
			ip,
			time.Minute*5, // make configurable?
			//h.options.SelectOrderOrAll(o[dhcp4.OptionParameterRequestList]),
			o.SelectOrderOrAll(nil),
		)
		// we discover PXE INIT and RunState INIT
		url1 := lib.NodeURLJoin(n.ID().String(), PxeURL)
		ev1 := core.NewEvent(
			lib.Event_DISCOVERY,
			url1,
			core.DiscoveryEvent{
				Module:  px.Name(),
				URL:     url1,
				ValueID: "INIT",
			},
		)
		url2 := lib.NodeURLJoin(n.ID().String(), "/RunState")
		ev2 := core.NewEvent(
			lib.Event_DISCOVERY,
			url1,
			core.DiscoveryEvent{
				Module:  px.Name(),
				URL:     url2,
				ValueID: "NODE_INIT",
			},
		)
		px.dchan <- ev1
		px.dchan <- ev2
		//d.AddOption(dhcp4.OptionHostName, []byte(l.hostname))
		return
	case dhcp4.Request: /* we shoudln't ever get Requests
		si.Log.Logf(kraken.LLINFO, "got DHCP request for %s", p.CHAddr().String())
		d = dhcp4.ReplyPacket(
			p,
			dhcp4.NAK,
			si.Config.(*Config).IP.To4(),
			p.CIAddr(),
			si.Config.(*Config).LeaseDuration,
			si.Config.(*Config).Options.SelectOrderOrAll(nil),
		)
		if server, ok := o[dhcp4.OptionServerIdentifier]; ok && !net.IP(server).Equal(si.Config.(*Config).IP) {
			si.Log.Log(kraken.LLDEBUG, "sending a NAK because wrong serverID")
			return
		}
		reqIP := net.IP(o[dhcp4.OptionRequestedIPAddress])
		if reqIP == nil {
			reqIP = net.IP(p.CIAddr())
		}
		if len(reqIP) != 4 || reqIP.Equal(net.IPv4zero) {
			si.Log.Log(kraken.LLDEBUG, "sending a NAK because misformed request")
			return
		}
		hardwareAddr := p.CHAddr()
		l, e := h.leases[hardwareAddr.String()]
		if !e || !l.ip.IP.Equal(reqIP) {
			si.Log.Log(kraken.LLDEBUG, "sending a NAK because IP mismatch")
			return
		}
		l.Renew(si.Config.(*Config).LeaseDuration)
		si.Log.Logf(kraken.LLDEBUG, "send DHCP ack of %s for %s", l.ip.String(), p.CHAddr().String())
		d = dhcp4.ReplyPacket(
			p,
			dhcp4.ACK,
			si.Config.(*Config).IP.To4(),
			l.ip.IP,
			si.Config.(*Config).LeaseDuration,
			//h.options.SelectOrderOrAll(o[dhcp4.OptionParameterRequestList]),
			options.SelectOrderOrAll(nil),
		)
		d.AddOption(dhcp4.OptionHostName, []byte(l.hostname))
		return */
	case dhcp4.Release: // don't need these either
	default:
		fmt.Println("Unhandled DHCP packet.")
	}
	return nil
}

// StartDHCP starts up the DHCP service
func (px *PiPXE) StartDHCP(iface string, ip net.IP) {
	options := make(dhcp4.Options)
	options[dhcp4.OptionSubnetMask] = net.ParseIP("255.255.255.0").To4()
	options[dhcp4.OptionRouter] = ip.To4()
	/* Uncomment for standard PXE
	options[dhcp4.OptionNameServer] = ip.To4()
	h.options[dhcp4.OptionTFTPServerName] = conf.Ip.To4()
	h.options[dhcp4.OptionBootFileName] = []byte("pxelinux.0")
	options[dhcp4.OptionDomainNameServer] = ip.To4()
	options[dhcp4.OptionDomainName] = []byte(si.Config.(*Config).Domain)
	*/
	options[dhcp4.OptionVendorClassIdentifier] = []byte("PXEClient")
	options[dhcp4.OptionVendorSpecificInformation] = []byte{
		0x6, 0x1, 0x3, 0xa, 0x4, 0x0, 0x50, 0x58, 0x45, 0x9, 0x14, 0x0, 0x0, 0x11, 0x52, 0x61,
		0x73, 0x70, 0x62, 0x65, 0x72, 0x72, 0x79, 0x20, 0x50, 0x69, 0x20, 0x42, 0x6f, 0x6f, 0x74, 0xff}

	c, e := conn.NewUDP4FilterListener(iface, ":67")
	if e != nil {
		fmt.Printf("%v: %s\n", e, iface)
		return
	}
	fmt.Printf("started DHCP listener on: %s\n", iface)
	buffer := make([]byte, 1500)
	netIf, _ := net.InterfaceByName(iface)
	ac, e := arp.Dial(netIf)
	if e != nil {
		fmt.Printf("%v\n", e)
		return
	}

	for {
		n, addr, e := c.ReadFrom(buffer)
		if e != nil {
			fmt.Printf("%v\n", e)
		}
		if n < 240 {
			continue
		}
		req := dhcp4.Packet(buffer[:n])
		if req.HLen() > 16 {
			continue
		}
		options := req.ParseOptions()
		var reqType dhcp4.MessageType
		if t := options[dhcp4.OptionDHCPMessageType]; len(t) != 1 {
			continue
		} else {
			reqType = dhcp4.MessageType(t[0])
			if reqType < dhcp4.Discover || reqType > dhcp4.Inform {
				continue
			}
		}
		// for portability, we still defer package response decisions to the handler
		if res := px.ServeDHCP(req, reqType, options); res != nil {
			ipStr, portStr, e := net.SplitHostPort(addr.String())
			if e != nil {
				fmt.Printf("%v\n", e)
			}
			if net.ParseIP(ipStr).Equal(net.IPv4zero) || req.Broadcast() {
				port, _ := strconv.Atoi(portStr)
				addr = &net.UDPAddr{IP: net.IPv4bcast, Port: port}
			}
			if reqType == dhcp4.Discover {
				go px.transmitDhcpOffer(c, ac, addr, res)
			} else {
				_, e = c.WriteTo(res, addr)
			}
			if e != nil {
				fmt.Printf("%v\n", e)
			}
		}
	}
	fmt.Println("DHCP stopped.")
}

// StartTFTP starts up the TFTP service
func (px *PiPXE) StartTFTP(ip net.IP) {
	fmt.Println("starting TFTP service")
	srv := tftp.NewServer(px.writeToTFTP, nil)
	srv.ListenAndServe(ip.String() + ":69")
	fmt.Println("TFTP service stopped")
}

func (px *PiPXE) writeToTFTP(filename string, rf io.ReaderFrom) (e error) {
	ip := rf.(tftp.OutgoingTransfer).RemoteAddr().IP
	n := px.NodeGet(queryByIP, ip.String())
	if n == nil {
		fmt.Printf("got TFTP request from unknown node: %s\n", ip.String())
		return fmt.Errorf("got TFTP request from unknown node: %s", ip.String())
	}
	vs := n.GetValues([]string{"/Arch", "/Platform"})
	lfile := filepath.Join(
		px.cfg.TftpDir,
		vs["/Arch"].String(),
		vs["/Platform"].String(),
		filename,
	)
	var f io.Reader
	if _, e = os.Stat(lfile); os.IsNotExist(e) {
		if _, e = os.Stat(lfile + ".tpl"); os.IsNotExist(e) {
			// neither file nor template exist
			fmt.Printf("no such file: %s\n", lfile)
			return fmt.Errorf("no such file: %s", lfile)
		}
		// file doesn't exist, but template does
		// we could potentially make a lot more data than this available
		type tplData struct {
			IP       string
			CIDR     string
			ID       string
			ParentIP string
		}
		data := tplData{}
		i, _ := n.GetValue(px.cfg.IpUrl)
		data.IP = IPv4.BytesToIP(i.Bytes()).String()
		i, _ = n.GetValue(px.cfg.NmUrl)
		data.CIDR = IPv4.BytesToIP(i.Bytes()).String()
		data.ID = n.ID().String()
		data.ParentIP = px.selfIP.String()
		tpl, e := template.ParseFiles(lfile + ".tpl")
		if e != nil {
			fmt.Printf("template parse error: %v\n", e)
			return fmt.Errorf("template parse error: %v", e)
		}
		f := &bytes.Buffer{}
		tpl.Execute(f, &data)
	} else {
		// file exists
		f, e = os.Open(lfile)
		defer f.(*os.File).Close()
	}

	written, e := rf.ReadFrom(f)
	fmt.Printf("wrote %s (%s), %d bytes\n", filename, lfile, written)
	return
}

/*
 * concurrency safe accessors for nodeBy
 */

// NodeGet gets a node that we know about -- concurrency safe
func (px *PiPXE) NodeGet(qb nodeQueryBy, q string) (n lib.Node) { // returns nil for not found
	var ok bool
	px.mutex.RLock()
	if n, ok = px.nodeBy[qb][q]; !ok {
		fmt.Printf("tried to acquire node that doesn't exist: %s %s\n", qb, q)
		px.mutex.RUnlock()
	}
	px.mutex.RUnlock()
	return
}

// NodeDelete deletes a node that we know about -- cuncurrency safe
func (px *PiPXE) NodeDelete(qb nodeQueryBy, q string) { // silently ignores non-existent nodes
	var n lib.Node
	var ok bool
	px.mutex.Lock()
	if n, ok = px.nodeBy[qb][q]; !ok {
		px.mutex.Unlock()
		return
	}
	v := n.GetValues([]string{px.cfg.IpUrl, px.cfg.MacUrl})
	ip := IPv4.BytesToIP(v[px.cfg.IpUrl].Bytes())
	mac := IPv4.BytesToMAC(v[px.cfg.MacUrl].Bytes())
	delete(px.nodeBy[queryByIP], ip.String())
	delete(px.nodeBy[queryByMAC], mac.String())
	px.mutex.Unlock()
}

// NodeCreate creates a new node in our node pool -- concurrency safe
func (px *PiPXE) NodeCreate(n lib.Node) (e error) {
	v := n.GetValues([]string{px.cfg.IpUrl, px.cfg.MacUrl})
	if len(v) != 2 {
		return fmt.Errorf("missing ip or mac for node, aborting\n")
	}
	ip := IPv4.BytesToIP(v[px.cfg.IpUrl].Bytes())
	mac := IPv4.BytesToMAC(v[px.cfg.MacUrl].Bytes())
	if ip == nil || mac == nil { // incomplete node
		return fmt.Errorf("won't add incomplete node\n")
	}
	px.mutex.Lock()
	px.nodeBy[queryByIP][ip.String()] = n
	px.nodeBy[queryByMAC][mac.String()] = n
	px.mutex.Unlock()
	return
}

/*
 * lib.Module
 */

var _ lib.Module = (*PiPXE)(nil)

// Name returns the FQDN of the module
func (*PiPXE) Name() string { return "github.com/hpc/kraken/modules/pipxe" }

/*
 * lib.ModuleWithConfig
 */

var _ lib.Module = (*PiPXE)(nil)

// NewConfig returns a fully initialized default config
func (*PiPXE) NewConfig() proto.Message {
	r := &pb.PiPXEConfig{
		SrvIfaceUrl: "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Eth/Iface",
		SrvIpUrl:    "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		IpUrl:       "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		SubnetUrl:   "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Subnet",
		MacUrl:      "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Mac",
		TftpDir:     "tftp",
		ArpDeadline: "1s",
		DhcpRetry:   9,
	}
	return r
}

// UpdateConfig updates the running config
func (px *PiPXE) UpdateConfig(cfg proto.Message) (e error) {
	if pxcfg, ok := cfg.(*pb.PiPXEConfig); ok {
		px.cfg = pxcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*PiPXE) ConfigURL() string {
	cfg := &pb.PiPXEConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

/*
 * lib.ModuleWithMutations & lib.ModuleWithDiscovery
 */
var _ lib.ModuleWithMutations = (*PiPXE)(nil)
var _ lib.ModuleWithDiscovery = (*PiPXE)(nil)

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (px *PiPXE) SetMutationChan(c <-chan lib.Event) { px.mchan = c }

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (px *PiPXE) SetDiscoveryChan(c chan<- lib.Event) { px.dchan = c }

/*
 * lib.ModuleSelfService
 */
var _ lib.ModuleSelfService = (*PiPXE)(nil)

// Entry is the module's executable entrypoint
func (px *PiPXE) Entry() {
	nself, _ := px.api.QueryRead(px.api.Self().String())
	v, _ := nself.GetValue(px.cfg.SrvIpUrl)
	px.selfIP = IPv4.BytesToIP(v.Bytes())
	v, _ = nself.GetValue(px.cfg.SrvIfaceUrl)
	go px.StartDHCP(v.String(), px.selfIP)
	go px.StartTFTP(px.selfIP)
	url := lib.NodeURLJoin(px.api.Self().String(), SrvStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  px.Name(),
			URL:     url,
			ValueID: "RUN",
		},
	)
	px.dchan <- ev
	for {
		select {
		case v := <-px.mchan:
			if v.Type() != lib.Event_STATE_MUTATION {
				fmt.Printf("got unexpected non-mutation event\n")
				break
			}
			m := v.Data().(*core.MutationEvent)
			go px.handleMutation(m)
			break
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (px *PiPXE) Init(api lib.APIClient) {
	px.api = api
	px.mutex = sync.RWMutex{}
	px.nodeBy = make(map[nodeQueryBy]map[string]lib.Node)
	px.nodeBy[queryByIP] = make(map[string]lib.Node)
	px.nodeBy[queryByMAC] = make(map[string]lib.Node)
	px.cfg = px.NewConfig().(*pb.PiPXEConfig)
}

// Stop should perform a graceful exit
func (px *PiPXE) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (px *PiPXE) transmitDhcpOffer(c dhcp4.ServeConn, ac *arp.Client, addr net.Addr, res dhcp4.Packet) {
	deadline, _ := time.ParseDuration(px.cfg.ArpDeadline)
	ac.SetDeadline(time.Now().Add(deadline))
	fmt.Printf("arping %s...", res.YIAddr())
	hw, e := ac.Resolve(res.YIAddr())
	if e == nil && hw.String() != res.CHAddr().String() {
		fmt.Printf("address conflict, %s already in use by %s\n", res.YIAddr().String(), hw.String())
		return
	}
	if e != nil {
		fmt.Println("no answer.")
	}
	for i := 0; i < int(px.cfg.DhcpRetry); i++ {
		fmt.Println("(re)transmitting DHCP offer")
		_, e = c.WriteTo(res, addr)
		if e != nil {
			fmt.Printf("%v\n", e)
		}
		fmt.Printf("arping %s...", res.YIAddr().String())
		ac.SetDeadline(time.Now().Add(deadline))
		hw, e := ac.Resolve(res.YIAddr())
		if e == nil {
			if hw.String() != res.CHAddr().String() {
				fmt.Printf("address conflict, %s already in use by %s", res.YIAddr().String(), hw.String())
				continue
			} else {
				fmt.Printf("Got an arp match for %s on %s", res.YIAddr().String(), res.CHAddr().String())
				break
			}
		} else {
			fmt.Println("no answer.")
		}
	}
}

func (px *PiPXE) handleMutation(m *core.MutationEvent) {
	switch m.Type {
	case core.MutationEvent_MUTATE:
		switch m.Mutation[1] {
		case "NONEtoWAIT": // starting a new mutation, register the node
			if e := px.NodeCreate(m.NodeCfg); e != nil {
				fmt.Printf("%v\n", e)
				break
			}
			url := lib.NodeURLJoin(m.NodeCfg.ID().String(), PxeURL)
			ev := core.NewEvent(
				lib.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{
					Module:  px.Name(),
					URL:     url,
					ValueID: "WAIT",
				},
			)
			px.dchan <- ev
		case "WAITtoINIT": // we're initializing, but don't do anything (more for discovery/timeout)
		case "INITtoCOMP": // done mutating a node, deregister
			v, _ := m.NodeCfg.GetValue(px.cfg.IpUrl)
			ip := IPv4.BytesToIP(v.Bytes())
			px.NodeDelete(queryByIP, ip.String())
			url := lib.NodeURLJoin(m.NodeCfg.ID().String(), PxeURL)
			ev := core.NewEvent(
				lib.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{
					Module:  px.Name(),
					URL:     url,
					ValueID: "COMP",
				},
			)
			px.dchan <- ev
		}
	case core.MutationEvent_INTERRUPT: // on any interrupt, we remove the node
		v, _ := m.NodeCfg.GetValue(px.cfg.IpUrl)
		ip := IPv4.BytesToIP(v.Bytes())
		px.NodeDelete(queryByIP, ip.String())
	}
}

func init() {
	module := &PiPXE{}
	mutations := make(map[string]lib.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	dpxe := make(map[string]reflect.Value)

	for m := range muts {
		dur, _ := time.ParseDuration(muts[m].timeout)
		mutations[m] = core.NewStateMutation(
			map[string][2]reflect.Value{
				PxeURL: [2]reflect.Value{
					reflect.ValueOf(muts[m].f),
					reflect.ValueOf(muts[m].t),
				},
			},
			reqs,
			excs,
			lib.StateMutationContext_CHILD,
			dur,
			[3]string{module.Name(), "/PhysState", "PHYS_HANG"},
		)
		dpxe[rpipb.RPi3_PXE_name[int32(muts[m].t)]] = reflect.ValueOf(muts[m].t)
	}

	mutations["WAITtoINIT"] = core.NewStateMutation(
		map[string][2]reflect.Value{
			PxeURL: [2]reflect.Value{
				reflect.ValueOf(rpipb.RPi3_WAIT),
				reflect.ValueOf(rpipb.RPi3_INIT),
			},
			"/RunState": [2]reflect.Value{
				reflect.ValueOf(cpb.Node_UNKNOWN),
				reflect.ValueOf(cpb.Node_INIT),
			},
		},
		reqs,
		excs,
		lib.StateMutationContext_CHILD,
		time.Second*20,
		[3]string{module.Name(), "/PhysState", "PHYS_HANG"},
	)
	dpxe["INIT"] = reflect.ValueOf(rpipb.RPi3_INIT)

	discovers[PxeURL] = dpxe
	discovers["/RunState"] = map[string]reflect.Value{
		"NODE_INIT": reflect.ValueOf(cpb.Node_INIT),
	}
	discovers[SrvStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}
	si := core.NewServiceInstance("pipxe", module.Name(), module.Entry, nil)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(module, discovers)
	core.Registry.RegisterMutations(module, mutations)
}
