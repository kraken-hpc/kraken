/* PXE.go: extension adds special fields tracking generic PXE/iPXE state
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package pxe

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/extensions/PXE/proto"
	"github.com/hpc/kraken/lib"
)

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/PXE.proto

/////////////////
// PXE Object /
///////////////

var _ lib.Extension = PXE{}

type PXE struct{}

func (PXE) New() proto.Message {
	return &pb.PXE{}
}

func (r PXE) Name() string {
	a, _ := ptypes.MarshalAny(r.New())
	return a.GetTypeUrl()
}

func (r PXE) EnumerableValues() map[string][]string {
	enumMap := make(map[string][]string)
	enumMap["state"] = []string{pb.PXE_NONE.String(), pb.PXE_WAIT.String(), pb.PXE_INIT.String(), pb.PXE_COMP.String()}
	enumMap["method"] = []string{pb.PXE_PXE.String(), pb.PXE_iPXE.String()}
	return enumMap
}

func init() {
	core.Registry.RegisterExtension(PXE{})
}
