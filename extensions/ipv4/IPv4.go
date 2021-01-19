/* IPv4.go: this extension adds standard IPv4 and Ethernet properties to the Node state
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=grpc:. IPv4.proto

package ipv4

import (
	"github.com/hpc/kraken/core"
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

func init() {
	core.Registry.RegisterExtension(&IPv4OverEthernet{})
}
