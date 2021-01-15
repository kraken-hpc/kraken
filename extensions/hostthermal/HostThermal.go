/* HostThermal.go: extension adds enumerated states for tracking thermal conditions of node components.
 *
 * Author: Ghazanfar Ali <ghazanfar.ali@ttu.edu>;Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package hostthermal

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/extensions/HostThermal/proto"
	"github.com/hpc/kraken/lib"
)

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/Thermal.proto

/////////////////
// HostThermal Object /
///////////////

var _ lib.Extension = HostThermal{}

type HostThermal struct{}

func (HostThermal) New() proto.Message {
	return &pb.HostThermal{}
}

func (r HostThermal) Name() string {
	a, _ := ptypes.MarshalAny(r.New())
	return a.GetTypeUrl()
}

func init() {
	core.Registry.RegisterExtension(HostThermal{})
}
