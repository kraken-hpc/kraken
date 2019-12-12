/* cpuburn.go: this module induces artificial load on the system.  It can throttle itself based on thermal condition.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/cpuburn.proto

package cpuburn

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib"

	pb "github.com/hpc/kraken/modules/cpuburn/proto"
)

// SrvStateURL is the URL For this service instance
const SrvStateURL = "/Services/cpuburn/State"

var defaultKernel = (Kernel)(kernelLucasLehmer)

var _ lib.Module = (*CPUBurn)(nil)
var _ lib.ModuleSelfService = (*CPUBurn)(nil)
var _ lib.ModuleWithConfig = (*CPUBurn)(nil)
var _ lib.ModuleWithDiscovery = (*CPUBurn)(nil)

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
	api       lib.APIClient
	cfg       *pb.CPUBurnConfig
	dchan     chan<- lib.Event
	control   chan int
	workers   []chan int // our workers are defined by their quit channels
	running   bool
	throttled bool
}

// Name returns a unique URL name for the module
// Module
func (*CPUBurn) Name() string { return "github.com/hpc/kraken/modules/cpuburn" }

// Entry provides the module entry point
// ModuleSelfService
func (c *CPUBurn) Entry() {
	url := lib.NodeURLJoin(c.api.Self().String(), SrvStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)
	c.dchan <- ev
	var therm chan int
	for {
		switch sig := <-c.control; sig {
		case CONTROL_STOP:
			if !c.running {
				continue
			}
			if therm != nil {
				therm <- 0
				therm = nil
			}
			for i := 0; i < len(c.workers); i++ {
				c.stopWorker()
			}
			c.running = false
		case CONTROL_START:
			if c.running {
				continue
			}
			if c.cfg.ThermalThrottle {
				therm = make(chan int)
				c.runTherm(therm)
			}
			for i := 0; i < int(c.cfg.Workers); i++ {
				c.runWorker(defaultKernel)
			}
			c.running = true
		case CONTROL_THROTTLE:
			if c.throttled {
				continue
			}
			if c.cfg.WorkersThrottled < c.cfg.Workers {
				for i := c.cfg.Workers - c.cfg.WorkersThrottled; i > 0; i-- {
					c.stopWorker()
				}
			}
		case CONTROL_UNTHROTTLE:
			if !c.throttled {
				continue
			}
			if c.cfg.WorkersThrottled < c.cfg.Workers {
				for i := c.cfg.Workers - c.cfg.WorkersThrottled; i > 0; i-- {
					c.runWorker(defaultKernel)
				}
			}
		}
	}
}

// Init is run before Entry, provides an APIClient
// ModuleSelfService
func (c *CPUBurn) Init(api lib.APIClient) {
	c.api = api
	c.cfg = c.NewConfig().(*pb.CPUBurnConfig)
	c.control = make(chan int)
}

// Stop should gracefully exit a running service instance
// ModuleSelfService
func (*CPUBurn) Stop() {
	os.Exit(0)
}

// NewConfig should return a guaranteed sane (default) config
// ModuleWithConfig
func (*CPUBurn) NewConfig() (r proto.Message) {
	r = &pb.CPUBurnConfig{
		TempSensor:       "/sys/class/thermal/thermal_zone0/temp",
		ThermalThrottle:  true,
		ThermalPoll:      1,
		ThermalResume:    60000,
		ThermalCrit:      85000,
		Workers:          4,
		WorkersThrottled: 0,
	}
	return
}

// UpdateConfig should update the running module config
// ModuleWithConfig
func (c *CPUBurn) UpdateConfig(cfg proto.Message) (e error) {
	if ccfg, ok := cfg.(*pb.CPUBurnConfig); ok {
		c.cfg = ccfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL should return the unique url for the Config
// ModuleWithConfig
func (*CPUBurn) ConfigURL() string {
	cfg := &pb.CPUBurnConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetDiscoveryChan takes a discovery channel (and presumably stores it)
// ModuleWithDiscovery
func (c *CPUBurn) SetDiscoveryChan(dc chan<- lib.Event) { c.dchan = dc }

/*
 * Module specific functions
 */

func (c *CPUBurn) runTherm(quit <-chan int) {
	cooling := false
	for {
		tstr, err := ioutil.ReadFile(c.cfg.TempSensor)
		if err != nil {
			// couldn't read temp
			time.Sleep(time.Duration(c.cfg.ThermalPoll) * time.Second)
			continue
		}
		t, err := strconv.Atoi(strings.TrimSuffix(string(tstr), "\n"))
		if err != nil {
			// couldn't interpret thermal file
			time.Sleep(time.Duration(c.cfg.ThermalPoll) * time.Second)
			continue
		}
		if t >= int(c.cfg.ThermalCrit) && !cooling {
			// start throttling
			cooling = true
			c.control <- CONTROL_THROTTLE
		} else if t <= int(c.cfg.ThermalResume) && cooling {
			// resume
			cooling = false
			c.control <- CONTROL_UNTHROTTLE
		}
		time.Sleep(time.Duration(c.cfg.ThermalPoll) * time.Second)
	}
}

func (c *CPUBurn) runWorker(kernel func()) {
	quit := make(chan int)
	c.workers = append(c.workers, quit)
	go worker(quit, kernel)
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

func worker(quit <-chan int, k Kernel) {
	for {
		select {
		case <-quit:
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
