//go:generate go-bindata -o templates/templates.go -fs -pkg templates -prefix templates templates/app

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
