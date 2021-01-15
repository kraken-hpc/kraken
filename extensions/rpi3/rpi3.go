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
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/json"
	"github.com/hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto -I . --go_out=plugins=grpc:. rpi3.proto

const Name = "type.googleapis.com/RPi3.Pi"

/////////////////
// Pi Object /
///////////////

var _ types.Extension = (*Pi)(nil)

func (*Pi) New() types.Message {
	return &Pi{}
}

func (*Pi) Name() string {
	return Name
}

// MarshalJSON creats a JSON version of Node
func (p *Pi) MarshalJSON() ([]byte, error) {
	return json.MarshalJSON(p)
}

// UnmarshalJSON populates a node from JSON
func (p *Pi) UnmarshalJSON(j []byte) error {
	return json.UnmarshalJSON(j, p)
}

func init() {
	core.Registry.RegisterExtension(&Pi{})
}
