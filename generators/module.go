/* module.go: generators for making kraken modules
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2021, Triad National Security, LLC
 * See LICENSE file for details.
 */

package generators

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/kraken-hpc/kraken/generators/templates"
	"github.com/kraken-hpc/kraken/lib/util"
	"gopkg.in/yaml.v2"
)

type ModuleConfigMutations struct {
	Mutates map[string]struct { // Key is mutation URL. Values are string representations.
		From string
		To   string
	} `yaml:"mutates"`
	// key is Url, value is state value (string representation)
	Requires map[string]string `yaml:"requires"`
	// key is Url, value is state value (string representation)
	Excludes map[string]string `yaml:"excludes"`
	// execution context {"CHILD", "SELF", "ALL"}, default: "SELF"
	Context string `yaml:"context"`
	// execution timeout in string form, e.g. "10s", "5m". Default: 0 (no timeout)
	Timeout string `yaml:"timeout"`
	// what we discover on timeout.  Must be set of timeout != 0.
	FailTo struct {
		Url   string
		Value string
	} `yaml:"fail_to,omitempty"`
}

type ModuleConfig struct {
	// Global should not be specified on config; it will get overwritten regardless.
	Global *GlobalConfigType `yaml:"__global,omitempty"`
	// Module name, i.e. Object name (default: CamelCase of PackageName)
	Name string `yaml:"name"`
	// Go package name (default: last element of PackageUrl)
	PackageName string `yaml:"package_name"`
	// URL of package, e.g. "github.com/kraken-hpc/kraken/modules/restapi" (required)
	PackageUrl string `yaml:"package_url"`
	// Generate code for a polling loop, implies WithConfig (default: true)
	WithPolling bool `yaml:"with_polling,omitempty"`
	// Generate code/proto for a configuration (default: true)
	WithConfig bool `yaml:"with_config,omitempty"`
	// A list of URLs we discover for.
	// Our own service, and anything mentioned in Mutations are automatic.
	Discoveries []string `yaml:"discoveries"`
	// Mutation definitions.  Key is the unique mutation name.
	Mutations map[string]*ModuleConfigMutations `yaml:"mutations"`
}

func moduleReadConfig(file string) (cfg *ModuleConfig, err error) {
	cfgData, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read config file %s: %v", file, err)
	}
	cfg = &ModuleConfig{
		WithPolling: true,
		WithConfig:  true,
	}
	if err = yaml.Unmarshal(cfgData, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", file, err)
	}
	// now, apply sanity checks & defaults
	if cfg.PackageUrl == "" {
		return nil, fmt.Errorf("package_url must be specified")
	}
	urlParts := util.URLToSlice(cfg.PackageUrl)
	name := urlParts[len(urlParts)-1]
	if cfg.PackageName == "" {
		cfg.PackageName = name
	}
	if cfg.Name == "" {
		parts := strings.Split(cfg.PackageName, "_") // in the off chance _ is used
		cname := ""
		for _, s := range parts {
			cname += strings.Title(s)
		}
		cfg.Name = cname
	}
	if cfg.WithPolling {
		cfg.WithConfig = true
	}
	for _, m := range cfg.Mutations {
		if m.Context == "" {
			m.Context = "SELF"
		}
	}
	return
}

func moduleCompileTemplate(tplFile, outDir string, cfg *ModuleConfig) (target string, err error) {
	var tpl *template.Template
	var out *os.File
	parts := strings.Split(filepath.Base(tplFile), ".")
	if parts[0] == "template" {
		parts = append([]string{cfg.PackageName}, parts[1:len(parts)-1]...)
	} else {
		parts = parts[:len(parts)-1]
	}
	target = strings.Join(parts, ".")
	dest := filepath.Join(outDir, target)
	if _, err := os.Stat(dest); err == nil {
		if !Global.Force {
			return "", fmt.Errorf("refusing to overwrite file: %s (force not specified)", dest)
		}
	}
	tpl = template.New(tplFile)
	data, err := templates.Asset(tplFile)
	if err != nil {
		return
	}
	if tpl, err = tpl.Parse(string(data)); err != nil {
		return
	}
	if out, err = os.Create(dest); err != nil {
		return
	}
	defer out.Close()
	err = tpl.Execute(out, cfg)
	return
}

