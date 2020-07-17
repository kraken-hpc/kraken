/* kraken-build.go: builds kraken binaries based on a YAML specification
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/hpc/kraken/lib"
	"go/build"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	cp "github.com/hpc/kraken/lib/copy"
	yaml "gopkg.in/yaml.v2"
)

const KrModStr string = "module github.com/hpc/kraken"

// globals (set by flags)
var (
	cfgFile   = flag.String("config", "config/kraken.yaml", "specify the build configuration YAML file")
	buildDir  = flag.String("dir", "build", "specify directory to put built binaries in")
	noCleanup = flag.Bool("noclean", false, "don't cleanup temp dir after build")
	force     = flag.Bool("force", false, "force will overwrite existing build targets")
	verbose   = flag.Bool("v", false, "verbose will print extra information about the build process")
	race      = flag.Bool("race", false, "build with -race, warning: enables CGO")
	pprof     = flag.Bool("pprof", false, "build with pprof support")
)

// config
var cfg *Config

// Target spec
type Target struct {
	Os   string
	Arch string
}

// Config yaml file structure
type Config struct {
	Pprof      bool
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

// compileTemplates looks for Go templates (*.go.tpl) in krakenDir and
// compiles them into tmpDir. If forUroot is true, the compiled templates
// will have their import paths modified to use dependencies included
// with the generated source tree instead of those in the normal kraken
// source tree.
func compileTemplates(krakenDir, tmpDir string, forUroot bool) (targets []string, e error) {
	var files []os.FileInfo
	re, _ := regexp.Compile(".*\\.go\\.tpl$")
	// build a list of all of the templates
	if files, e = ioutil.ReadDir(filepath.Join(krakenDir, "kraken")); e != nil {
		return
	}
	for _, f := range files {
		if f.Mode().IsRegular() {
			if re.MatchString(f.Name()) { // ends in .go.tpl?
				if *verbose {
					log.Printf("executing template: %s", f.Name())
				}
				var target string
				if target, e = compileTemplate(filepath.Join(krakenDir, "kraken", f.Name()), tmpDir); e != nil {
					return
				}

				// Use import paths of generated files for u-root instead of the main
				// kraken ones
				if forUroot {
					targPath := path.Join(tmpDir, target)
					// TODO: Dynamically find kraken's import path
					// instead of hard-coding it.
					// See: https://github.com/hpc/kraken/issues/184
					from := "hpc/kraken/"
					to := "hpc/kraken/build/u-root/"
					if *verbose {
						log.Printf("%s: %s -> %s", targPath, from, to)
					}
					if e = lib.SearchAndReplace(targPath, from, to); e != nil {
						return
					}
				}

				targets = append(targets, target)
			}
		}
	}
	return
}

// copyModules reads a Config struct and copies only the modules specified
// from source to destination
func copyModules(src, dst string, config *Config) (e error) {
	// Create "modules" dir in u-root build dir
	var modStat os.FileInfo
	if modStat, e = os.Stat(src); e != nil {
		log.Printf("could not get file information for %s", src)
		return
	}
	if e = os.Mkdir(dst, modStat.Mode()); e != nil {
		if *force {
			if e = os.RemoveAll(dst); e != nil {
				return
			}
			if e = os.Mkdir(dst, modStat.Mode()); e != nil {
				return
			}
		} else {
			log.Printf("error creating \"%s\" directory; use '-force' to override", dst)
			return
		}
	}

	// Create copy options
	opts := cp.GetDefaultOptions()
	opts.Recursive = true
	// Overwrite preexisting file(s) if -force passed
	if *force {
		opts.Clobber = true
	}

	for _, module := range config.Modules {
		// Get name of specified module in config file
		modName := path.Base(module)

		// Copy module from regular "modules" directory to
		// that in the u-root directory
		modFrom := filepath.Join(src, modName)
		modTo := filepath.Join(dst, modName)
		if e = opts.Copy(modFrom, modTo); e != nil {
			return
		}
	}
	return
}

// copyExtensions reads a Config struct and copies only the extensions
// specified from source to destination
func copyExtensions(src, dst string, config *Config) (e error) {
	// Create "extensions" dir in u-root build dir
	var extStat os.FileInfo
	if extStat, e = os.Stat(src); e != nil {
		log.Printf("could not get file information for %s", src)
		return
	}
	if e = os.Mkdir(dst, extStat.Mode()); e != nil {
		if *force {
			if e = os.RemoveAll(dst); e != nil {
				return
			}
			if e = os.Mkdir(dst, extStat.Mode()); e != nil {
				return
			}
		} else {
			log.Printf("error creating \"%s\" directory; use '-force' to override", dst)
			return
		}
	}

	// Create copy options
	opts := cp.GetDefaultOptions()
	opts.Recursive = true
	if *force {
		opts.Clobber = true
	}

	for _, extension := range config.Extensions {
		// Get name of specified extension in config file
		extName := path.Base(extension)

		// Copy extension from regular "extensions" directory to
		// that in the u-root directory
		extFrom := filepath.Join(src, extName)
		extTo := filepath.Join(dst, extName)
		if e = opts.Copy(extFrom, extTo); e != nil {
			return
		}
	}
	return
}

// copySources copies kraken source files from src to dst
func copySources(src, dst string) (e error) {
	// Create copy options
	opts := cp.GetDefaultOptions()
	opts.Recursive = true
	// Overwrite preexisting file(s) if -force passed
	if *force {
		opts.Clobber = true
	}

	e = opts.Copy(src, dst)
	return
}

// modifySources modifies the source files in the passed dir to satisfy u-root
func modifySources(dir string) (e error) {
	// Replace regular import paths in all of the generated source files with
	// import paths using the build dir path
	var from, to, base string
	base = "\"github.com/hpc/kraken/"
	from = base + "(?:build/u-root/)?"
	to = base + "build/u-root/"
	if *verbose {
		log.Printf("%s: updating import paths to match u-root build dir", dir)
	}
	if e = lib.RegexReplace(dir, from, to); e != nil {
		e = fmt.Errorf("unable to replace import paths for files in %s: %v", dir, e)
		return
	}

	// ServiceInstance.go: BusyBox cannot handle argv[0] not being kraken.
	// TODO: This is a hack that deserves a better fix.
	// See: https://github.com/hpc/kraken/issues/185
	siPath := path.Join(dir, "core/ServiceInstance.go")
	e = lib.SearchAndReplace(siPath,
		"si.cmd.Args = []string{\"[kraken:\" + si.ID() + \"]\"}",
		"si.cmd.Args = []string{os.Args[0]}")
	if e != nil {
		e = fmt.Errorf("unable to modify %s: %v", siPath, e)
		return
	}

	// Rename "main.go" to "kraken.go" for better identification in u-root
	kDir := path.Join(dir, "kraken")
	pathMainGo := path.Join(kDir, "main.go")
	pathKrakenGo := path.Join(kDir, "kraken.go")
	if *verbose {
		log.Printf("renaming \"%s\" to \"%s\"", pathMainGo, pathKrakenGo)
	}
	if e = os.Rename(pathMainGo, pathKrakenGo); e != nil {
		e = fmt.Errorf("unable to rename %s into %s: %v", pathMainGo, pathKrakenGo, e)
	}
	return
}

// buildUrootKraken generates a kraken source tree for a u-root build target,
// in order to be used as a u-root command in an initramfs
func buildUrootKraken(config *Config, outDir, krakenDir string) (targets []string, e error) {
	// Create output directory if nonexistent
	if e = os.MkdirAll(outDir, 0755); e != nil {
		log.Printf("error locating/creating directory for u-root-embeddable kraken source tree")
		return
	} else {
		if *verbose {
			log.Printf("created/found directory \"%s\" for generated kraken source tree", outDir)
		}
	}

	// Create directory within outDir for template compilation output
	srcDir := filepath.Join(outDir, "kraken") // Needed by u-root: name of result binary
	os.Mkdir(srcDir, 0755)

	// Generate kraken source from templates into outDir
	if _, e = compileTemplates(krakenDir, srcDir, true); e != nil {
		log.Printf("error compiling templates for u-root-embeddable kraken source tree")
		return
	} else {
		if *verbose {
			log.Printf("generated kraken source tree for u-root in %s", outDir)
		}
	}

	// Copy needed files from krakenDir to outDir
	files := []string{"core", "lib", "extensions", "modules"}
	for _, file := range files {
		if *verbose {
			log.Printf("copying \"%s\" to \"%s\"", file, outDir)
		}

		// Copy each file from krakenDir to outDir (u-root destination)
		inFile := path.Join(krakenDir, file)
		outFile := path.Join(outDir, file)

		// Specially handle extensions and modules (only copy those that
		// are specified by the config file)
		switch file {
		case "extensions":
			if e = copyExtensions(inFile, outFile, config); e != nil {
				e = fmt.Errorf("unable to copy extensions from %s to %s: %v", inFile, outFile, e)
				return
			}
		case "modules":
			if e = copyModules(inFile, outFile, config); e != nil {
				e = fmt.Errorf("unable to copy modules from %s to %s: %v", inFile, outFile, e)
				return
			}
		default:
			if e = copySources(inFile, outFile); e != nil {
				e = fmt.Errorf("unable to copy sources from %s to %s: %v", inFile, outFile, e)
				return
			}
		}
	}

	// Make necessary changes to copied source to satisfy u-root
	if e = modifySources(outDir); e != nil {
		e = fmt.Errorf("unable to modify sources: %v", e)
		return
	}

	return
}

// buildMainKraken builds a kraken binary for a non-u-root build target.
func buildMainKraken(dir string, fromTemplates []string, tName string, t Target, verbose bool) (e error) {
	// setup log file
	var f *os.File
	if verbose {
		f = os.Stderr
	} else {
		if f, e = os.OpenFile(filepath.Join(dir, "log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); e != nil {
			return e
		}
		defer f.Close()
	}

	args := []string{"build", "-o", "main"}
	if *race {
		args = append(args, "-race")
	}
	args = append(args, fromTemplates...)
	cmd := exec.Command("go", args...)
	if verbose {
		log.Printf("Run: %s", strings.Join(cmd.Args, " "))
	}
	cmd.Dir = dir

	cmd.Env = os.Environ()
	if *race {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	} else {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}
	cmd.Env = append(cmd.Env, "GOOS="+t.Os)
	cmd.Env = append(cmd.Env, "GOARCH="+t.Arch)
	cmd.Env = append(cmd.Env, "GOPATH="+build.Default.GOPATH)
	cmd.Env = append(cmd.Env, "GOROOT="+build.Default.GOROOT)

	cmd.Stdout = f
	cmd.Stderr = f
	e = cmd.Run()

	path := filepath.Join(*buildDir, "kraken-"+tName)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		if *force {
			log.Printf("force was specified, overwriting old build: %s", path)
			os.Remove(path)
		} else {
			log.Printf("refusing to overwrite old build, use -force to override: %s", path)
		}
	}
	if e = os.Link(filepath.Join(dir, "main"), path); e != nil {
		e = fmt.Errorf("failed to link executable %s: %v", path, e)
	}
	return
}

func getModDir() (d string, e error) {
	// first, are we sitting in the Krakend dir?
	pwd, _ := os.Getwd()
	var f *os.File
	if f, e = os.Open(path.Join(pwd, "go.mod")); e == nil {
		defer f.Close()
		rd := bufio.NewReader(f)
		var line []byte
		if line, _, e = rd.ReadLine(); e == nil {
			if string(line) == KrModStr {
				d = pwd
				return
			}
		}
	}

	// couldn't open go.mod; obviously not in pwd, try for GOPATh
	var p *build.Package
	if p, e = build.Default.Import("github.com/hpc/kraken", "", build.FindOnly); e == nil {
		d = p.Dir
		return
	}
	e = fmt.Errorf("couldn't find craken in either PWD or GOPATH")
	return
}

// readConfig reads in a YAML configuration for kraken and
// returns a Config struct populated with its values
func readConfig(cfgFile string) (cfg *Config, e error) {
	var cfgBytes []byte
	if cfgBytes, e = ioutil.ReadFile(cfgFile); e != nil {
		e = fmt.Errorf("could not read config file: %v", e)
		return
	}

	cfg = &Config{}
	if e = yaml.Unmarshal(cfgBytes, cfg); e != nil {
		e = fmt.Errorf("could not read config: %v", e)
		return
	}
	if *pprof {
		cfg.Pprof = true
	}
	return
}

// krakenBuild is the wrapper function that builds kraken. It reads
// the Config struct, compiles the necessary templates, and
// builds kraken for the specified build targets.
func krakenBuild(cfg *Config, krakenDir, tmpDir string) (e error) {
	// Determine which build targets are present to only
	// compile needed templates.
	var haveUrootTarget, haveOtherTarget bool = false, false
	if _, ok := cfg.Targets["u-root"]; ok {
		haveUrootTarget = true
	}
	for t := range cfg.Targets {
		if t != "u-root" {
			haveOtherTarget = true
			break
		}
	}

	// Create build dir
	log.Printf("setting up build environment")
	if _, e = os.Stat(*buildDir); os.IsNotExist(e) {
		if e = os.Mkdir(*buildDir, 0755); e != nil {
			log.Fatalf("could not create build directory: %v", e)
		}
	}

	if haveUrootTarget {
		urootBuildDir := filepath.Join(*buildDir, "u-root")
		log.Printf("building: kraken source tree for u-root located at: %s", urootBuildDir)

		// Create separate directory for generated source
		// to distinguish from other build targets
		if _, e = os.Stat(urootBuildDir); os.IsNotExist(e) {
			if e = os.Mkdir(urootBuildDir, 0755); e != nil {
				log.Fatalf("could not create build directory: %v", e)
			}
		}

		// Perform uroot source generation
		if _, e = buildUrootKraken(cfg, urootBuildDir, krakenDir); e != nil {
			log.Fatalf("could not create source tree for u-root: %v", e)
		}
	}
	if haveOtherTarget {
		// Create temporary dir for compiled templates
		os.Mkdir(tmpDir, 0755)

		var fromTemplates []string
		if fromTemplates, e = compileTemplates(krakenDir, tmpDir, false); e != nil {
			e = fmt.Errorf("could not compile templates: %v", e)
			return
		}

		// Build kraken for each non-u-root build target
		for t := range cfg.Targets {
			if t != "u-root" {
				log.Printf("building: %s (GOOS: %s, GOARCH; %s)", t, cfg.Targets[t].Os, cfg.Targets[t].Arch)
				if e = buildMainKraken(tmpDir, fromTemplates, t, cfg.Targets[t], *verbose); e != nil {
					log.Printf("failed to build %s: %v", t, e)
					continue
				}
			}
		}
	}
	return
}

func main() {
	var e error
	flag.Parse()

	// read config
	if cfg, e = readConfig(*cfgFile); e != nil {
		log.Fatalf("error reading config file \"%s\": %v", *cfgFile, e)
	}

	// Get kraken source tree root (and module directory)
	krakenDir, e := getModDir()
	if e != nil {
		log.Fatalf("error getting current module directory: %v", e)
	}
	log.Printf("using kraken at: %s", krakenDir)

	// Specify where temp directory should be for compiled templates
	tmpDir := filepath.Join(krakenDir, "tmp") // make an option to change where this is?

	// build
	if e = krakenBuild(cfg, krakenDir, tmpDir); e != nil {
		log.Fatalf("failed to build: %v", e)
	}

	if !*noCleanup { // cleanup now
		os.RemoveAll(tmpDir)
	} else {
		log.Printf("leaving temp directory: %s", tmpDir)
	}
}
