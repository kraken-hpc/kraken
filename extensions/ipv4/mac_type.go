package ipv4

/* MAC_type.go: define methods to make a CustomType that stores Hardware addresses
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

import (
	fmt "fmt"
	"net"

	"github.com/hpc/kraken/lib/types"
)

var _ (types.ExtensionCustomType) = IP{}
var _ (types.ExtensionCustomTypePtr) = (*IP)(nil)

type MAC struct {
	net.HardwareAddr
}

func (m MAC) Marshal() ([]byte, error) {
	return []byte(m.HardwareAddr), nil
}

func (m *MAC) MarshalTo(data []byte) (int, error) {
	copy(data, []byte(m.HardwareAddr))
	return 4, nil
}

func (m *MAC) Unmarshal(data []byte) error {
	if len(data) > 20 { // IPoIB is longest we allow
		return fmt.Errorf("incorrect hardware address lenght: %d > 20", len(data))
	}
	copy(m.HardwareAddr, data)
	return nil
}

func (m *MAC) Size() int { return len(m.HardwareAddr) }

func (m MAC) MarshalJSON() ([]byte, error) {
	return []byte("\"" + m.String() + "\""), nil
}

func (m *MAC) UnmarshalJSON(data []byte) (e error) {
	m.HardwareAddr, e = net.ParseMAC(string(data))
	return
}
