/* IPv4.z.go: this extension adds standard IPv4 and Ethernet properties to the Node state
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto -I . --go_out=plugins=grpc:. IPv4.proto

package ipv4

import (
	"net"

	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/json"
	"github.com/hpc/kraken/lib/types"
)

const Name = "type.googleapis.com/IPv4.IPv4OverEthernet"

/////////////////////////////
// IPv4OverEthernet Object /
///////////////////////////

var _ types.Extension = (*IPv4OverEthernet)(nil)

func (i *IPv4OverEthernet) New() types.Message {
	return &IPv4OverEthernet{
		/*
				Ifaces: []*pb.IPv4OverEthernet_ConfiguredInterface{
					&pb.IPv4OverEthernet_ConfiguredInterface{
						Eth: &pb.Ethernet{},
						Ip:  &pb.IPv4{},
					},
					&pb.IPv4OverEthernet_ConfiguredInterface{
						Eth: &pb.Ethernet{},
						Ip:  &pb.IPv4{},
					},
					&pb.IPv4OverEthernet_ConfiguredInterface{
						Eth: &pb.Ethernet{},
						Ip:  &pb.IPv4{},
					},
					&pb.IPv4OverEthernet_ConfiguredInterface{
						Eth: &pb.Ethernet{},
						Ip:  &pb.IPv4{},
					},
				},
				Routes: []*pb.IPv4{
					&pb.IPv4{},
					&pb.IPv4{},
					&pb.IPv4{},
					&pb.IPv4{},
				},
				DnsNameservers: []*pb.IPv4{
					&pb.IPv4{},
					&pb.IPv4{},
					&pb.IPv4{},
					&pb.IPv4{},
				},
			Hostname: &pb.DNSA{Ip: &pb.IPv4{}},
		*/
	}
}

func (*IPv4OverEthernet) Name() string {
	return Name
}

// BytesToIP converts 4 bytes to a net.IP
// returns nil if you don't give it 4 bytes
func BytesToIP(b []byte) (ip net.IP) {
	if len(b) != 4 {
		return
	}
	return net.IPv4(b[0], b[1], b[2], b[3])
}

func BytesToMAC(b []byte) (hw net.HardwareAddr) {
	if len(b) != 6 {
		return
	}
	return net.HardwareAddr(b)
}

// MarshalJSON creats a JSON version of Node
func (n *IPv4OverEthernet) MarshalJSON() ([]byte, error) {
	return json.MarshalJSON(n)
}

// UnmarshalJSON populates a node from JSON
func (n *IPv4OverEthernet) UnmarshalJSON(j []byte) error {
	return json.UnmarshalJSON(j, n)
}

func init() {
	core.Registry.RegisterExtension(&IPv4OverEthernet{})
}
