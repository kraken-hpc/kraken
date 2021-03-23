/* image.go: extension adds fields for mapping images to nodes
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package imageapi

import (
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto/src -I proto/ --gogo_out=plugins=grpc:. proto/imageapi.proto proto/generated.proto

const Name = "type.googleapis.com/ImageAPI.ImageSet"

/////////////////////
// ImageSet Object /
///////////////////

var _ types.Extension = (*ImageSet)(nil)

func (*ImageSet) New() types.Message {
	return &ImageSet{}
}

func (*ImageSet) Name() string {
	return Name
}

func init() {
	core.Registry.RegisterExtension(&ImageSet{})
}
