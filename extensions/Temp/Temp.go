/* rpi3.go: extension adds special fields for Bitscope/Raspberry Pi 3B(+) management
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package temp

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/extensions/Temp/proto"
	"github.com/hpc/kraken/lib"
)

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/Temp.proto

/////////////////
// Temp Object /
///////////////

var _ lib.Extension = Temp{}

type Temp struct{}

func (Temp) New() proto.Message {
	return &pb.Temp{}
}

func (r Temp) Name() string {
	a, _ := ptypes.MarshalAny(r.New())
	return a.GetTypeUrl()
}

func (Temp) Context() lib.ExtensionContext {
	return lib.ExtensionContext_PARENT
}

func init() {
	core.Registry.RegisterExtension(Temp{})
}
