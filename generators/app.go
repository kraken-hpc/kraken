/* app.go: generators for making kraken entry points
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
	"gopkg.in/yaml.v2"
)

// App generation
type AppConfig struct {
	Global     *GlobalConfigType
	Name       string   // Application name
	Version    string   // Application version (not Kraken version)
	Pprof      bool     // Build with pprof (default: false)?
	Extensions []string // List of extensions to include (url paths)
	Modules    []string // List of modules to include (url paths)
}

func appCompileTemplate(tplFile, tmpDir string, cfg *AppConfig) (target string, e error) {
	var tpl *template.Template
	var out *os.File
	parts := strings.Split(filepath.Base(tplFile), ".")
	target = strings.Join(parts[:len(parts)-1], ".")
	dest := filepath.Join(tmpDir, target)
	if _, err := os.Stat(dest); err == nil {
		if !Global.Force {
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

func AppGenerate(global *GlobalConfigType, args []string) {
	Global = global
	Log = global.Log
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
		Log.Fatalf("unknown option: %s", fs.Args()[0])
	}

	ts, err := templates.AssetDir("app")
	if err != nil || len(ts) < 2 {
		Log.Fatalf("no assets found for app generation: %v", err)
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
	cfgData, err := ioutil.ReadFile(configFile)
	if err != nil {
		Log.Fatalf("could not read config file %s: %v", configFile, err)
	}
	cfg := &AppConfig{}
	if err = yaml.Unmarshal(cfgData, cfg); err != nil {
		Log.Fatalf("failed to parse config file %s: %v", configFile, err)
	}
	cfg.Global = Global
	if cfg.Name == "" {
		Log.Fatal("an application name must be specified in the app config")
	}
	if cfg.Version == "" {
		Log.Info("app version was not specified, will default to v0.0.0")
	}
	for _, t := range ts {
		written, err := appCompileTemplate("app/"+t, outDir, cfg)
		if err != nil {
			Log.Fatalf("failed to write template: %v", err)
		}
		Log.Debugf("wrote file: %s", written)
	}
	Log.Infof("app \"%s\" generated at %s", cfg.Name, outDir)
}
