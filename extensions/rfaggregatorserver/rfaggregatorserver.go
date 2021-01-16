/* RFAggregatorServer.go: extension adds fields related to aggregator servers.
 *
 * Author: Ghazanfar Ali <ghazanfar.ali@ttu.edu>, Kevin Pelzel <kevinpelzel22@gmail.com>;J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto -I . --go_out=plugins=grpc:. rfaggregatorserver.proto

package rfaggregatorserver

import (
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/json"
)

const Name = "type.googleapis.com/RFAggregator.Server"

/////////////////
// RFAggregatorServer Object /
///////////////

var _ types.Extension = (*Server)(nil)

func (*Server) New() types.Message {
	return &Server{}
}

func (*Server) Name() string {
	return Name
}

// MarshalJSON creats a JSON version of Node
func (s *Server) MarshalJSON() ([]byte, error) {
	return json.MarshalJSON(s)
}

// UnmarshalJSON populates a node from JSON
func (s *Server) UnmarshalJSON(j []byte) error {
	return json.UnmarshalJSON(j, s)
}

func init() {
	core.Registry.RegisterExtension(&Server{})
}
