/* kraken-build.go: builds kraken binaries based on a YAML specification
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
	"flag"
	"go/build"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// globals (set by flags)
var cfgFile, buildDir *string
var noCleanup, force, verbose *bool

// config
var cfg *Config

// Target spec
type Target struct {
	Os   string
	Arch string
}

// Config yaml file structure
type Config struct {
	Targets    map[string]Target
	Extensions []string
	Modules    []string
	Instances  []struct {
		Name      string
		Module    string
		Config    interface{}
		Requires  map[string]string
		Excludes  map[string]string
		Mutations map[string]struct {
			Mutates map[string]struct {
				From string
				To   string
			}
			Requires map[string]string
			Excludes map[string]string
		}
	}
}

func compileTemplate(tplFile, tmpDir string) (target string, e error) {
	var tpl *template.Template
	var out *os.File
	parts := strings.Split(filepath.Base(tplFile), ".")
	target = strings.Join(parts[:len(parts)-1], ".")
	if tpl, e = template.ParseFiles(tplFile); e != nil {
		return
	}
	if out, e = os.Create(filepath.Join(tmpDir, target)); e != nil {
		return
	}
	defer out.Close()
	e = tpl.Execute(out, cfg)
	return
}

func compileTemplates(krakenDir, tmpDir string) (targets []string, e error) {
	var files []os.FileInfo
	re, _ := regexp.Compile(".*\\.go\\.tpl$")
	// build a list of all of the templates
	files, e = ioutil.ReadDir(filepath.Join(krakenDir, "kraken"))
	if e != nil {
		return
	}
	for _, f := range files {
		if f.Mode().IsRegular() {
			if re.MatchString(f.Name()) { // ends in .go.tpl?
				if *verbose {
					log.Printf("executing template: %s", f.Name())
				}
				var target string
				target, e = compileTemplate(filepath.Join(krakenDir, "kraken", f.Name()), tmpDir)
				if e != nil {
					return
				}
				targets = append(targets, target)
			}
		}
	}
	return
}

func buildKraken(dir string, fromTemplates []string, t Target, verbose bool) (e error) {
	// setup log file
	var f *os.File
	if verbose {
		f = os.Stderr
	} else {
		f, e = os.OpenFile(filepath.Join(dir, "log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if e != nil {
			return e
		}
		defer f.Close()
	}

	args := []string{"build", "main.go"}
	args = append(args, fromTemplates...)
	cmd := exec.Command("go", args...)
	if verbose {
		log.Printf("Run: %s", strings.Join(cmd.Args, " "))
	}
	cmd.Dir = dir

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	cmd.Env = append(cmd.Env, "GOOS="+t.Os)
	cmd.Env = append(cmd.Env, "GOARCH="+t.Arch)
	cmd.Env = append(cmd.Env, "GOPATH="+build.Default.GOPATH)
	cmd.Env = append(cmd.Env, "GOROOT="+build.Default.GOROOT)

	cmd.Stdout = f
	cmd.Stderr = f
	e = cmd.Run()
	return
}
func main() {
	var e error
	cfgFile = flag.String("config", "config/kraken.yaml", "specify the build configuration YAML file")
	buildDir = flag.String("dir", "build", "specify directory to put built binaries in")
	noCleanup = flag.Bool("noclean", false, "don't cleanup temp dir after build")
	force = flag.Bool("force", false, "force will overwrite existing build targets")
	verbose = flag.Bool("v", false, "verbose will print extra information about the build process")
	flag.Parse()

	// read config
	cfgBytes, e := ioutil.ReadFile(*cfgFile)
	if e != nil {
		log.Fatalf("could not read config file: %v", e)
	}
	cfg = &Config{}
	if e = yaml.Unmarshal(cfgBytes, cfg); e != nil {
		log.Fatalf("could not read config: %v", e)
	}

	// create build dir
	if _, e = os.Stat(*buildDir); os.IsNotExist(e) {
		if e = os.Mkdir(*buildDir, 0755); e != nil {
			log.Fatalf("could not create build directory: %v", e)
		}
	}

	p, e := build.Default.Import("github.com/hpc/kraken", "", build.FindOnly)
	if e != nil {
		log.Fatalf("couldn't find kraken package: %v", e)
	}
	krakenDir := p.Dir
	log.Printf("using kraken at: %s", krakenDir)

	tmpDir := filepath.Join(krakenDir, "tmp") // make an option to change where this is?
	os.Mkdir(tmpDir, 0755)

	// setup build environment
	log.Println("setting up build environment")

	// hardlink kraken.go to tmpDir
	os.Link(filepath.Join(krakenDir, "kraken/main.go"), filepath.Join(tmpDir, "main.go"))

	// build templates
	var fromTemplates []string
	if fromTemplates, e = compileTemplates(krakenDir, tmpDir); e != nil {
		log.Fatalf("could not compile templates: %v", e)
	}

	// build
	for t := range cfg.Targets {
		log.Printf("building: %s (GOOS: %s, GOARCH; %s)", t, cfg.Targets[t].Os, cfg.Targets[t].Arch)
		if e = buildKraken(tmpDir, fromTemplates, cfg.Targets[t], *verbose); e != nil {
			log.Printf("failed to build %s: %v", t, e)
			continue
		}
		path := filepath.Join(*buildDir, "kraken-"+t)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			if *force {
				log.Printf("force was specified, overwriting old build: %s", path)
				os.Remove(path)
			} else {
				log.Printf("refusing to overwrite old build, use -force to override: %s", path)
			}
		}
		if e = os.Link(filepath.Join(tmpDir, "main"), path); e != nil {
			log.Printf("failed to link executable %s: %v", path, e)
			continue
		}
	}

	if !*noCleanup { // cleanup now
		os.RemoveAll(tmpDir)
	} else {
		log.Printf("leaving temp directory: %s", tmpDir)
	}
}
