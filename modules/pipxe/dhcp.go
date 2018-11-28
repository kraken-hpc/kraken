/* dhcp.go: provides pipxe DHCP support
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package pipxe

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/extensions/IPv4"
	"github.com/hpc/kraken/lib"
	"golang.org/x/net/ipv4"
)

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

	/*
		// We use an ARPResolver
		d, e := time.ParseDuration(px.cfg.ArpDeadline)
		if e != nil {
			px.api.Logf(lib.LLERROR, "invalid arp duration: %v", e)
			d = time.Millisecond * 500
		}
		px.arp = NewARPResolver(px.iface, px.selfIP, d)
		go px.arp.Start()
	*/

	// We need the raw handle to send unicast packet replies
	px.rawHandle, e = pcap.OpenLive(px.iface.Name, 1024, false, (30 * time.Second))
	if e != nil {
		px.api.Logf(lib.LLCRITICAL, "%v: %s", e, iface)
		return
	}
	defer px.rawHandle.Close()
	// restrict our socket a bit
	px.rawHandle.SetDirection(pcap.DirectionOut)
	// this may not really be necessary.  we currently never read from this
	if e = px.rawHandle.SetBPFFilter("udp and port 67"); e != nil {
		px.api.Logf(lib.LLERROR, "failed to set BPF on raw socket: %v", e)
	}

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
		n, _, addr, e := c.ReadFrom(buffer)
		if e != nil {
			px.api.Logf(lib.LLCRITICAL, "%v", e)
			break
		}
		/* FIXME: This appears to be broken, we disable for now
		if cm == nil || cm.IfIndex != px.iface.Index {
			// ignore packets not intended for our interface
			break
		}
		*/
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
	}
	px.api.Log(lib.LLNOTICE, "DHCP stopped.")
}

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

		px.transmitDHCPOffer(n, ip, p.ClientHWAddr, r)
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

func (px *PiPXE) transmitDHCPOffer(n lib.Node, ip net.IP, mac net.HardwareAddr, raw []byte) error {
	//var rmac net.HardwareAddr
	var e error
	/*
		rmac, e = px.arp.Resolve(ip)
		if e == nil && rmac.String() != mac.String() {
			return fmt.Errorf("IP address conflict: %s is in use by %s", ip.String(), rmac.String())
		}
	*/
	for i := 0; i < int(px.cfg.DhcpRetry); i++ {
		px.api.Logf(lib.LLDEBUG, "transmitting DHCP offer (attempt: %d)", i+1)

		e = px.rawHandle.WritePacketData(raw)
		if e != nil {
			return e
		}

		//_, e = px.arp.Resolve(ip)
		if e == nil {
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
			return nil
		}
	}
	return fmt.Errorf("client did not take DHCP offer")
}
