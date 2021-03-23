/* dhcp.go: provides pipxe DHCP support
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
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

	"github.com/mdlayher/raw"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/kraken-hpc/kraken/core"
	ipv4t "github.com/kraken-hpc/kraken/extensions/ipv4/customtypes"
	"github.com/kraken-hpc/kraken/lib/types"
	"github.com/kraken-hpc/kraken/lib/util"
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
		px.api.Logf(types.LLCRITICAL, "%v: %s", e, iface)
		return
	}

	/*
		// We use an ARPResolver
		d, e := time.ParseDuration(px.cfg.ArpDeadline)
		if e != nil {
			px.api.Logf(types.LLERROR, "invalid arp duration: %v", e)
			d = time.Millisecond * 500
		}
		px.arp = NewARPResolver(px.iface, px.selfIP, d)
		go px.arp.Start()
	*/

	// We need the raw handle to send unicast packet replies
	// This is only used for sending initial DHCP offers
	// Note: 0x0800 is EtherType for ipv4e. See: https://en.wikipedia.org/wiki/EtherType
	px.rawHandle, e = raw.ListenPacket(px.iface, 0x0800, nil)
	if e != nil {
		px.api.Logf(types.LLCRITICAL, "%v: %s", e, iface)
		return
	}
	defer px.rawHandle.Close()

	// We use this packetconn to read from
	nc, e := net.ListenPacket("udp4", ":67")
	if e != nil {
		px.api.Logf(types.LLCRITICAL, "%v", e)
		return
	}
	c := ipv4.NewPacketConn(nc)
	defer c.Close()
	px.api.Logf(types.LLINFO, "started DHCP listener on: %s", iface)

	// main read loop
	for {
		buffer := make([]byte, 1500)
		var req layers.DHCPv4
		parser := gopacket.NewDecodingLayerParser(layers.LayerTypeDHCPv4, &req)
		decoded := []gopacket.LayerType{}

		n, _, addr, e := c.ReadFrom(buffer)
		if e != nil {
			px.api.Logf(types.LLCRITICAL, "%v", e)
			break
		}
		/* FIXME: This appears to be broken, we disable for now
		if cm == nil || cm.IfIndex != px.iface.Index {
			// ignore packets not intended for our interface
			break
		}
		*/
		px.api.Logf(types.LLDDEBUG, "got a dhcp packet from: %s", addr.String())
		if n < 240 {
			px.api.Logf(types.LLDDEBUG, "packet is too short: %d < 240", n)
			continue
		}

		if e = parser.DecodeLayers(buffer[:n], &decoded); e != nil {
			px.api.Logf(types.LLERROR, "error decoding packet: %v", e)
			continue
		}
		if len(decoded) < 1 || decoded[0] != layers.LayerTypeDHCPv4 {
			px.api.Logf(types.LLERROR, "decoded non-DHCP packet")
			continue
		}
		// at this point we have a parsed DHCPv4 packet

		if req.Operation != layers.DHCPOpRequest {
			// odd...
			continue
		}
		if req.HardwareLen > 16 {
			px.api.Logf(types.LLDDEBUG, "packet HardwareLen too long: %d > 16", req.HardwareLen)
			continue
		}

		go px.handleDHCPRequest(req)
		// count++
	}
	px.api.Log(types.LLNOTICE, "DHCP stopped.")
}

