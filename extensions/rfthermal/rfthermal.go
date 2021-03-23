/* RFThermal.go: extension adds enumerated states for tracking thermal conditions of node components.
 *
 * Author: Ghazanfar Ali <ghazanfar.ali@ttu.edu>;Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package rfthermal

import (
	"github.com/kraken-hpc/kraken/core"
	"github.com/kraken-hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. rfthermal.proto

const Name = "type.googleapis.com/RFThermal.Temp"

//////////////////////
// RFThermal Object /
////////////////////

var _ types.Extension = (*Temp)(nil)

func (*Temp) New() types.Message {
	return &Temp{}
}

func (*Temp) Name() string {
	return Name
}

func init() {
	core.Registry.RegisterExtension(&Temp{})
}
