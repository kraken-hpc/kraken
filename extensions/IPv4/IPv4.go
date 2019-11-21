/* IPv4.go: this extension adds standard IPv4 and Ethernet properties to the Node state
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/IPv4.proto

package IPv4

import (
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/extensions/IPv4/proto"
	"github.com/hpc/kraken/lib"
)

/////////////////////////////
// IPv4OverEthernet Object /
///////////////////////////

var _ lib.Extension = IPv4OverEthernet{}

// IPv4OverEthernet is a ProtoMessage wrapper around IPv4 protobuf
type IPv4OverEthernet struct{}

func (i IPv4OverEthernet) New() proto.Message {
	return &pb.IPv4OverEthernet{
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

func (i IPv4OverEthernet) Name() string {
	a, _ := ptypes.MarshalAny(i.New())
	return a.GetTypeUrl()
}

// Returns an empty map because none of these values are enumerable
func (i IPv4OverEthernet) EnumerableValues() map[string][]string {
	var emptyMap map[string][]string
	return emptyMap
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

func init() {
	core.Registry.RegisterExtension(IPv4OverEthernet{})
}
