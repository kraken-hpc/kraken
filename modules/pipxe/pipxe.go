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
	"crypto/rand"
	"encoding/binary"
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

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/ipv4"

	"github.com/golang/protobuf/ptypes"
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
	MACVendor   = "b8:27:eb"
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
		timeout: "10s",
	},
	"INITtoCOMP": pxmut{
		f: rpipb.RPi3_INIT,
		t: rpipb.RPi3_COMP,
		reqs: map[string]reflect.Value{
			"/Arch":      reflect.ValueOf("aarch64"),
			"/Platform":  reflect.ValueOf("rpi3"),
			"/PhysState": reflect.ValueOf(cpb.Node_POWER_ON),
			"/RunState":  reflect.ValueOf(cpb.Node_SYNC),
		},
		timeout: "180s",
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

	selfIP  net.IP
	selfNet net.IP

	options   layers.DHCPOptions
	leaseTime time.Duration

	iface     *net.Interface
	rawHandle *pcap.Handle

	// for maintaining our list of currently booting nodes

	mutex  sync.RWMutex
	nodeBy map[nodeQueryBy]map[string]lib.Node
}

/*
 * service starters
 */

// StartDHCP starts up the DHCP service
func (px *PiPXE) StartDHCP(iface string, ip net.IP) {

	var netmask net.IP
	if px.selfNet.IsUnspecified() {
		netmask = net.ParseIP("255.255.255.0").To4()
	} else {
		netmask = px.selfNet.To4()
	}

	// FIXME: hardcoded value
	px.leaseTime = time.Minute * 5
	leaseTime := make([]byte, 4)
	binary.BigEndian.PutUint32(leaseTime, uint32(px.leaseTime.Seconds()))

	px.options = layers.DHCPOptions{
		layers.DHCPOption{Type: layers.DHCPOptMessageType, Length: 1, Data: []byte{0x02}},
		layers.DHCPOption{Type: layers.DHCPOptServerID, Length: 4, Data: px.selfIP.To4()},
		layers.DHCPOption{Type: layers.DHCPOptLeaseTime, Length: 4, Data: leaseTime},
		layers.DHCPOption{Type: layers.DHCPOptSubnetMask, Length: 4, Data: netmask.To4()},
		layers.DHCPOption{Type: layers.DHCPOptRouter, Length: 4, Data: px.selfIP.To4()},
		layers.DHCPOption{Type: layers.DHCPOptClassID, Length: 9, Data: []byte{0x50, 0x58, 0x45, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74}},
		layers.DHCPOption{Type: layers.DHCPOptVendorOption, Length: 32, Data: []byte{
			0x6, 0x1, 0x3, 0xa, 0x4, 0x0, 0x50, 0x58, 0x45, 0x9, 0x14, 0x0, 0x0, 0x11, 0x52, 0x61,
			0x73, 0x70, 0x62, 0x65, 0x72, 0x72, 0x79, 0x20, 0x50, 0x69, 0x20, 0x42, 0x6f, 0x6f, 0x74, 0xff}},
	}

	var e error

	// Find our interface
	px.iface, e = net.InterfaceByName(iface)
	if e != nil {
		px.api.Logf(lib.LLCRITICAL, "%v: %s", e, iface)
		return
	}

	// We need the raw handle to send unicast packet replies
	px.rawHandle, e = pcap.OpenLive(px.iface.Name, 1024, false, (30 * time.Second))
	if e != nil {
		px.api.Logf(lib.LLCRITICAL, "%v: %s", e, iface)
		return
	}
	defer px.rawHandle.Close()

	// We use this packetconn to read from
	nc, e := net.ListenPacket("udp4", ":67")
	if e != nil {
		px.api.Logf(lib.LLCRITICAL, "%v", e)
		return
	}
	c := ipv4.NewPacketConn(nc)
	defer c.Close()
	px.api.Logf(lib.LLINFO, "started DHCP listener on: %s", iface)

	buffer := make([]byte, 1500)
	var req layers.DHCPv4
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeDHCPv4, &req)
	decoded := []gopacket.LayerType{}
	// main read loop
	for {
		n, cm, addr, e := c.ReadFrom(buffer)
		if e != nil {
			px.api.Logf(lib.LLCRITICAL, "%v", e)
			break
		}
		if cm.IfIndex != px.iface.Index {
			// ignore packets not intended for our interface
			break
		}
		px.api.Logf(lib.LLDDEBUG, "got a dhcp packet from: %s", addr.String())
		if n < 240 {
			px.api.Logf(lib.LLDDEBUG, "packet is too short: %d < 240", n)
			continue
		}

		if e = parser.DecodeLayers(buffer[:n], &decoded); e != nil {
			px.api.Logf(lib.LLERROR, "error decoding packet: %v", e)
			continue
		}
		if len(decoded) < 1 || decoded[0] != layers.LayerTypeDHCPv4 {
			px.api.Logf(lib.LLERROR, "decoded non-DHCP packet")
			continue
		}
		// at this point we have a parsed DHCPv4 packet

		if req.Operation != layers.DHCPOpRequest {
			// odd...
			continue
		}
		if req.HardwareLen > 16 {
			px.api.Logf(lib.LLDDEBUG, "packet HardwareLen too long: %d > 16", req.HardwareLen)
			continue
		}

		go px.handleDHCPRequest(req)

		/*
			// for portability, we still defer package response decisions to the handler
			if res, n, raw := px.ServeDHCP(req, reqType, options); res != nil {
				ipStr, portStr, e := net.SplitHostPort(addr.String())
				if e != nil {
					px.api.Logf(lib.LLERROR, "%v", e)
				}
				if net.ParseIP(ipStr).Equal(net.IPv4zero) || req.Broadcast() {
					// 	port, _ := strconv.Atoi(portStr)
					// 	addr = &net.UDPAddr{IP: net.IPv4bcast, Port: port}
				}
				port, _ := strconv.Atoi(portStr)
				addr = &net.UDPAddr{IP: res.CIAddr(), Port: port}
				if reqType == dhcp4.Discover {
					go px.transmitDHCPOffer(n, c, ac, addr, res, raw)
				} else {
					_, e = c.WriteTo(res, addr)
				}
				if e != nil {
					px.api.Logf(lib.LLERROR, "%v", e)
				}
			}
		*/
	}
	px.api.Log(lib.LLNOTICE, "DHCP stopped.")
}

