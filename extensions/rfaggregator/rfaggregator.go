/* RFAggregatorServer.go: extension adds fields related to aggregator servers.
 *
 * Author: Ghazanfar Ali <ghazanfar.ali@ttu.edu>, Kevin Pelzel <kevinpelzel22@gmail.com>;J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. rfaggregator.proto

package rfaggregator

import (
	"github.com/kraken-hpc/kraken/core"
	"github.com/kraken-hpc/kraken/lib/types"
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

func init() {
	core.Registry.RegisterExtension(&Server{})
}
