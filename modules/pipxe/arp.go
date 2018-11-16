/* arp.go: provides ARP ping support for many concurrent ARP pings
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package pipxe

import (
	"net"

	"github.com/google/gopacket"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	ARPOpRequest uint16 = 1
	ARPOpReply   uint16 = 2
)

type ARPResolver struct {
	iface net.Interface
	ip    net.IP
	h     *pcap.Handle
}

func (ar *ARPResolver) Start() {
}

func (ar *ARPResolver) sendARP(ip net.IP) (e error) {
	arp := &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         ARPOpRequest,
		SourceHwAddress:   ar.iface.HardwareAddr,
		SourceProtAddress: ar.ip.To4(),
		DstProtAddress:    ip.To4(),
	}

	eth := &layers.Ethernet{
		SrcMAC:       ar.iface.HardwareAddr,
		DstMAC:       []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeARP,
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	if e = gopacket.SerializeLayers(buf, opts, eth, arp); e != nil {
		return
	}

	e = ar.h.WritePacketData(buf.Bytes())
	return
}