// handleDHCPRequest is the main handler for new DHCP packets
func (px *PiPXE) handleDHCPRequest(p layers.DHCPv4) {

	// ignore if this doesn't appear to be a Pi
	if string([]rune(strings.ToLower(p.ClientHWAddr.String())[0:8])) != MACVendor {
		px.api.Logf(types.LLDDEBUG, "ignoring packet from non-Pi mac: %s", p.ClientHWAddr.String())
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
		px.api.Logf(types.LLDEBUG, "got DHCP discover from %s", p.ClientHWAddr.String())
		n := px.NodeGet(queryByMAC, p.ClientHWAddr.String())
		// fmt.Printf("%v, %v, %p, %v\n", count, n.ID().String(), &n, p.ClientHWAddr.String())
		if n == nil {
			px.api.Logf(types.LLDEBUG, "ignoring DHCP discover from unknown %s", p.ClientHWAddr.String())
			return
		}
		// fmt.Printf("%v, %v, %p, %v\n", count, n.ID().String(), &n, p.ClientHWAddr.String())
		v, e := n.GetValue(px.cfg.IpUrl)
		if e != nil {
			px.api.Logf(types.LLDEBUG, "node does not have an IP in state %s", p.ClientHWAddr.String())
			return
		}
		// fmt.Printf("%v, %v, %p, %v, %v\n", count, n.ID().String(), &n, p.ClientHWAddr.String(), ipv4e.BytesToIP(v.Bytes()))
		ip := v.Interface().(ipv4t.IP).IP
		px.api.Logf(types.LLDEBUG, "sending DHCP offer of %s to %s", ip.String(), p.ClientHWAddr.String())

		// fmt.Printf("%v, %v, %p, %v, %v\n", count, n.ID().String(), &n, p.ClientHWAddr.String(), ip)
		r := px.offerPacket(
			p,
			layers.DHCPMsgTypeOffer,
			px.selfIP.To4(),
			ip,
			px.leaseTime,
			layers.DHCPOptions{},
		)
		// fmt.Printf("%v, %v, %p, %v, %v\n", count, n.ID().String(), &n, p.ClientHWAddr.String(), ip)
		px.transmitDHCPOffer(n, ip, p.ClientHWAddr, r)

		// stop wake packets
		px.wakeMutex.Lock()
		if stop, ok := px.nodeWake[n.ID().String()]; ok {
			stop <- true
		}
		delete(px.nodeWake, n.ID().String())
		px.wakeMutex.Unlock()
		return
	default: // Pi's only send Discovers
		px.api.Log(types.LLDEBUG, "Unhandled DHCP packet.")
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

func (px *PiPXE) transmitDHCPOffer(n types.Node, ip net.IP, mac net.HardwareAddr, rawp []byte) error {
	//var rmac net.HardwareAddr
	var e error
	/*
		rmac, e = px.arp.Resolve(ip)
		if e == nil && rmac.String() != mac.String() {
			return fmt.Errorf("IP address conflict: %s is in use by %s", ip.String(), rmac.String())
		}
	*/
	for i := 0; i < int(px.cfg.DhcpRetry); i++ {
		px.api.Logf(types.LLDEBUG, "transmitting DHCP offer (attempt: %d)", i+1)

		_, e = px.rawHandle.WriteTo(rawp, &raw.Addr{HardwareAddr: mac})
		if e != nil {
			return e
		}

		//_, e = px.arp.Resolve(ip)
		if e == nil {
			url1 := util.NodeURLJoin(n.ID().String(), PxeURL)
			ev1 := core.NewEvent(
				types.Event_DISCOVERY,
				url1,
				&core.DiscoveryEvent{
					URL:     url1,
					ValueID: "INIT",
				},
			)
			url2 := util.NodeURLJoin(n.ID().String(), "/RunState")
			ev2 := core.NewEvent(
				types.Event_DISCOVERY,
				url2,
				&core.DiscoveryEvent{
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

// wakeNodes sends a wake packet every second until something is passed into the stop channel.
// will also stop after 30 seconds of spamming
func (px *PiPXE) wakeNode(n types.Node, stop <-chan bool) {
	macValue, e := n.GetValue(px.cfg.GetMacUrl())
	if e != nil {
		px.api.Logf(types.LLERROR, "Error getting mac address for node: %v", e)
		return
	}
	mac := macValue.Interface().(ipv4t.MAC).HardwareAddr
	if mac == nil {
		px.api.Logf(types.LLERROR, "Error parsing mac address from value: %v", macValue.Bytes())
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	// safeguard timer
	timer := time.NewTimer(30 * time.Second)

	for {
		select {
		case <-timer.C:
			px.api.Logf(types.LLDDDEBUG, "30 second wake packet timer for: %v %v", n.ID().String(), mac.String())
			px.wakeMutex.Lock()
			delete(px.nodeWake, n.ID().String())
			px.wakeMutex.Unlock()
			return
		case <-stop:
			// assumes the handling function deleted the channel from the px.nodeWake map
			px.api.Logf(types.LLDDDEBUG, "Stopping wake packets for: %v %v", n.ID().String(), mac.String())
			return
		case <-ticker.C:
			e = px.sendWakePacket(mac)
			if e != nil {
				px.api.Logf(types.LLERROR, "Error sending wake packet: %v", e)
				px.wakeMutex.Lock()
				delete(px.nodeWake, n.ID().String())
				px.wakeMutex.Unlock()
				return
			}
		}
	}
}

var wakePacket = []byte{
	0x01, 0x00, 0x5e, 0x7f, 0xff, 0xfa, 0x00, 0xe0, 0x4c, 0x34, 0x48, 0x72, 0x08, 0x00, 0x45, 0x00,
	0x00, 0xcb, 0x89, 0x92, 0x00, 0x00, 0x01, 0x11, 0x3e, 0x22, 0x0a, 0x0f, 0xf7, 0x64, 0xef, 0xff,
	0xff, 0xfa, 0xdd, 0xf4, 0x07, 0x6c, 0x00, 0xb7, 0x4d, 0x80, 0x4d, 0x2d, 0x53, 0x45, 0x41, 0x52,
	0x43, 0x48, 0x20, 0x2a, 0x20, 0x48, 0x54, 0x54, 0x50, 0x2f, 0x31, 0x2e, 0x31, 0x0d, 0x0a, 0x48,
	0x4f, 0x53, 0x54, 0x3a, 0x20, 0x32, 0x33, 0x39, 0x2e, 0x32, 0x35, 0x35, 0x2e, 0x32, 0x35, 0x35,
	0x2e, 0x32, 0x35, 0x30, 0x3a, 0x31, 0x39, 0x30, 0x30, 0x0d, 0x0a, 0x4d, 0x41, 0x4e, 0x3a, 0x20,
	0x22, 0x73, 0x73, 0x64, 0x70, 0x3a, 0x64, 0x69, 0x73, 0x63, 0x6f, 0x76, 0x65, 0x72, 0x22, 0x0d,
	0x0a, 0x4d, 0x58, 0x3a, 0x20, 0x31, 0x0d, 0x0a, 0x53, 0x54, 0x3a, 0x20, 0x75, 0x72, 0x6e, 0x3a,
	0x64, 0x69, 0x61, 0x6c, 0x2d, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x73, 0x63, 0x72, 0x65, 0x65, 0x6e,
	0x2d, 0x6f, 0x72, 0x67, 0x3a, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x3a, 0x64, 0x69, 0x61,
	0x6c, 0x3a, 0x31, 0x0d, 0x0a, 0x55, 0x53, 0x45, 0x52, 0x2d, 0x41, 0x47, 0x45, 0x4e, 0x54, 0x3a,
	0x20, 0x47, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x20, 0x43, 0x68, 0x72, 0x6f, 0x6d, 0x65, 0x2f, 0x37,
	0x30, 0x2e, 0x30, 0x2e, 0x33, 0x35, 0x33, 0x38, 0x2e, 0x31, 0x31, 0x30, 0x20, 0x4d, 0x61, 0x63,
	0x20, 0x4f, 0x53, 0x20, 0x58, 0x0d, 0x0a, 0x0d, 0x0a,
}

func (px *PiPXE) sendWakePacket(mac net.HardwareAddr) (e error) {
	_, e = px.rawHandle.WriteTo(wakePacket, &raw.Addr{HardwareAddr: mac})
	if e != nil {
		return e
	}

	px.api.Logf(types.LLDDDEBUG, "Sent wake packet to %v", mac.String())
	return nil
}
