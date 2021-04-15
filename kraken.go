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
	"path/filepath"
)

const modPath = "github.com/kraken-hpc/kraken" // TODO: find a less static way?

// Globals
var verbose bool
var debug bool
var templatePath string

type Empty struct{} // this is used to lookup the modpath

func cmdApp(args []string) error {
	return nil
}

func main() {
	var help = false
	fs := flag.NewFlagSet("kraken", flag.ContinueOnError)
	usage := func() {
		fmt.Println("Usage: kraken [-htv] <command> [options]")
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
	defaultTemplatePath := filepath.Join(gopath, build.Default.Dir)
	defaultTemplatePath = filepath.Join(defaultTemplatePath, "templates")
	fmt.Println("tmpl: " + defaultTemplatePath)
	fs.BoolVar(&verbose, "v", false, "verbose output")
	fs.BoolVar(&help, "h", false, "print usage and exit")
	fs.StringVar(&templatePath, "t", filepath.Join(build.Default.GOPATH, "src", modPath, "templates"), "specify path to code generation templates")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Println(err)
	}
	if help {
		usage()
		os.Exit(0)
	}
	args := fs.Args()
	fmt.Printf("%v\n", args)
	if len(args) < 1 {
		fmt.Println("No command specified.")
		usage()
		os.Exit(1)
	}
	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "app":
		if err := cmdApp(args); err != nil {

		}
	case "module":
		fmt.Println("module command is not yet supported")
	case "extensions":
		fmt.Println("extension command is not yet supported")
	default:
		fmt.Println("unknown command: " + cmd)
		usage()
		os.Exit(1)
	}
}
