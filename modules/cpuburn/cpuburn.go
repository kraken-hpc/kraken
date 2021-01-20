/* cpuburn.go: this module induces artificial load on the system.  It can throttle itself based on thermal condition.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. cpuburn.proto

package cpuburn

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"

	cpb "github.com/hpc/kraken/core/proto"
)

// SrvStateURL is the URL For this service instance
const SrvStateURL = "/Services/cpuburn/State"

var defaultKernel = (Kernel)(kernelLucasLehmer)

var _ types.Module = (*CPUBurn)(nil)
var _ types.ModuleSelfService = (*CPUBurn)(nil)
var _ types.ModuleWithConfig = (*CPUBurn)(nil)
var _ types.ModuleWithDiscovery = (*CPUBurn)(nil)

// A Kernel is a function that can be executed as stress kernels
type Kernel func()

const (
	CONTROL_STOP = iota
	CONTROL_START
	CONTROL_THROTTLE
	CONTROL_UNTHROTTLE
)

// A CPUBurn manages burners on a cpu
type CPUBurn struct {
	api       types.ModuleAPIClient
	cfg       *Config
	dchan     chan<- types.Event
	control   chan int
	workers   []chan int // our workers are defined by their quit channels
	running   bool
	throttled bool
	therm     chan int
}

// Name returns a unique URL name for the module
// Module
func (*CPUBurn) Name() string { return "github.com/hpc/kraken/modules/cpuburn" }

// Entry provides the module entry point
// ModuleSelfService
func (c *CPUBurn) Entry() {
	url := util.NodeURLJoin(c.api.Self().String(), SrvStateURL)
	ev := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)
	c.dchan <- ev
	go func() {
		c.control <- CONTROL_START
	}()
	for {
		switch sig := <-c.control; sig {
		case CONTROL_STOP:
			c.api.Log(types.LLINFO, "got STOP control")
			c.ctlStop()
		case CONTROL_START:
			c.api.Log(types.LLINFO, "got START control")
			c.ctlStart()
		case CONTROL_THROTTLE:
			c.api.Log(types.LLINFO, "got THROTTLE control")
			c.ctlThrottle()
		case CONTROL_UNTHROTTLE:
			c.api.Log(types.LLINFO, "got UNTHROTTLE control")
			c.ctlThrottle()
		}
	}
}

// Init is run before Entry, provides an ModuleAPIClient
// ModuleSelfService
func (c *CPUBurn) Init(api types.ModuleAPIClient) {
	c.api = api
	c.cfg = c.NewConfig().(*Config)
	c.control = make(chan int)
	c.running = false
}

// Stop should gracefully exit a running service instance
// ModuleSelfService
func (*CPUBurn) Stop() {
	os.Exit(0)
}

// NewConfig should return a guaranteed sane (default) config
// ModuleWithConfig
func (*CPUBurn) NewConfig() (r proto.Message) {
	r = &Config{
		TempSensor:       "/sys/class/thermal/thermal_zone0/temp",
		ThermalThrottle:  true,
		ThermalPoll:      1,
		ThermalResume:    60000,
		ThermalCrit:      98000,
		Workers:          4,
		WorkersThrottled: 0,
	}
	return
}

// UpdateConfig should update the running module config
// ModuleWithConfig
func (c *CPUBurn) UpdateConfig(cfg proto.Message) (e error) {
	if ccfg, ok := cfg.(*Config); ok {
		c.cfg = ccfg
		return
	}
	c.control <- CONTROL_STOP
	c.control <- CONTROL_START
	return fmt.Errorf("invalid config type")
}

// ConfigURL should return the unique url for the Config
// ModuleWithConfig
func (*CPUBurn) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetDiscoveryChan takes a discovery channel (and presumably stores it)
// ModuleWithDiscovery
func (c *CPUBurn) SetDiscoveryChan(dc chan<- types.Event) { c.dchan = dc }

/*
 * Module specific functions
 */

