/* json.go: json handlers for messages
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package json

import (
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

// Resolver globally sets the proto resolver
// Typically, this will be the kraken Registry
var Resolver jsonpb.AnyResolver

// MarshalJSON is a helper for default JSON marshaling
func MarshalJSON(m proto.Message) ([]byte, error) {
	jm := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "  ",
		OrigName:     false,
		AnyResolver:  Resolver,
	}
	j, err := jm.MarshalToString(m)
	return []byte(j), err
}

// UnmarshalJSON is a helper for default JSON unmarshaling
func UnmarshalJSON(in []byte, p proto.Message) error {
	um := &jsonpb.Unmarshaler{
		AllowUnknownFields: false,
		AnyResolver:        Resolver,
	}
	return um.Unmarshal(strings.NewReader(string(in)), p)
}
