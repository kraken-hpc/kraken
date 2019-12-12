/* Powerman.go: extension adds special fields to allow for powerman control
*
* Author: R. Eli Snyder <resnyder@lanl.gov>
*
* This software is open source software available under the BSD-3 license.
* Copyright (c) 2018, Triad National Security, LLC
* See LICENSE file for details.
 */

package powerman

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/extensions/Powerman/proto"
	"github.com/hpc/kraken/lib"
)

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/Powerman.proto

/////////////////////
// Powerman Object /
///////////////////

var _ lib.Extension = Powerman{}

type Powerman struct{}

func (Powerman) New() proto.Message {
	return &pb.Powerman{}
}

func (r Powerman) Name() string {
	a, _ := ptypes.MarshalAny(r.New())
	return a.GetTypeUrl()
}

func (Powerman) Context() lib.ExtensionContext {
	return lib.ExtensionContext_PARENT
}

func init() {
	core.Registry.RegisterExtension(Powerman{})
}
