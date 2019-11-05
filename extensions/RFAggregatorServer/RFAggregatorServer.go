/* RFAggregatorServer.go: extension adds fields related to aggregator servers.
 *
 * Author: Ghazanfar Ali <ghazanfar.ali@ttu.edu>, Kevin Pelzel <kevinpelzel22@gmail.com>;J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package rfaggregatorserver

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/extensions/RFAggregatorServer/proto"
	"github.com/hpc/kraken/lib"
)

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/RFAggregatorServer.proto

/////////////////
// RFAggregatorServer Object /
///////////////

var _ lib.Extension = RFAggregatorServer{}

type RFAggregatorServer struct{}

func (RFAggregatorServer) New() proto.Message {
	return &pb.RFAggregatorServer{}
}

func (r RFAggregatorServer) Name() string {
	a, _ := ptypes.MarshalAny(r.New())
	return a.GetTypeUrl()
}

func init() {
	core.Registry.RegisterExtension(RFAggregatorServer{})
}
