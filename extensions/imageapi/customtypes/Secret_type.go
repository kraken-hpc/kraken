/* Secret_type.go: define methods to make a CustomType that stores a string that won't marshal to JSON
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package customtypes

import (
	"strconv"

	"github.com/hpc/kraken/lib/types"
)

var _ (types.ExtensionCustomType) = Secret{}
var _ (types.ExtensionCustomTypePtr) = (*Secret)(nil)

type Secret struct {
	S string
}

func (m Secret) Marshal() ([]byte, error) {
	return []byte(m.S), nil
}

func (m *Secret) MarshalTo(data []byte) (int, error) {
	copy(data, []byte(m.S))
	return 4, nil
}

func (m *Secret) Unmarshal(data []byte) error {
	m.S = string(data)
	return nil
}

func (m *Secret) Size() int { return len(m.S) }

func (m Secret) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote("XXXX")), nil
}

func (m *Secret) UnmarshalJSON(data []byte) (e error) {
	m.S = string(data)
	return
}
