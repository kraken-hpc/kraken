/* IP_type.go: define methods to make a CustomType that stores IP addresses
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018-2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package ipv4

import (
	fmt "fmt"
	"net"

	"github.com/hpc/kraken/lib/types"
)

var _ (types.ExtensionCustomType) = IP{}
var _ (types.ExtensionCustomTypePtr) = (*IP)(nil)

type IP struct {
	net.IP
}

func (i IP) Marshal() ([]byte, error) {
	return []byte(i.To4()), nil
}

func (i *IP) MarshalTo(data []byte) (int, error) {
	copy(data, []byte(i.To4()))
	return 4, nil
}

func (i *IP) Unmarshal(data []byte) error {
	if len(data) != 4 {
		return fmt.Errorf("incorrect IP address lenght: %d != 4", len(data))
	}
	copy(i.IP, data)
	return nil
}

func (i *IP) Size() int { return 4 }

func (i IP) MarshalJSON() ([]byte, error) {
	return i.MarshalText()
}

func (i *IP) UnmarshalJSON(data []byte) error {
	return i.UnmarshalText(data)
}