func (c *CPUBurn) runTherm(quit <-chan int) {
	c.api.Log(types.LLDEBUG, "thermal watcher is starting")
	cooling := false
	for {
		select {
		case <-quit:
			c.api.Log(types.LLDEBUG, "thermal watcher is exiting")
			return
		default:
		}
		tstr, err := ioutil.ReadFile(c.cfg.TempSensor)
		if err != nil {
			c.api.Logf(types.LLERROR, "failed to read temp sensor: %v", err)
			time.Sleep(time.Duration(c.cfg.ThermalPoll) * time.Second)
			continue
		}
		t, err := strconv.Atoi(strings.TrimSuffix(string(tstr), "\n"))
		if err != nil {
			c.api.Logf(types.LLERROR, "failed to interpret temp sensor: %v", err)
			time.Sleep(time.Duration(c.cfg.ThermalPoll) * time.Second)
			continue
		}
		if t >= int(c.cfg.ThermalCrit) && !cooling {
			c.api.Logf(types.LLDEBUG, "reached thermal critical temp: %d", t)
			cooling = true
			c.control <- CONTROL_THROTTLE
		} else if t <= int(c.cfg.ThermalResume) && cooling {
			c.api.Logf(types.LLDEBUG, "reached thermal resume temp: %d", t)
			cooling = false
			c.control <- CONTROL_UNTHROTTLE
		}
		time.Sleep(time.Duration(c.cfg.ThermalPoll) * time.Second)
	}
}

func (c *CPUBurn) runWorker(kernel func()) {
	quit := make(chan int)
	c.workers = append(c.workers, quit)
	go c.worker(quit, kernel)
}

func (c *CPUBurn) stopWorker() {
	var quit chan<- int
	if len(c.workers) > 0 {
		quit, c.workers = c.workers[0], c.workers[1:]
		go func() {
			quit <- 0
		}()
	}
}

func (c *CPUBurn) ctlStop() {
	if !c.running {
		c.api.Logf(types.LLERROR, "got STOP, but we're not running")
		return
	}
	if c.therm != nil {
		c.therm <- 0
		c.therm = nil
	}
	c.api.Logf(types.LLDEBUG, "stopping %d workers", len(c.workers))
	for i := 0; i < len(c.workers); i++ {
		c.stopWorker()
	}
	c.running = false
}

func (c *CPUBurn) ctlStart() {
	if c.running {
		c.api.Logf(types.LLERROR, "got START, but already running")
		return
	}
	if c.cfg.ThermalThrottle {
		c.therm = make(chan int)
		go c.runTherm(c.therm)
	}
	c.api.Logf(types.LLDEBUG, "starting %d workers", int(c.cfg.Workers))
	for i := 0; i < int(c.cfg.Workers); i++ {
		c.runWorker(defaultKernel)
	}
	c.running = true
}

func (c *CPUBurn) ctlThrottle() {
	if c.throttled {
		return
	}
	if c.cfg.WorkersThrottled < c.cfg.Workers {
		c.api.Logf(types.LLDEBUG, "stopping %d workers", c.cfg.Workers-c.cfg.WorkersThrottled)
		for i := c.cfg.Workers - c.cfg.WorkersThrottled; i > 0; i-- {
			c.stopWorker()
		}
	}
}

func (c *CPUBurn) ctlUnthrottle() {
	if !c.throttled {
		return
	}
	if c.cfg.WorkersThrottled < c.cfg.Workers {
		c.api.Logf(types.LLDEBUG, "(re)starting %d workers", c.cfg.Workers-c.cfg.WorkersThrottled)
		for i := c.cfg.Workers - c.cfg.WorkersThrottled; i > 0; i-- {
			c.runWorker(defaultKernel)
		}
	}
}

func (c *CPUBurn) worker(quit <-chan int, k Kernel) {
	c.api.Log(types.LLDDEBUG, "worker started")
	for {
		select {
		case <-quit:
			c.api.Log(types.LLDDEBUG, "worker quit")
			return
		default:
		}
		k()
	}
}

func kernelLucasLehmer() {
	p := rand.Int()
	s := 4
	M := int(math.Pow(2, float64(p)) - 1)
	for i := 0; i < p-2; i++ {
		s = ((s * s) - 2) % M
	}
	// if s == 0 prime, else composite, but we don't actually return
}

func init() {
	module := &CPUBurn{}
	si := core.NewServiceInstance("cpuburn", module.Name(), module.Entry)
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, map[string]map[string]reflect.Value{
		SrvStateURL: {
			"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN),
		},
	})
}
