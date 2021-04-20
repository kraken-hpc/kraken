/* extension.go: generators for making kraken extensions
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

type ExtensionConfig struct {
	// Global should not be specified on config; it will get overwritten regardless.
	Global *GlobalConfigType `yaml:""`
	// URL of package, e.g. "github.com/kraken-hpc/kraken/extensions/ipv4" (required)
	PackageUrl string `yaml:"package_url"`
	// Go package name (default: last element of PackageUrl)
	PackageName string `yaml:"package_name"`
	// Proto package name (default: camel case of PackageName)
	ProtoPackage string `yaml:"proto_name"`
	// Extension object name (required)
	// More than one extension object can be declared in the same package
	// Objects are referenced as ProtoName.Name, e.g. IPv4.IPv4OverEthernet
	Name string `yaml:"name"`
	// CustomTypes are an advanced feature and most people won't use them
	// Declaring a custom type will create a stub to develop a gogo customtype on
	// It will also include a commented example of linking a customtype in the proto
	CustomTypes []string `yaml:"custom_types"`
	// LowerName is intended for internal use only
	LowerName string `yaml:""`
}

type CustomTypeConfig struct {
	Global *GlobalConfigType `yaml:"__global,omitempty"`
	Name   string
}

func extensionReadConfig(file string) (cfg *ExtensionConfig, err error) {
	cfgData, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read config file %s: %v", file, err)
	}
	cfg = &ExtensionConfig{}
	if err = yaml.Unmarshal(cfgData, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", file, err)
	}
	// now, apply sanity checks & defaults
	if cfg.PackageUrl == "" {
		return nil, fmt.Errorf("package_url must be specified")
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("name must be specified")
	}
	urlParts := util.URLToSlice(cfg.PackageUrl)
	name := urlParts[len(urlParts)-1]
	if cfg.PackageName == "" {
		cfg.PackageName = name
	}
	if cfg.ProtoPackage == "" {
		parts := strings.Split(cfg.PackageName, "_") // in the off chance _ is used
		cname := ""
		for _, s := range parts {
			cname += strings.Title(s)
		}
		cfg.ProtoPackage = cname
	}
	cfg.LowerName = strings.ToLower(cfg.Name)
	return
}

func extensionCompileTemplate(tplFile, outDir string, cfg *ExtensionConfig) (target string, err error) {
	var tpl *template.Template
	var out *os.File
	parts := strings.Split(filepath.Base(tplFile), ".")
	if parts[0] == "template" {
		parts = append([]string{cfg.LowerName}, parts[1:len(parts)-1]...)
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

func extensionCompileCustomtype(outDir string, cfg *CustomTypeConfig) (target string, err error) {
	var tpl *template.Template
	var out *os.File
	target = cfg.Name + ".type.go"
	dest := filepath.Join(outDir, target)
	if _, err := os.Stat(dest); err == nil {
		if !Global.Force {
			return "", fmt.Errorf("refusing to overwrite file: %s (force not specified)", dest)
		}
	}
	tpl = template.New("extension/customtype.type.go.tpl")
	data, err := templates.Asset("extension/customtype.type.go.tpl")
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

func ExtensionGenerate(global *GlobalConfigType, args []string) {
	Global = global
	Log = global.Log
	var configFile string
	var outDir string
	var help bool
	fs := flag.NewFlagSet("extension generate", flag.ExitOnError)
	fs.StringVar(&configFile, "c", "extension.yaml", "name of extension config file to use")
	fs.StringVar(&outDir, "o", ".", "output directory for extension")
	fs.BoolVar(&help, "h", false, "print this usage")
	fs.Usage = func() {
		fmt.Println("extension [gen]erate will generate a kraken extension based on an extension config.")
		fmt.Println("Usage: kraken <opts> extension generate [-h] [-c <config_file>] [-o <out_dir>]")
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
	cfg, err := extensionReadConfig(configFile)
	if err != nil {
		Log.Fatalf("failed to read config file: %v", err)
	}
	Log.Debugf("generating %s.%s with %d custom types",
		cfg.ProtoPackage,
		cfg.Name,
		len(cfg.CustomTypes))
	cfg.Global = global
	// Ok, that's all the prep, now fill/write the templates
	common := []string{
		"extension/template.proto.tpl",
		"extension/template.ext.go.tpl",
		"extension/template.go.tpl",
	}
	for _, f := range common {
		written, err := extensionCompileTemplate(f, outDir, cfg)
		if err != nil {
			Log.Fatalf("failed to write template: %v", err)
		}
		Log.Debugf("wrote file: %s", written)
	}
	if len(cfg.CustomTypes) > 0 {
		// generate customtypes
		ctypeDir := filepath.Join(outDir, "customtypes")
		stat, err := os.Stat(ctypeDir)
		if err == nil && !stat.IsDir() {
			Log.Fatalf("customtypes directory %s exists, but is not a directory", outDir)
		}
		if err != nil {
			// create the dir
			if err = os.MkdirAll(ctypeDir, 0777); err != nil {
				Log.Fatalf("failed to create customtypes directory %s: %v", outDir, err)
			}
		}
		for _, name := range cfg.CustomTypes {
			ctCfg := &CustomTypeConfig{
				Global: global,
				Name:   name,
			}
			written, err := extensionCompileCustomtype(ctypeDir, ctCfg)
			if err != nil {
				Log.Fatalf("failed to write template: %v", err)
			}
			Log.Debugf("wrote file: %s", written)
		}
	}
	Log.Infof("extension \"%s.%s\" generated at %s", cfg.ProtoPackage, cfg.Name, outDir)
}
