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
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/json"
	"github.com/hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto -I . --go_out=plugins=grpc:. vbox.proto

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

// MarshalJSON creats a JSON version of Node
func (n *VirtualMachine) MarshalJSON() ([]byte, error) {
	return json.MarshalJSON(n)
}

// UnmarshalJSON populates a node from JSON
func (n *VirtualMachine) UnmarshalJSON(j []byte) error {
	return json.UnmarshalJSON(j, n)
}

func init() {
	core.Registry.RegisterExtension(&VirtualMachine{})
}
