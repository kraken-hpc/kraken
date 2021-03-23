/* PowerAPI.go: extenion to use PowerAI (github.com/kraken-hpc/powerapi)
*
* Author: J. Lowell Wofford
*
* This software is open source software available under the BSD-3 license.
* Copyright (c) 2020, Triad National Security, LLC
* See LICENSE file for details.
 */

package powerapi

import (
	"github.com/kraken-hpc/kraken/core"
	"github.com/kraken-hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. powerapi-node.proto

const Name = "type.googleapis.com/PowerAPI.Node"

////////////////////
// Control Object /
//////////////////

var _ types.Extension = (*Node)(nil)

func (*Node) New() types.Message {
	return &Node{}
}

func (*Node) Name() string {
	return Name
}

func init() {
	core.Registry.RegisterExtension(&Node{})
}
