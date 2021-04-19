/* kraken.go: the kraken executable can be used to generate kraken application, module, and extension stubs
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
	"flag"
	"fmt"
	"go/build"
	"os"

	"github.com/kraken-hpc/kraken/generators"
	log "github.com/sirupsen/logrus"
)

var Global = &generators.GlobalConfigType{
	Version: "v0.2.0",
}

var strToLL = map[string]log.Level{
	"panic": log.PanicLevel,
	"fatal": log.FatalLevel,
	"error": log.ErrorLevel,
	"warn":  log.WarnLevel,
	"info":  log.InfoLevel,
	"debug": log.DebugLevel,
	"trace": log.TraceLevel,
}

func cmdApp(args []string) {
	var help bool
	fs := flag.NewFlagSet("app", flag.ExitOnError)
	fs.BoolVar(&help, "h", false, "print this usage")
	fs.Usage = func() {
		fmt.Println("Usage: kraken <opts> app [-h] [command] [opts]")
		fmt.Println("Commands:")
		fmt.Println("\tgenerate")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	if help {
		fs.Usage()
		os.Exit(0)
	}

	args = fs.Args()
	if len(args) < 1 {
		Log.Fatal("no app sub-command")
	}

	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "generate":
		generators.AppGenerate(Global, args)
	default:
		Log.Errorf("unknown app sub-command: %s", cmd)
		fs.Usage()
		os.Exit(1)
	}
}

// Entry point
var Log *log.Logger

func main() {
	Log = log.New()
	Global.Log = Log
	var help = false
	fs := flag.NewFlagSet("kraken", flag.ContinueOnError)
	usage := func() {
		fmt.Println("kraken is a code-generator for kraken-based applications, modules, and extensions")
		fmt.Println()
		fmt.Println("Usage: kraken [-fv] [-l <log_level>] <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("\tapp")
		fmt.Println("\tmodule")
		fmt.Println("\textension")
		fmt.Println("For command help: kraken <command> -h")
		fs.PrintDefaults()
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	var logLevel string
	fs.BoolVar(&Global.Force, "f", false, "overwrite files if they exist")
	fs.StringVar(&logLevel, "l", "info", "Log level.  Valid values: panic, fatal, error, warn, info, debug, trace.")
	fs.BoolVar(&help, "h", false, "print usage and exit")
	if err := fs.Parse(os.Args[1:]); err != nil {
		Log.Fatalf("failed to parse arguments: %v", err)
	}
	if help {
		usage()
		os.Exit(0)
	}
	if ll, ok := strToLL[logLevel]; ok {
		Log.SetLevel(ll)
	} else {
		Log.Fatalf("unknown log level: %s", logLevel)
	}
	args := fs.Args()
	if len(args) < 1 {
		log.Error("no command specified")
		usage()
		os.Exit(1)
	}
	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "app":
		cmdApp(args)
	case "module":
		Log.Fatal("module command is not yet supported")
	case "extensions":
		Log.Fatal("extension command is not yet supported")
	default:
		Log.Fatal("unknown command: %s", cmd)
	}
}
