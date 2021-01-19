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
	"bytes"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

// Marshaler is a global marshaler that sets our default options
var Marshaler = jsonpb.Marshaler{}

// Unmarshaler is a global unmarshaler that sets our default options
var Unmarshaler = jsonpb.Unmarshaler{}

// Marshal turns a gogo proto.Message into json with the default marshaler
func Marshal(m proto.Message) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	e := Marshaler.Marshal(buf, m)
	return buf.Bytes(), e
}

// Unmarshal turns a json message into a proto.Message with the default marshaler
func Unmarshal(in []byte, m proto.Message) error {
	buf := bytes.NewBuffer(in)
	return Unmarshaler.Unmarshal(buf, m)
}
