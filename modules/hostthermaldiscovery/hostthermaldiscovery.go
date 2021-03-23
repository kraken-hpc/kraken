/* HostThermalDiscovery.go: This module performs CPU Thermal discovery of HPC nodes through in-band (through OS) mechanism.
 *
 * Author: Ghazanfar Ali, ghazanfar.ali@ttu.edu; Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. hostthermaldiscovery.proto

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

	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/kraken-hpc/kraken/core"
	cpb "github.com/kraken-hpc/kraken/core/proto"
	scalpb "github.com/kraken-hpc/kraken/extensions/hostfrequencyscaler"
	thpb "github.com/kraken-hpc/kraken/extensions/hostthermal"
	"github.com/kraken-hpc/kraken/lib/types"
	"github.com/kraken-hpc/kraken/lib/util"
)

//CPUTempObj is strututure for node CPU temperature
type CPUTempObj struct {
	TimeStamp   time.Time
	HostAddress string
	CPUTemp     int32
}

const (
	// HostThermalStateURL points to Thermal extension
	HostThermalStateURL = "type.googleapis.com/HostThermal.Temp/State"
	// ModuleStateURL refers to module state
	ModuleStateURL = "/Services/hostthermaldiscovery/State"
	// hostFreqScalerURL provides URL for host frequency scaler at host run time
	hostFreqScalerURL string = "type.googleapis.com/HostFrequencyScaler.Scaler/State"

	// freqSensorPath holds frequency sensor path on pi node
	freqSensorPath string = "/sys/devices/system/cpu/cpufreq/policy0/"

	// thermalSensorUrl holds thermal sensor path on pi node
	thermalSensorUrl string = "/sys/devices/virtual/thermal/thermal_zone0/temp"
)

var _ types.Module = (*HostDisc)(nil)
var _ types.ModuleWithConfig = (*HostDisc)(nil)
var _ types.ModuleWithDiscovery = (*HostDisc)(nil)
var _ types.ModuleSelfService = (*HostDisc)(nil)

var profileMap = map[string]string{
	"performance": scalpb.Scaler_PERFORMANCE.String(),
	"powersave":   scalpb.Scaler_POWER_SAVE.String(),
	"schedutil":   scalpb.Scaler_SCHEDUTIL.String(),
}

// HostDisc provides hostdiscovery module capabilities
type HostDisc struct {
	prevTemp      int32
	file          *os.File
	preFreqScaler string
	api           types.ModuleAPIClient
	cfg           *Config
	dchan         chan<- types.Event
	pollTicker    *time.Ticker
}

// Name returns the FQDN of the module
func (*HostDisc) Name() string { return "github.com/kraken-hpc/kraken/modules/hostthermaldiscovery" }

// NewConfig returns a fully initialized default config
func (*HostDisc) NewConfig() proto.Message {
	r := &Config{
		PollingInterval: "1s",
		TempSensorPath:  thermalSensorUrl,
		FreqSensorUrl:   freqSensorPath,
		LogThermalData:  true,
		LogHere:         "/tmp/ThermalLog.txt",
		LowerNormal:     3000,
		UpperNormal:     80000,
		LowerHigh:       80000,
		UpperHigh:       98000,
		LowerCritical:   3000,
		UpperCritical:   98000,
	}
	return r
}

// UpdateConfig updates the running config
func (hostDisc *HostDisc) UpdateConfig(cfg proto.Message) (e error) {
	if rcfg, ok := cfg.(*Config); ok {
		hostDisc.cfg = rcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*HostDisc) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (hostDisc *HostDisc) SetDiscoveryChan(c chan<- types.Event) { hostDisc.dchan = c }

func init() {
	module := &HostDisc{}
	discovers := make(map[string]map[string]reflect.Value)
	hostThermDisc := make(map[string]reflect.Value)
	hostFreqScalerDiscs := make(map[string]reflect.Value)

	hostThermDisc[thpb.Temp_CPU_TEMP_NONE.String()] = reflect.ValueOf(thpb.Temp_CPU_TEMP_NONE)
	hostThermDisc[thpb.Temp_CPU_TEMP_NORMAL.String()] = reflect.ValueOf(thpb.Temp_CPU_TEMP_NORMAL)
	hostThermDisc[thpb.Temp_CPU_TEMP_HIGH.String()] = reflect.ValueOf(thpb.Temp_CPU_TEMP_HIGH)
	hostThermDisc[thpb.Temp_CPU_TEMP_CRITICAL.String()] = reflect.ValueOf(thpb.Temp_CPU_TEMP_CRITICAL)

	discovers[HostThermalStateURL] = hostThermDisc

	hostFreqScalerDiscs[scalpb.Scaler_NONE.String()] = reflect.ValueOf(scalpb.Scaler_NONE)
	hostFreqScalerDiscs[scalpb.Scaler_PERFORMANCE.String()] = reflect.ValueOf(scalpb.Scaler_PERFORMANCE)
	hostFreqScalerDiscs[scalpb.Scaler_POWER_SAVE.String()] = reflect.ValueOf(scalpb.Scaler_POWER_SAVE)
	discovers[hostFreqScalerURL] = hostFreqScalerDiscs

	discovers[ModuleStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	si := core.NewServiceInstance("hostthermaldiscovery", module.Name(), module.Entry)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
}

// Init is used to intialize an executable module prior to entrypoint
func (hostDisc *HostDisc) Init(api types.ModuleAPIClient) {
	hostDisc.api = api
	hostDisc.cfg = hostDisc.NewConfig().(*Config)

	var err error
	hostDisc.file, err = os.OpenFile(hostDisc.cfg.GetLogHere(), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		hostDisc.api.Logf(types.LLERROR, "failed opening file: %v", err)
	}

}

// Stop should perform a graceful exit
func (hostDisc *HostDisc) Stop() {
	os.Exit(0)
}

// Entry is the module's executable entrypoint
func (hostDisc *HostDisc) Entry() {
	url := util.NodeURLJoin(hostDisc.api.Self().String(), ModuleStateURL)
	ev := core.NewEvent(
		types.Event_DISCOVERY,
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
			if hostDisc.cfg.GetLogThermalData() == true {
				go hostDisc.CapturingStatData()
			}

			break
		}
	}
}

// CapturingStatData logs thermal information
func (hostDisc *HostDisc) CapturingStatData() {
	freqScaler := hostDisc.preFreqScaler
	temp := hostDisc.prevTemp
	t := time.Now() // / int64(time.Millisecond)
	record := fmt.Sprintf("%s,%d,%s\n", t.String(), temp, freqScaler)

	_, err := hostDisc.file.WriteString(record)
	if err != nil {
		hostDisc.api.Logf(types.LLERROR, "failed opening file: %v", err)
	}

}

// DiscFreqScaler cpu frequency scaler
func (hostDisc *HostDisc) DiscFreqScaler() {

	hostFreqScaler := hostDisc.ReadFreqScaler()

	hostDisc.preFreqScaler = hostFreqScaler

	vid := profileMap[hostFreqScaler]

	url := util.NodeURLJoin(hostDisc.api.Self().String(), hostFreqScalerURL)

	// Generating discovery event for CPU Thermal state
	v := core.NewEvent(
		types.Event_DISCOVERY,
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
		hostDisc.api.Logf(types.LLERROR, "Reading CPU thermal sensor failed: %v", err)
		return ""
	}

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
	url := util.NodeURLJoin(hostDisc.api.Self().String(), HostThermalStateURL)

	// Generating discovery event for CPU Thermal state
	v := core.NewEvent(
		types.Event_DISCOVERY,
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
		hostDisc.api.Logf(types.LLERROR, "Reading CPU thermal sensor failed: %v", err)
		return 0
	}
	cpuTempInt, err := strconv.Atoi(strings.TrimSuffix(string(cpuTemp), "\n"))

	if err != nil {
		hostDisc.api.Logf(types.LLERROR, "String to Int conversion failed: %v", err)
		return 0
	}

	return int32(cpuTempInt)
}

// GetNodeIPAddress returns non local loop IP address
func (hostDisc *HostDisc) GetNodeIPAddress() string {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		hostDisc.api.Logf(types.LLERROR, "Can not find network interfaces: %v", err)
	}
	ip := ""
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				break
			} else {
				hostDisc.api.Logf(types.LLERROR, "Can not format IP address")
			}
		}
	}

	return ip
}

// Discovers state of the CPU based on CPU temperature thresholds
func (hostDisc *HostDisc) lambdaStateDiscovery(v CPUTempObj) (string, int32) {
	cpuTemp := v.CPUTemp
	cpuTempState := thpb.Temp_CPU_TEMP_NONE

	lowerNormal := hostDisc.cfg.GetLowerNormal()
	upperNormal := hostDisc.cfg.GetUpperNormal()

	lowerHigh := hostDisc.cfg.GetLowerHigh()
	upperHigh := hostDisc.cfg.GetUpperHigh()

	lowerCritical := hostDisc.cfg.GetLowerCritical()
	upperCritical := hostDisc.cfg.GetUpperCritical()

	if cpuTemp <= lowerCritical || cpuTemp >= upperCritical {
		cpuTempState = thpb.Temp_CPU_TEMP_CRITICAL
	} else if cpuTemp >= lowerHigh && cpuTemp < upperHigh {
		cpuTempState = thpb.Temp_CPU_TEMP_HIGH
	} else if cpuTemp > lowerNormal && cpuTemp < upperNormal {
		cpuTempState = thpb.Temp_CPU_TEMP_NORMAL
	}
	return cpuTempState.String(), cpuTemp

}