// StartTFTP starts up the TFTP service
func (px *PiPXE) StartTFTP(ip net.IP) {
	px.api.Log(lib.LLNOTICE, "starting TFTP service")
	srv := tftp.NewServer(px.writeToTFTP, nil)
	e := srv.ListenAndServe(ip.String() + ":69")
	if e != nil {
		px.api.Logf(lib.LLCRITICAL, "TFTP failed to start: %v", e)
	}
	px.api.Log(lib.LLNOTICE, "TFTP service stopped")
}

func (px *PiPXE) writeToTFTP(filename string, rf io.ReaderFrom) (e error) {
	ip := rf.(tftp.OutgoingTransfer).RemoteAddr().IP
	n := px.NodeGet(queryByIP, ip.String())
	if n == nil {
		px.api.Logf(lib.LLDEBUG, "got TFTP request from unknown node: %s", ip.String())
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
			px.api.Logf(lib.LLDEBUG, "no such file: %s", lfile)
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
		subip := IPv4.BytesToIP(i.Bytes())
		cidr, _ := net.IPMask(subip.To4()).Size()
		data.CIDR = strconv.Itoa(cidr)
		data.ID = n.ID().String()
		data.ParentIP = px.selfIP.String()
		tpl, e := template.ParseFiles(lfile + ".tpl")
		if e != nil {
			px.api.Logf(lib.LLDEBUG, "template parse error: %v", e)
			return fmt.Errorf("template parse error: %v", e)
		}
		f = &bytes.Buffer{}
		tpl.Execute(f.(io.Writer), &data)
	} else {
		// file exists
		f, e = os.Open(lfile)
		defer f.(*os.File).Close()
	}

	written, e := rf.ReadFrom(f)
	px.api.Logf(lib.LLDEBUG, "wrote %s (%s), %d bytes", filename, lfile, written)
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
		px.api.Logf(lib.LLERROR, "tried to acquire node that doesn't exist: %s %s", qb, q)
		px.mutex.RUnlock()
		return
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
		return fmt.Errorf("missing ip or mac for node, aborting")
	}
	ip := IPv4.BytesToIP(v[px.cfg.IpUrl].Bytes())
	mac := IPv4.BytesToMAC(v[px.cfg.MacUrl].Bytes())
	if ip == nil || mac == nil { // incomplete node
		return fmt.Errorf("won't add incomplete node: ip: %v, mac: %v", ip, mac)
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
		NmUrl:       "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Subnet",
		SubnetUrl:   "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Subnet",
		MacUrl:      "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Eth/Mac",
		TftpDir:     "tftp",
		ArpDeadline: "500ms",
		DhcpRetry:   30,
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
	v, _ = nself.GetValue(px.cfg.SubnetUrl)
	px.selfNet = IPv4.BytesToIP(v.Bytes())
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
				px.api.Log(lib.LLERROR, "got unexpected non-mutation event")
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

// handleDHCPRequest is the main handler for new DHCP packets
func (px *PiPXE) handleDHCPRequest(p layers.DHCPv4) {

	// ignore if this doesn't appear to be a Pi
	if string([]rune(strings.ToLower(p.ClientHWAddr.String())[0:8])) != MACVendor {
		px.api.Logf(lib.LLDDEBUG, "ignoring packet from non-Pi mac: %s", p.ClientHWAddr.String())
		return
	}

	// The only option we're using for now is MessageType, let's get it
	t := layers.DHCPMsgTypeUnspecified
	for _, o := range p.Options {
		if o.Type == layers.DHCPOptMessageType {
			if o.Length != 1 {
				continue
			}
			t = layers.DHCPMsgType(o.Data[0])
			break
		}
	}

	switch t {
	case layers.DHCPMsgTypeDiscover:
		px.api.Logf(lib.LLDEBUG, "got DHCP discover from %s", p.ClientHWAddr.String())
		n := px.NodeGet(queryByMAC, p.ClientHWAddr.String())
		if n == nil {
			px.api.Logf(lib.LLDEBUG, "ignoring DHCP discover from unknown %s", p.ClientHWAddr.String())
			return
		}
		v, e := n.GetValue(px.cfg.IpUrl)
		if e != nil {
			px.api.Logf(lib.LLDEBUG, "node does not have an IP in state %s", p.ClientHWAddr.String())
			return
		}
		ip := IPv4.BytesToIP(v.Bytes())
		px.api.Logf(lib.LLDEBUG, "sending DHCP offer of %s to %s", ip.String(), p.ClientHWAddr.String())

		r := px.offerPacket(
			p,
			layers.DHCPMsgTypeOffer,
			px.selfIP.To4(),
			ip,
			px.leaseTime,
			layers.DHCPOptions{},
		)

		px.transmitDHCPOffer(n, r)
		return
	default: // Pi's only send Discovers
		px.api.Log(lib.LLDEBUG, "Unhandled DHCP packet.")
	}
	return
}

func (px *PiPXE) offerPacket(p layers.DHCPv4, msgType layers.DHCPMsgType, selfIP net.IP, destIP net.IP, leaseTimeDuration time.Duration, dhcpOptions layers.DHCPOptions) []byte {
	piMac := p.ClientHWAddr
	randToken := make([]byte, 2)
	rand.Read(randToken)
	leaseTime := make([]byte, 4)
	binary.BigEndian.PutUint32(leaseTime, uint32(leaseTimeDuration.Seconds()))

	o := append(px.options, []layers.DHCPOption(dhcpOptions)...)

	var broadcast bool
	if (p.Flags >> 15) == 1 {
		broadcast = true
	}

	dhcp := &layers.DHCPv4{
		Operation:    layers.DHCPOpReply,
		HardwareType: layers.LinkTypeEthernet,
		HardwareLen:  6,
		Xid:          p.Xid,
		Secs:         0,
		Flags:        0x0000,
		ClientIP:     net.IPv4zero,
		YourClientIP: destIP.To4(),
		NextServerIP: net.IPv4zero,
		RelayAgentIP: net.IPv4zero,
		ClientHWAddr: piMac,
		ServerName:   []byte{},
		File:         []byte{},
		Options:      o,
	}

	udp := &layers.UDP{
		SrcPort: 67,
		DstPort: 68,
	}

	ipv4 := &layers.IPv4{
		Version:    4,
		IHL:        20,
		TOS:        0x00,
		Id:         binary.BigEndian.Uint16(randToken),
		Flags:      0x00,
		FragOffset: 0x00,
		TTL:        64,
		Protocol:   layers.IPProtocolUDP,
		SrcIP:      selfIP,
		DstIP:      destIP,
	}
	if broadcast {
		ipv4.DstIP = []byte{255, 255, 255, 255}
	}

	err := udp.SetNetworkLayerForChecksum(ipv4)

	eth := &layers.Ethernet{
		SrcMAC:       px.iface.HardwareAddr,
		DstMAC:       piMac,
		EthernetType: layers.EthernetTypeIPv4,
	}
	if broadcast {
		eth.DstMAC = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	err = gopacket.SerializeLayers(buf, opts, eth, ipv4, udp, dhcp)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (px *PiPXE) transmitDHCPOffer(n lib.Node, raw []byte) {
	//deadline, _ := time.ParseDuration(px.cfg.ArpDeadline)
	//ac.SetDeadline(time.Now().Add(deadline))
	//px.api.Logf(lib.LLDEBUG, "arping %s...", res.YIAddr())
	// hw, e := ac.Resolve(res.YIAddr())
	// if e == nil && hw.String() != res.CHAddr().String() {
	// 	px.api.Logf(lib.LLERROR, "address conflict, %s already in use by %s", res.YIAddr().String(), hw.String())
	// 	return
	// }
	// if e != nil {
	// 	px.api.Log(lib.LLDDEBUG, "no answer.")
	// }
	for i := 0; i < int(px.cfg.DhcpRetry); i++ {
		px.api.Log(lib.LLDEBUG, "(re)transmitting DHCP offer")
		// _, e = c.WriteTo(res, addr)
		// if e != nil {
		// 	px.api.Logf(lib.LLERROR, "%v", e)
		// }

		err := px.rawHandle.WritePacketData(raw)
		if err != nil {
			panic(err)
		}

		//px.api.Logf(lib.LLDEBUG, "arping %s...", res.YIAddr().String())
		//ac.SetDeadline(time.Now().Add(deadline))
		// hw, e := ac.Resolve(res.YIAddr())
		/*
			if hw.String() != res.CHAddr().String() {
				px.api.Logf(lib.LLERROR, "address conflict, %s already in use by %s", res.YIAddr().String(), hw.String())
				continue
			} else { */
		// px.api.Logf(lib.LLDEBUG, "Got an arp match for %s on %s", res.YIAddr().String(), res.CHAddr().String())
		// we discover PXE INIT and RunState INIT
		url1 := lib.NodeURLJoin(n.ID().String(), PxeURL)
		ev1 := core.NewEvent(
			lib.Event_DISCOVERY,
			url1,
			&core.DiscoveryEvent{
				Module:  px.Name(),
				URL:     url1,
				ValueID: "INIT",
			},
		)
		url2 := lib.NodeURLJoin(n.ID().String(), "/RunState")
		ev2 := core.NewEvent(
			lib.Event_DISCOVERY,
			url1,
			&core.DiscoveryEvent{
				Module:  px.Name(),
				URL:     url2,
				ValueID: "NODE_INIT",
			},
		)
		px.dchan <- ev1
		px.dchan <- ev2
		break
		//	}
	}
}

func (px *PiPXE) handleMutation(m *core.MutationEvent) {
	switch m.Type {
	case core.MutationEvent_MUTATE:
		switch m.Mutation[1] {
		case "NONEtoWAIT": // starting a new mutation, register the node
			if e := px.NodeCreate(m.NodeCfg); e != nil {
				px.api.Logf(lib.LLERROR, "%v", e)
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
		v, e := m.NodeCfg.GetValue(px.cfg.IpUrl)
		if e != nil || !v.IsValid() {
			break
		}
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
		time.Second*90,
		[3]string{module.Name(), "/PhysState", "PHYS_HANG"},
	)
	dpxe["INIT"] = reflect.ValueOf(rpipb.RPi3_INIT)

	discovers[PxeURL] = dpxe
	discovers["/RunState"] = map[string]reflect.Value{
		"NODE_INIT": reflect.ValueOf(cpb.Node_INIT),
	}
	discovers["/PhysState"] = map[string]reflect.Value{
		"PHYS_HANG": reflect.ValueOf(cpb.Node_PHYS_HANG),
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
