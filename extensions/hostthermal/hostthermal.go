/* HostThermal.go: extension adds enumerated states for tracking thermal conditions of node components.
 *
 * Author: Ghazanfar Ali <ghazanfar.ali@ttu.edu>;Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package hostthermal

import (
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/json"
	"github.com/hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto -I . --go_out=plugins=grpc:. hostthermal.proto

const Name = "type.googleapis.com/HostThermal.Temp"

/////////////////////////
// HostThermal Object //
///////////////////////

var _ types.Extension = (*Temp)(nil)

func (*Temp) New() types.Message {
	return &Temp{}
}

func (*Temp) Name() string {
	return Name
}

// MarshalJSON creats a JSON version of Node
func (t *Temp) MarshalJSON() ([]byte, error) {
	return json.MarshalJSON(t)
}

// UnmarshalJSON populates a node from JSON
func (t *Temp) UnmarshalJSON(j []byte) error {
	return json.UnmarshalJSON(j, t)
}

func init() {
	core.Registry.RegisterExtension(&Temp{})
}
