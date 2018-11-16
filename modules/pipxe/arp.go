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
	"fmt"
	"net"
	"time"

	"github.com/google/gopacket"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	ARPOpRequest uint16 = 1
	ARPOpReply   uint16 = 2
)

// An ARPResponse is returned for every ARPRequest
// If Err != nil, other fields may not be set
type ARPResponse struct {
	Err  error
	MAC  net.HardwareAddr
	Time time.Duration
}

// An ARPRequest request ARP resolution for an IP
type ARPRequest struct {
	IP        net.IP
	RChan     chan ARPResponse
	TimeoutOn time.Time
}

// An ARPResolver handles parallel ARP resolution
type ARPResolver struct {
	queue   []ARPRequest
	iface   *net.Interface
	ip      net.IP
	h       *pcap.Handle
	timeout time.Duration
	c       chan ARPRequest
}

func NewARPResolver(iface *net.Interface, ip net.IP, timeout time.Duration) *ARPResolver {
	ar := ARPResolver{
		iface:   iface,
		ip:      ip,
		timeout: timeout,
		c:       make(chan ARPRequest),
	}
	return &ar
}

func (ar *ARPResolver) Chan() chan<- ARPRequest {
	return ar.c
}

func (ar *ARPResolver) Start() {
	var e error
	if ar.h, e = pcap.OpenLive(ar.iface.Name, 1500, false, pcap.BlockForever); e != nil {
		fmt.Println(e)
		return
	}
	if e = ar.h.SetBPFFilter("arp"); e != nil {
		fmt.Println(e)
		return
	}

	readChan := make(chan layers.ARP)
	go func() { // reads ARP packets and delivers them back via readChan
		var buf []byte
		var e error
		var eth layers.Ethernet
		var arp layers.ARP
		var payload gopacket.Payload
		parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, &eth, &arp, &payload)
		decoded := []gopacket.LayerType{}
		for {
			buf, _, e = ar.h.ReadPacketData()
			if e != nil {
				fmt.Println(e)
				continue
			}
			e = parser.DecodeLayers(buf, &decoded)
			if e != nil {
				fmt.Println(e)
				continue
			}
			gotARP := false
			for _, l := range decoded {
				if l == layers.LayerTypeARP {
					gotARP = true
				}
			}
			if !gotARP {
				fmt.Printf("ARPResolver got non-ARP packet")
				continue
			}
			readChan <- arp
		}
	}()

	timer := time.NewTimer(ar.timeout) // timer handles reaping of timeouts
	for {
		select {
		case <-timer.C: // timer fired, reap timeouts
			for i := range ar.queue {
				if ar.queue[i].TimeoutOn.After(time.Now()) { // everything beyond here hasn't yet timed out
					break
				}
				ar.queue[i].RChan <- ARPResponse{
					Err:  fmt.Errorf("ARP timeout"),
					Time: ar.timeout,
				}
				ar.queue = ar.queue[1:]
			}
		case req := <-ar.c: // got a new request
			req.TimeoutOn = time.Now().Add(ar.timeout)
			ar.queue = append(ar.queue, req)
			ar.sendARP(req.IP)
			break
		case p := <-readChan: // we got a response
			for i := range ar.queue {
				if ar.queue[i].IP.Equal(net.IP(p.DstProtAddress)) { // we got a match
					ar.queue[i].RChan <- ARPResponse{
						Err:  nil,
						Time: ar.timeout - time.Now().Sub(ar.queue[i].TimeoutOn),
						MAC:  net.HardwareAddr(p.DstHwAddress),
					}
					ar.queue = append(ar.queue[:i], ar.queue[i+1:]...)
					break
				}
			}
		}
	}
}

// Resolve an IP -> MAC ARP request.  This is blocking, but should timeout.
func (ar *ARPResolver) Resolve(ip net.IP) (mac net.HardwareAddr, e error) {
	rchan := make(chan ARPResponse)
	ar.c <- ARPRequest{
		IP:    ip,
		RChan: rchan,
	}
	r := <-rchan
	if r.Err != nil {
		e = r.Err
		return
	}
	mac = r.MAC
	return
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
