/* kraken.go: the kraken executable can be used to generate kraken application, module, and extension stubs
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate go-bindata -o templates/templates.go -fs -pkg templates -prefix templates templates/app templates/extension templates/module

package main

import (
	"flag"
	"fmt"
	"go/build"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kraken-hpc/kraken/templates"
	"gopkg.in/yaml.v2"
)

// Globals
var verbose bool
var debug bool
var quiet bool
var force bool

func pError(f string, args ...interface{}) {
	log.Printf("ERROR: "+f, args...)
}

func pFail(f string, args ...interface{}) {
	log.Printf("FAIL: "+f, args...)
	os.Exit(1)
}

func pInfo(f string, args ...interface{}) {
	if !quiet {
		log.Printf("INFO: "+f, args...)
	}
}

func pVerbose(f string, args ...interface{}) {
	if (verbose || debug) && !quiet {
		log.Printf("VERBOSE: "+f, args...)
	}
}

func pDebug(f string, args ...interface{}) {
	if debug && !quiet {
		log.Printf("DEBUG: "+f, args...)
	}
}

// App generation
type AppConfig struct {
	Name       string
	Version    string
	Pprof      bool
	Extensions []string
	Modules    []string
}

func appCompileTemplate(tplFile, tmpDir string, cfg *AppConfig) (target string, e error) {
	var tpl *template.Template
	var out *os.File
	parts := strings.Split(filepath.Base(tplFile), ".")
	target = strings.Join(parts[:len(parts)-1], ".")
	dest := filepath.Join(tmpDir, target)
	if _, err := os.Stat(dest); err == nil {
		if !force {
			return "", fmt.Errorf("refusing to overwrite file: %s (force not specified)", dest)
		}
	}
	tpl = template.New(tplFile)
	if tpl, e = tpl.Parse(string(templates.MustAsset(tplFile))); e != nil {
		return
	}
	if out, e = os.Create(dest); e != nil {
		return
	}
	defer out.Close()
	e = tpl.Execute(out, cfg)
	return
}

func appGenerate(args []string) {
	var configFile string
	var outDir string
	var help bool
	fs := flag.NewFlagSet("app generate", flag.ExitOnError)
	fs.StringVar(&configFile, "c", "kraken.yaml", "name of app config file to use")
	fs.StringVar(&outDir, "o", ".", "output directory for app")
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
	if len(fs.Args()) != 0 {
		pError("unknown option: %s", fs.Args()[0])
		fs.Usage()
		os.Exit(1)
	}

	ts, err := templates.AssetDir("app")
	if err != nil || len(ts) < 2 {
		pFail("no assets found for app generation: %v", err)
	}

	stat, err := os.Stat(outDir)
	if !stat.IsDir() {
		pFail("output directory %s exists, but is not a directory", outDir)
	}
	if err != nil {
		// create the dir
		if err = os.MkdirAll(outDir, 0777); err != nil {
			pFail("failed to create output directory %s: %v", outDir, err)
		}
	}
	cfgData, err := ioutil.ReadFile(configFile)
	if err != nil {
		pFail("could not read config file %s: %v", configFile, err)
	}
	cfg := &AppConfig{}
	if err = yaml.Unmarshal(cfgData, cfg); err != nil {
		pFail("failed to parse config file %s: %v", configFile, err)
	}
	if cfg.Name == "" {
		pFail("an application name must be specified in the app config")
	}
	if cfg.Version == "" {
		pInfo("app version was not specified, will default to v0.0.0")
	}
	for _, t := range ts {
		written, err := appCompileTemplate("app/"+t, outDir, cfg)
		if err != nil {
			pFail("failed to write template: %v", err)
		}
		pVerbose("wrote file: %s", written)
	}
	pInfo("app \"%s\" generated at %s", cfg.Name, outDir)
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
		pError("no app sub-command")
		fs.Usage()
		os.Exit(1)
	}

	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "generate":
		appGenerate(args)
	default:
		pError("unknown app sub-command: %s", cmd)
		fs.Usage()
		os.Exit(1)
	}
}

// Entry point

func main() {
	var help = false
	fs := flag.NewFlagSet("kraken", flag.ContinueOnError)
	usage := func() {
		fmt.Println("Usage: kraken [-fhqv] <command> [options]")
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
	fs.BoolVar(&force, "f", false, "overwrite files if they exist")
	fs.BoolVar(&verbose, "v", false, "verbose output")
	fs.BoolVar(&help, "h", false, "print usage and exit")
	fs.BoolVar(&quiet, "q", false, "suppress informational messages")
	if err := fs.Parse(os.Args[1:]); err != nil {
		pError("failed to parse arguments: %v", err)
		os.Exit(1)
	}
	if help {
		usage()
		os.Exit(0)
	}
	args := fs.Args()
	if len(args) < 1 {
		pError("no command specified")
		usage()
		os.Exit(1)
	}
	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "app":
		cmdApp(args)
	case "module":
		pFail("module command is not yet supported")
	case "extensions":
		pFail("extension command is not yet supported")
	default:
		pError("unknown command: %s", cmd)
		usage()
		os.Exit(1)
	}
}
