/* dhcp.go: provides pxe DHCP support
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package pxe

import (
	"crypto/rand"
	"encoding/binary"
	"net"
	"time"

	"github.com/mdlayher/raw"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/extensions/IPv4"
	"github.com/hpc/kraken/lib"
	"golang.org/x/net/ipv4"
)

const (
	DHCPPacketBuffer int    = 1500
	DHCPPXEFile      string = "pxelinux.0" // this should probably be configurable
)

// StartDHCP starts up the DHCP service
func (px *PXE) StartDHCP(iface string, ip net.IP) {

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
	}

	var e error

	// Find our interface
	px.iface, e = net.InterfaceByName(iface)
	if e != nil {
		px.api.Logf(lib.LLCRITICAL, "%v: %s", e, iface)
		return
	}

	// We need the raw handle to send unicast packet replies
	// This is only used for sending initial DHCP offers
	// Note: 0x0800 is EtherType for IPv4. See: https://en.wikipedia.org/wiki/EtherType
	px.rawHandle, e = raw.ListenPacket(px.iface, 0x0800, nil)
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

	// main read loop
	for {
		buffer := make([]byte, DHCPPacketBuffer)
		var req layers.DHCPv4
		parser := gopacket.NewDecodingLayerParser(layers.LayerTypeDHCPv4, &req)
		decoded := []gopacket.LayerType{}

		n, _, addr, e := c.ReadFrom(buffer)
		if e != nil {
			px.api.Logf(lib.LLCRITICAL, "%v", e)
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
	}
	px.api.Log(lib.LLNOTICE, "DHCP stopped.")
}

// handleDHCPRequest is the main handler for new DHCP packets
func (px *PXE) handleDHCPRequest(p layers.DHCPv4) {

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
		// fmt.Printf("%v, %v, %p, %v\n", count, n.ID().String(), &n, p.ClientHWAddr.String())
		if n == nil {
			px.api.Logf(lib.LLDEBUG, "ignoring DHCP discover from unknown %s", p.ClientHWAddr.String())
			return
		}
		// fmt.Printf("%v, %v, %p, %v\n", count, n.ID().String(), &n, p.ClientHWAddr.String())
		v, e := n.GetValue(px.cfg.IpUrl)
		if e != nil {
			px.api.Logf(lib.LLDEBUG, "node does not have an IP in state %s", p.ClientHWAddr.String())
			return
		}
		// fmt.Printf("%v, %v, %p, %v, %v\n", count, n.ID().String(), &n, p.ClientHWAddr.String(), IPv4.BytesToIP(v.Bytes()))
		ip := IPv4.BytesToIP(v.Bytes())
		px.api.Logf(lib.LLDEBUG, "sending DHCP offer of %s to %s", ip.String(), p.ClientHWAddr.String())

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
		return
	case layers.DHCPMsgTypeRequest:
		// TODO: process request packet
	// We don't expect any of the following, so we don't handle them
	case layers.DHCPMsgTypeAck:
		fallthrough
	case layers.DHCPMsgTypeDecline:
		fallthrough
	case layers.DHCPMsgTypeInform:
		fallthrough
	case layers.DHCPMsgTypeNak:
		fallthrough
	case layers.DHCPMsgTypeOffer:
		fallthrough
	case layers.DHCPMsgTypeRelease:
		fallthrough
	case layers.DHCPMsgTypeUnspecified:
		fallthrough
	default: // Pi's only send Discovers
		px.api.Log(lib.LLDEBUG, "Unhandled DHCP packet.")
	}
	return
}

func (px *PXE) offerPacket(p layers.DHCPv4, msgType layers.DHCPMsgType, selfIP net.IP, destIP net.IP, leaseTimeDuration time.Duration, dhcpOptions layers.DHCPOptions) []byte {
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
		NextServerIP: selfIP.To4(),
		RelayAgentIP: net.IPv4zero,
		ClientHWAddr: piMac,
		ServerName:   []byte{},
		File:         []byte(DHCPPXEFile),
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

func (px *PXE) transmitDHCPOffer(n lib.Node, ip net.IP, mac net.HardwareAddr, rawp []byte) error {
	var e error
	px.api.Logf(lib.LLDEBUG, "transmitting DHCP offer")

	_, e = px.rawHandle.WriteTo(rawp, &raw.Addr{HardwareAddr: mac})
	if e != nil {
		return e
	}

	// ... this should move to after we send an ACK
	url1 := lib.NodeURLJoin(n.ID().String(), PXEStateURL)
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
