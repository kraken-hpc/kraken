// One-time generated by kraken {{ .Global.Version }}.
// You probably want to edit this file to add values you need.
// `kraken module update` *will not* replace this file.
// `kraken -f module generate` *will* replace this file.
// To generate your config protobuf run `go generate` (requires `protoc` and `gogo-protobuf`)

{{ if .WithConfig }}
//go:generate protoc -I $GOPATH/src -I . --gogo_out=plugins=grpc:. {{ .PackageName }}.config.proto
{{ end }}

package {{ .PackageName }}

import (
	{{- if .WithConfig }}
	"fmt"
	{{ end }}
	"os"
	{{- if .WithPolling }}
	"time"
	{{ end }}

	{{- if .WithConfig }}
	proto "github.com/gogo/protobuf/proto"
	{{ end }}
	"github.com/kraken-hpc/kraken/lib/types"
)

/////////////////////////
// {{ .Name }} Object //
///////////////////////

// These dummy declarations exist to ensure that we're adhering to all of our intended interfaces
var _ types.Module = (*{{ .Name }})(nil)
{{- if .WithConfig }}
var _ types.ModuleWithConfig = (*{{- .Name }})(nil)
{{- end }}
var _ types.ModuleWithMutations = (*{{ .Name }})(nil)
var _ types.ModuleWithDiscovery = (*{{ .Name }})(nil)
var _ types.ModuleSelfService = (*{{ .Name }})(nil)

// {{ .Name }} is the primary object interface of the module
type {{ .Name }} struct {
	{{- if .WithConfig }}
	cfg        *Config
	{{ end }}
	{{- if .WithPolling }}
	pollTicker *time.Ticker
	{{ end }}
	// TODO: You can add more internal variables here
}

///////////////////////
// Module Execution //
/////////////////////

// Init is used to intialize an executable module prior to entrypoint
func (mod *{{ .Name }}) Init(a types.ModuleAPIClient) {
	api = a
	self = api.Self()
	{{- if .WithConfig }}
	mod.cfg = mod.NewConfig().(*Config)
	{{ end }}
	// TODO: You should set these mutation handlers to real functions
	{{- range $name, $mutation := .Mutations }}
	mutations["{{- $name }}"].handler = hNotImplemented
	{{- end }}
	// TODO: You may need to initialize more things here
	Log(DEBUG, "initialized")
}

// Entry is the module's executable entrypoint
func (mod *{{ .Name }}) Entry() {
	Log(INFO, "starting")

	{{- if .WithPolling }}
	// setup a ticker for polling discovery
	dur, _ := time.ParseDuration(mod.cfg.GetPollingInterval())
	mod.pollTicker = time.NewTicker(dur)

	{{ end }}
	// TODO: You may need to start more things here.

	// set our own state to be running
	mod.State(StateRun)

	// enter our event listener
	mod.Listen()
}

// Stop should perform a graceful exit
func (mod *{{ .Name }}) Stop() {
	Log(INFO, "stopping")
	// TODO: You may need to safely stop more things.
	os.Exit(0)
}

{{- if .WithPolling }}
//////////////
// Polling //
////////////

// Poll is called repeated only a timer.
func (mod *{{ .Name }}) Poll() {
	Log(DDEBUG, "polling loop called")

	// TODO: Add some stuff for Poll to do.  Discovering this is a common option.
}

{{ end }}
{{- if .WithConfig }}
//////////////////////
// Config Handling //
////////////////////

// NewConfig returns a fully initialized default config
func (*{{ .Name }}) NewConfig() proto.Message {
	// TODO: fill out this structure with default values for your config
	r := &Config{
		{{- if .WithPolling }}
		PollingInterval: "10s",
		{{ end }}
	}
	return r
}

// UpdateConfig updates the running config
func (mod *{{ .Name }}) UpdateConfig(cfg proto.Message) (e error) {
	if cfgConfig, ok := cfg.(*Config); ok {
		{{- if .WithPolling }}
		// Update the polling ticker
		if mod.pollTicker != nil {
			mod.pollTicker.Stop()
			dur, _ := time.ParseDuration(mod.cfg.GetPollingInterval())
			mod.pollTicker = time.NewTicker(dur)
		}
		{{ end }}
		// TODO: You made need to update other things here when your config is changed.
		mod.cfg = cfgConfig
		return
	}
	return fmt.Errorf("invalid config type")
}
{{ end }}
