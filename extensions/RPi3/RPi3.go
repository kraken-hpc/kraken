/* rpi3.go: extension adds special fields for Bitscope/Raspberry Pi 3B(+) management
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package rpi3

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/extensions/RPi3/proto"
	"github.com/hpc/kraken/lib"
)

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/RPi3.proto

/////////////////
// RPi3 Object /
///////////////

var _ lib.Extension = RPi3{}

type RPi3 struct{}

func (RPi3) New() proto.Message {
	return &pb.RPi3{}
}

func (r RPi3) Name() string {
	a, _ := ptypes.MarshalAny(r.New())
	return a.GetTypeUrl()
}

func (r RPi3) EnumerableValues() map[string][]string {
	enumMap := make(map[string][]string)
	enumMap["pxe"] = []string{pb.RPi3_NONE.String(), pb.RPi3_WAIT.String(), pb.RPi3_INIT.String(), pb.RPi3_COMP.String()}
	enumMap["model"] = []string{pb.RPi3_ThreeB.String(), pb.RPi3_ThreeBPlus.String()}
	return enumMap
}

func init() {
	core.Registry.RegisterExtension(RPi3{})
}
