/* global.go: global config for generators
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate go-bindata -o templates/templates.go -fs -pkg templates -prefix templates templates/app templates/module templates/extension

package generators

import (
	log "github.com/sirupsen/logrus"
)

type GlobalConfigType struct {
	Version string
	Log     *log.Logger
	Force   bool
}

var Global *GlobalConfigType
var Log *log.Logger
