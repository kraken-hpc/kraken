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
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/json"
	"github.com/hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto -I . --go_out=plugins=grpc:. powerman.proto

const Name = "type.googleapis.com/Powerman.Control"

////////////////////
// Control Object /
//////////////////

var _ types.Extension = (*Control)(nil)

func (*Control) New() types.Message {
	return &Control{}
}

func (*Control) Name() string {
	return Name
}

// MarshalJSON creats a JSON version of Node
func (c *Control) MarshalJSON() ([]byte, error) {
	return json.MarshalJSON(c)
}

// UnmarshalJSON populates a node from JSON
func (c *Control) UnmarshalJSON(j []byte) error {
	return json.UnmarshalJSON(j, c)
}

func init() {
	core.Registry.RegisterExtension(&Control{})
}
