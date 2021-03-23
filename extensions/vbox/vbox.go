/* rpi3.go: extension adds special fields for Bitscope/Raspberry Pi 3B(+) management
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package vbox

import (
	"github.com/kraken-hpc/kraken/core"
	"github.com/kraken-hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. vbox.proto

const Name = "type.googleapis.com/VBox.VirtualMachine"

/////////////////
// VBox Object /
///////////////

var _ types.Extension = (*VirtualMachine)(nil)

func (*VirtualMachine) New() types.Message {
	return &VirtualMachine{}
}

func (*VirtualMachine) Name() string {
	return Name
}

func init() {
	core.Registry.RegisterExtension(&VirtualMachine{})
}