func ModuleGenerate(global *GlobalConfigType, args []string) {
	Global = global
	Log = global.Log
	var configFile string
	var outDir string
	var help bool
	fs := flag.NewFlagSet("module generate", flag.ExitOnError)
	fs.StringVar(&configFile, "c", "module.yaml", "name of app config file to use")
	fs.StringVar(&outDir, "o", ".", "output directory for app")
	fs.BoolVar(&help, "h", false, "print this usage")
	fs.Usage = func() {
		fmt.Println("module generate will generate a kraken module based on a module config.")
		fmt.Println("Usage: kraken <opts> module generate [-h] [-c <config_file>] [-o <out_dir>]")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if help {
		fs.Usage()
		os.Exit(0)
	}
	if len(fs.Args()) != 0 {
		Log.Fatalf("unknown option: %s", fs.Args()[0])
	}
	stat, err := os.Stat(outDir)
	if err == nil && !stat.IsDir() {
		Log.Fatalf("output directory %s exists, but is not a directory", outDir)
	}
	if err != nil {
		// create the dir
		if err = os.MkdirAll(outDir, 0777); err != nil {
			Log.Fatalf("failed to create output directory %s: %v", outDir, err)
		}
	}
	cfg, err := moduleReadConfig(configFile)
	if err != nil {
		Log.Fatalf("failed to read config file: %v", err)
	}
	Log.Debugf("generating %s with %d discoveries and %d mutations, with_config=%t, with_polling=%t",
		cfg.PackageName,
		len(cfg.Discoveries),
		len(cfg.Mutations),
		cfg.WithConfig,
		cfg.WithPolling)
	cfg.Global = global
	// Ok, that's all the prep, now fill/write the templates
	common := []string{
		"module/template.mod.go.tpl",
		"module/template.go.tpl",
		"module/README.md.tpl",
	}
	for _, f := range common {
		written, err := moduleCompileTemplate(f, outDir, cfg)
		if err != nil {
			Log.Fatalf("failed to write template: %v", err)
		}
		Log.Debugf("wrote file: %s", written)
	}
	if cfg.WithConfig {
		written, err := moduleCompileTemplate("module/template.config.proto.tpl", outDir, cfg)
		if err != nil {
			Log.Fatalf("failed to write template: %v", err)
		}
		Log.Debugf("write file: %s", written)
	}
	Log.Infof("module \"%s\" generated at %s", cfg.PackageName, outDir)
}

func ModuleUpdate(global *GlobalConfigType, args []string) {
	Global = global
	Log = global.Log
	var configFile string
	var outDir string
	var help bool

	fs := flag.NewFlagSet("module update", flag.ExitOnError)
	fs.StringVar(&configFile, "c", "module.yaml", "name of app config file to use")
	fs.StringVar(&outDir, "o", ".", "output directory for app")
	fs.BoolVar(&help, "h", false, "print this usage")
	fs.Usage = func() {
		fmt.Println("module update will update the <module>.mod.go file, but leave other module files alone")
		fmt.Println("Usage: kraken <opts> module update [-h] [-c <config_file>] [-o <out_dir>]")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if help {
		fs.Usage()
		os.Exit(0)
	}
	if len(fs.Args()) != 0 {
		Log.Fatalf("unknown option: %s", fs.Args()[0])
	}
	cfg, err := moduleReadConfig(configFile)
	if err != nil {
		Log.Fatalf("failed to read config file: %v", err)
	}
	if _, err = os.Stat(filepath.Join(outDir, cfg.PackageName+".mod.go")); err != nil {
		Log.Fatalf("could not find an existing generated module file: %v", err)
	}
	Log.Debugf("updating %s with %d discoveries and %d mutations, with_config=%t, with_polling=%t",
		cfg.PackageName,
		len(cfg.Discoveries),
		len(cfg.Mutations),
		cfg.WithConfig,
		cfg.WithPolling)
	cfg.Global = global
	// Ok, that's all the prep, now fill/write the templates
	common := []string{
		"module/template.mod.go.tpl",
	}
	Global.Force = true
	for _, f := range common {
		written, err := moduleCompileTemplate(f, outDir, cfg)
		if err != nil {
			Log.Fatalf("failed to write template: %v", err)
		}
		Log.Debugf("wrote file: %s", written)
	}
	Log.Infof("updated \"%s\" at %s", cfg.PackageName, outDir)
}
