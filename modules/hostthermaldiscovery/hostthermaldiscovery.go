/* HostThermalDiscovery.go: This module performs CPU Thermal discovery of HPC nodes through in-band (through OS) mechanism.
 *
 * Author: Ghazanfar Ali, ghazanfar.ali@ttu.edu; Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package hostthermaldiscovery

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	scalpb "github.com/hpc/kraken/extensions/HostFrequencyScaler/proto"
	thpb "github.com/hpc/kraken/extensions/HostThermal/proto"

	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/hostthermaldiscovery/proto"
)

//CPUTempObj is strututure for node CPU temperature
type CPUTempObj struct {
	TimeStamp   time.Time
	HostAddress string
	CPUTemp     int32
}

const (
	// HostThermalStateURL points to Thermal extension
	HostThermalStateURL = "type.googleapis.com/proto.HostThermal/State"
	// ModuleStateURL refers to module state
	ModuleStateURL = "/Services/hostthermaldiscovery/State"
	// hostFreqScalerURL provides URL for host frequency scaler at host run time
	hostFreqScalerURL string = "type.googleapis.com/proto.HostFrequencyScaler/State"

	// freqSensorPath holds frequency sensor path on pi node
	freqSensorPath string = "/sys/devices/system/cpu/cpufreq/policy0/"
)

var _ lib.Module = (*HostDisc)(nil)
var _ lib.ModuleWithConfig = (*HostDisc)(nil)
var _ lib.ModuleWithDiscovery = (*HostDisc)(nil)
var _ lib.ModuleSelfService = (*HostDisc)(nil)

var profileMap = map[string]string{
	"performance": scalpb.HostFrequencyScaler_PERFORMANCE.String(),
	"powersave":   scalpb.HostFrequencyScaler_POWER_SAVE.String(),
}

// HostDisc provides hostdiscovery module capabilities
type HostDisc struct {
	prevTemp      int32
	preFreqScaler string
	api           lib.APIClient
	cfg           *pb.HostDiscoveryConfig
	dchan         chan<- lib.Event
	pollTicker    *time.Ticker
}

// Name returns the FQDN of the module
func (*HostDisc) Name() string { return "github.com/hpc/kraken/modules/hostthermaldiscovery" }

// NewConfig returns a fully initialized default config
func (*HostDisc) NewConfig() proto.Message {
	r := &pb.HostDiscoveryConfig{
		PollingInterval: "10s",
		TempSensorPath:  "/sys/devices/virtual/thermal/thermal_zone0/temp",
		FreqSensorUrl:   freqSensorPath,
		ThermalThresholds: map[string]*pb.HostThermalThresholds{
			"CPUThermalThresholds": {
				LowerNormal:   3000,
				UpperNormal:   60000,
				LowerHigh:     60000,
				UpperHigh:     70000,
				LowerCritical: 3000,
				UpperCritical: 70000,
			},
		},
	}
	return r
}

// UpdateConfig updates the running config
func (hostDisc *HostDisc) UpdateConfig(cfg proto.Message) (e error) {
	if rcfg, ok := cfg.(*pb.HostDiscoveryConfig); ok {
		hostDisc.cfg = rcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*HostDisc) ConfigURL() string {
	cfg := &pb.HostDiscoveryConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (hostDisc *HostDisc) SetDiscoveryChan(c chan<- lib.Event) { hostDisc.dchan = c }

func init() {
	module := &HostDisc{}
	discovers := make(map[string]map[string]reflect.Value)
	hostThermDisc := make(map[string]reflect.Value)
	hostFreqScalerDiscs := make(map[string]reflect.Value)

	hostThermDisc[thpb.HostThermal_CPU_TEMP_NONE.String()] = reflect.ValueOf(thpb.HostThermal_CPU_TEMP_NONE)
	hostThermDisc[thpb.HostThermal_CPU_TEMP_NORMAL.String()] = reflect.ValueOf(thpb.HostThermal_CPU_TEMP_NORMAL)
	hostThermDisc[thpb.HostThermal_CPU_TEMP_HIGH.String()] = reflect.ValueOf(thpb.HostThermal_CPU_TEMP_HIGH)
	hostThermDisc[thpb.HostThermal_CPU_TEMP_CRITICAL.String()] = reflect.ValueOf(thpb.HostThermal_CPU_TEMP_CRITICAL)

	discovers[HostThermalStateURL] = hostThermDisc

	// hostFreqScalerDiscs[scalpb.HostFrequencyScaler_NONE.String()] = reflect.ValueOf(scalpb.HostFrequencyScaler_NONE)
	// hostFreqScalerDiscs[scalpb.HostFrequencyScaler_PERFORMANCE.String()] = reflect.ValueOf(scalpb.HostFrequencyScaler_PERFORMANCE)
	// hostFreqScalerDiscs[scalpb.HostFrequencyScaler_POWER_SAVE.String()] = reflect.ValueOf(scalpb.HostFrequencyScaler_POWER_SAVE)
	// discovers[hostFreqScalerURL] = hostFreqScalerDiscs

	discovers[ModuleStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	si := core.NewServiceInstance("hostthermaldiscovery", module.Name(), module.Entry, nil)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
}

// Init is used to intialize an executable module prior to entrypoint
func (hostDisc *HostDisc) Init(api lib.APIClient) {
	hostDisc.api = api
	hostDisc.cfg = hostDisc.NewConfig().(*pb.HostDiscoveryConfig)
}

// Stop should perform a graceful exit
func (hostDisc *HostDisc) Stop() {
	os.Exit(0)
}

// Entry is the module's executable entrypoint
func (hostDisc *HostDisc) Entry() {
	url := lib.NodeURLJoin(hostDisc.api.Self().String(), ModuleStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{

			URL:     url,
			ValueID: "RUN",
		},
	)
	hostDisc.dchan <- ev

	// setup a ticker for polling discovery
	dur, _ := time.ParseDuration(hostDisc.cfg.GetPollingInterval())
	hostDisc.pollTicker = time.NewTicker(dur)

	// main loop
	for {
		select {
		case <-hostDisc.pollTicker.C:
			go hostDisc.discoverHostCPUTemp()
			go hostDisc.DiscFreqScaler()
			break
		}
	}
}

// DiscFreqScaler cpu frequency scaler
func (hostDisc *HostDisc) DiscFreqScaler() {

	hostFreqScaler := hostDisc.ReadFreqScaler()
	if hostFreqScaler == hostDisc.preFreqScaler {
		// no change in frequency scaler so no need to generate discovery event
		return
	}
	hostDisc.preFreqScaler = hostFreqScaler

	vid := profileMap[hostFreqScaler]

	hostDisc.api.Logf(lib.LLERROR, "PRINTSCALER: %s", vid)

	url := lib.NodeURLJoin(hostDisc.api.Self().String(), hostFreqScalerURL)

	// Generating discovery event for CPU Thermal state
	v := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{

			URL:     url,
			ValueID: vid,
		},
	)
	hostDisc.dchan <- v

}

// ReadFreqScaler cpu frequency scaler
func (hostDisc *HostDisc) ReadFreqScaler() string {

	basePath := hostDisc.cfg.GetFreqSensorUrl()
	bscalingGovernor, err := ioutil.ReadFile(basePath + "scaling_governor")
	if err != nil {
		hostDisc.api.Logf(lib.LLERROR, "Reading CPU thermal sensor failed: %v", err)
		return ""
	}
	//fscalingGovernor := strings.TrimSuffix(string(bscalingGovernor), "\n")
	return strings.TrimSuffix(string(bscalingGovernor), "\n")
}

// discoverHostCPUTemp is used to acquire CPU thermal locally.
func (hostDisc *HostDisc) discoverHostCPUTemp() {
	hostCPUTemp := hostDisc.GetCPUTemp()
	if hostCPUTemp.CPUTemp == hostDisc.prevTemp {
		// no change in temperature so no need to generate discovery event
		return
	}
	hostDisc.prevTemp = hostCPUTemp.CPUTemp

	vid, _ := hostDisc.lambdaStateDiscovery(hostCPUTemp)
	url := lib.NodeURLJoin(hostDisc.api.Self().String(), HostThermalStateURL)

	// Generating discovery event for CPU Thermal state
	v := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{

			URL:     url,
			ValueID: vid,
		},
	)
	hostDisc.dchan <- v

}

// GetCPUTemp returns CPU temperature
func (hostDisc *HostDisc) GetCPUTemp() CPUTempObj {

	hostIP := hostDisc.GetNodeIPAddress()

	// Its a mockup CPU temperature
	cpuTempObj := CPUTempObj{}
	cpuTempObj.TimeStamp = time.Now()
	cpuTempObj.HostAddress = hostIP

	tempVal := hostDisc.ReadCPUTemp()
	cpuTempObj.CPUTemp = tempVal

	return cpuTempObj
}

// ReadCPUTemp function reads the CPU thermal sensor
func (hostDisc *HostDisc) ReadCPUTemp() int32 {

	tempSensorPath := hostDisc.cfg.GetTempSensorPath()

	cpuTemp, err := ioutil.ReadFile(tempSensorPath)
	if err != nil {
		hostDisc.api.Logf(lib.LLERROR, "Reading CPU thermal sensor failed: %v", err)
		return 0
	}
	cpuTempInt, err := strconv.Atoi(strings.TrimSuffix(string(cpuTemp), "\n"))

	if err != nil {
		hostDisc.api.Logf(lib.LLERROR, "String to Int conversion failed: %v", err)
		return 0
	}

	return int32(cpuTempInt)
}

// GetNodeIPAddress returns non local loop IP address
func (hostDisc *HostDisc) GetNodeIPAddress() string {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		hostDisc.api.Logf(lib.LLERROR, "Can not find network interfaces: %v", err)
	}
	ip := ""
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				break
			} else {
				hostDisc.api.Logf(lib.LLERROR, "Can not format IP address")
			}
		}
	}

	return ip
}

// Discovers state of the CPU based on CPU temperature thresholds
func (hostDisc *HostDisc) lambdaStateDiscovery(v CPUTempObj) (string, int32) {
	cpuTemp := v.CPUTemp
	cpuTempState := thpb.HostThermal_CPU_TEMP_NONE

	cpuThermalThresholds := hostDisc.cfg.GetThermalThresholds()
	lowerNormal := cpuThermalThresholds["CPUThermalThresholds"].GetLowerNormal()
	upperNormal := cpuThermalThresholds["CPUThermalThresholds"].GetUpperNormal()

	lowerHigh := cpuThermalThresholds["CPUThermalThresholds"].GetLowerHigh()
	upperHigh := cpuThermalThresholds["CPUThermalThresholds"].GetUpperHigh()

	lowerCritical := cpuThermalThresholds["CPUThermalThresholds"].GetLowerCritical()
	upperCritical := cpuThermalThresholds["CPUThermalThresholds"].GetUpperCritical()

	if cpuTemp <= lowerCritical || cpuTemp >= upperCritical {
		cpuTempState = thpb.HostThermal_CPU_TEMP_CRITICAL
	} else if cpuTemp >= lowerHigh && cpuTemp < upperHigh {
		cpuTempState = thpb.HostThermal_CPU_TEMP_HIGH
	} else if cpuTemp > lowerNormal && cpuTemp < upperNormal {
		cpuTempState = thpb.HostThermal_CPU_TEMP_NORMAL
	}
	return cpuTempState.String(), cpuTemp

}
