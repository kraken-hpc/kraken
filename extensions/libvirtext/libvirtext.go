/* libvirt.go: extension adds needed fields for LibVirt Virtual Machine management
 *
 * Author: Benjamin S. Allen <bsallen@alcf.anl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package libvirtext

import (
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. libvirtext.proto

// Name is FQDN of the LibVirt VirtualMachine object
const Name = "type.googleapis.com/LibVirtExt.VirtualMachine"

///////////////////////////////////
// LibVirtExt Virtual Machine Object /
///////////////////////////////////

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
