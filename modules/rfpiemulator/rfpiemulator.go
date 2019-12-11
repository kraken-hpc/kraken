/* rfpiemulator.go: This module is intended to run pi-node and delivers real-time and emulated telemetry data. This data is exposed via Redfish API methodology.
 *
 * Currently this module exposes CPU thermal, get/set CPU frequency in realtime, and performs soft power control in real-time.
 * It also simulates delivery of power usage (in watts) on CPU and memory level. The goal is to attach real-time interfaces (e.g. Intel RAPL) to get the real-time functionalities.
 *
 * Additionally, there are many other functionalities are under investinations and considerations.
 *
 * Authors: Ghazanfar Ali, ghazanfar.ali@ttu.edu; Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2019, Triad National Security, LLC
 * See LICENSE file for details.
 */

package rfpiemulator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/gorilla/mux"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/rfpiemulator/proto"
)

// CPUTempObj CPU Temp object
type CPUTempObj struct {
	TimeStamp   time.Time
	HostAddress string
	CPUTemp     int
}

//CPUPower CPU power object
type CPUPower struct {
	Socket1CPUPowerUsage float64 `json:"socket1cpupowerusage"`
	Socket2CPUPowerUsage float64 `json:"socket2cpupowerusage"`
}

// MemPower socket level representation
type MemPower struct {
	Socket1MemPowerUsage float64 `json:"socket1mempowerusage"`
	Socket2MemPowerUsage float64 `json:"socket2mempowerusage"`
}

//CPUPowerObj CPU power object
type CPUPowerObj struct {
	TimeStamp     time.Time
	HostAddress   string
	CPUPowerUsage CPUPower
}

//MemPowerObj memory object
type MemPowerObj struct {
	TimeStamp     time.Time `json:"timestamp"`
	HostAddress   string    `json:"hostaddress"`
	MemPowerUsage MemPower  `json:"mempowerusage"`
}

type payLoad struct {
	ResetType string
}

// CPUPerfScalingReq Payload for CPU performance scaling request
type CPUPerfScalingReq struct {
	ScalingGovernor  string   `json:"scalinggovernor"`
	ScalingMinFreq   int      `json:"scalingminfreq"`
	ScalingMaxFreq   int      `json:"scalingmaxfreq"`
	NodesAddressList []string `json:"nodesaddresslist,omitempty"`
	Timeout          int      `json:"timeout,omitempty"`
}

//CPUPerfScalingResp  structure
type CPUPerfScalingResp struct {
	TimeStamp          time.Time `json:"timestamp"`
	HostAddress        string    `json:"hostaddress"`
	CurScalingGovernor string    `json:"curscalinggovernor"`
	ScalingMinFreq     int       `json:"scalingminfreq"`
	ScalingMaxFreq     int       `json:"scalingmaxfreq"`
	ScalingCurFreq     int       `json:"scalingcurfreq"`

	CPUCurFreq int `json:"cpucurfreq"`
	CPUMinFreq int `json:"cpuminfreq"`
	CPUMaxFreq int `json:"cpumaxfreq"`
}

const (
	// ModuleStateURL refers to module state
	ModuleStateURL = "/Services/rfpiemulator/State"
)

var _ lib.Module = (*RFPiEmu)(nil)
var _ lib.ModuleWithConfig = (*RFPiEmu)(nil)
var _ lib.ModuleWithDiscovery = (*RFPiEmu)(nil)
var _ lib.ModuleSelfService = (*RFPiEmu)(nil)

// RFPiEmu provides hostdiscovery module capabilities
type RFPiEmu struct {
	prevTemp int32
	api      lib.APIClient
	cfg      *pb.RFEmulatorConfig
	dchan    chan<- lib.Event
}

// Name returns the FQDN of the module
func (*RFPiEmu) Name() string { return "github.com/hpc/kraken/modules/rfpiemulator" }

// NewConfig returns a fully initialized default config
func (rfPiEmu *RFPiEmu) NewConfig() proto.Message {
	localIP := rfPiEmu.GetNodeIPAddress()
	r := &pb.RFEmulatorConfig{
		EmulatorIp:     localIP,
		EmulatorPort:   "8000",
		TempSensorPath: "/sys/devices/virtual/thermal/thermal_zone0/temp",
		FreqSensorPath: "/sys/devices/system/cpu/cpufreq/policy0/",
		PwrSensorPath:  "",
	}
	return r
}

// UpdateConfig updates the running config
func (rfPiEmu *RFPiEmu) UpdateConfig(cfg proto.Message) (e error) {
	if rcfg, ok := cfg.(*pb.RFEmulatorConfig); ok {
		rfPiEmu.cfg = rcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*RFPiEmu) ConfigURL() string {
	cfg := &pb.RFEmulatorConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (rfPiEmu *RFPiEmu) SetDiscoveryChan(c chan<- lib.Event) { rfPiEmu.dchan = c }

func init() {
	module := &RFPiEmu{}
	discovers := make(map[string]map[string]reflect.Value)

	discovers[ModuleStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	si := core.NewServiceInstance("rfpiemulator", module.Name(), module.Entry, nil)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
}

// Init is used to intialize an executable module prior to entrypoint
func (rfPiEmu *RFPiEmu) Init(api lib.APIClient) {
	rfPiEmu.api = api
	rfPiEmu.cfg = rfPiEmu.NewConfig().(*pb.RFEmulatorConfig)
}

// Stop should perform a graceful exit
func (rfPiEmu *RFPiEmu) Stop() {
	os.Exit(0)
}

// Entry is the module's executable entrypoint
func (rfPiEmu *RFPiEmu) Entry() {
	url := lib.NodeURLJoin(rfPiEmu.api.Self().String(), ModuleStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{

			URL:     url,
			ValueID: "RUN",
		},
	)
	rfPiEmu.dchan <- ev

	router := mux.NewRouter()

	router.HandleFunc("/redfish/v1/Systems/{SystemID}/Processors/power", rfPiEmu.GetProcessorPowerUsage).Methods(http.MethodGet)
	router.HandleFunc("/redfish/v1/Systems/{SystemID}/Memory/power", rfPiEmu.GetMemoryPowerUsage).Methods(http.MethodGet)

	router.HandleFunc("/redfish/v1/Chassis/{ChassisID}/Thermal", rfPiEmu.GetCPUTemp).Methods(http.MethodGet)
	router.HandleFunc("/redfish/v1/Chassis/{ChassisID}/ScaleCPUFreq", rfPiEmu.ScaleCPUFreq).Methods(http.MethodPut)
	router.HandleFunc("/redfish/v1/Systems/{SystemID}/Actions/Reset", rfPiEmu.NodePowerControl).Methods(http.MethodPut)

	piIPAddr := rfPiEmu.cfg.GetEmulatorIp()
	piPort := rfPiEmu.cfg.GetEmulatorPort()
	piSrv := net.JoinHostPort(piIPAddr, piPort)
	err := http.ListenAndServe(piSrv, router)
	rfPiEmu.api.Logf(lib.LLERROR, "can't start rf emulator on pi node: %v", err)
}

// NodePowerControl performs power control of pi node
func (rfPiEmu *RFPiEmu) NodePowerControl(w http.ResponseWriter, r *http.Request) {

	var reset payLoad
	var result []byte
	var msg string

	err := json.NewDecoder(r.Body).Decode(&reset)
	if err != nil {
		rfPiEmu.api.Logf(lib.LLERROR, "can't decode request: %v", err)
	} else {
		resetType := reset.ResetType
		if resetType == "r" || resetType == "reboot" {
			command := "/bbin/shutdown reboot"
			cmdSplit := strings.Split(command, " ")
			cmd := exec.Command(cmdSplit[0], cmdSplit[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			if err := cmd.Run(); err != nil {
				msg = fmt.Sprintf("Could not reboot node: %v", err)
			} else {
				msg = "\nRebooting the node!\n"
			}
			rfPiEmu.api.Logf(lib.LLERROR, "%v", msg)

			result, err = json.Marshal(msg)
			if err != nil {
				rfPiEmu.api.Logf(lib.LLERROR, "%v", err)
			}
			w.Write(result)
		} else if resetType == "h" || resetType == "halt" {
			command := "/bbin/shutdown halt"
			cmdSplit := strings.Split(command, " ")
			cmd := exec.Command(cmdSplit[0], cmdSplit[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			if err := cmd.Run(); err != nil {
				msg = fmt.Sprintf("Could not power off node: %v", err)
			} else {
				msg = "\nPowering off the node!\n"
			}
			rfPiEmu.api.Logf(lib.LLINFO, "%v", msg)
			result, err = json.Marshal(msg)
			if err != nil {
				rfPiEmu.api.Logf(lib.LLERROR, "%v", err)
			}
			w.Write(result)
		}
	}

}

// GetNodeIPAddress returns IP address of pi node
func (rfPiEmu *RFPiEmu) GetNodeIPAddress() string {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		rfPiEmu.api.Logf(lib.LLERROR, "%v", err)
	}
	ip := ""
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				break
			}
		}
	}

	return ip

	/*
		     hostname, err := os.Hostname()
		     if err != nil {
				 hostname = "nil"
				 rfPiEmu.api.Logf(lib.LLERROR, "%v", err)
		     }
		     return hostname
	*/
}

// randTemperature returns random temperature to simulate CPU temperature of pi node
func (rfPiEmu *RFPiEmu) randTemperature(min, max float64) float64 {
	rand.Seed(time.Now().UnixNano())
	return math.Floor((min+rand.Float64()*(max-min))*100) / 100
}

// ReadCPUTemp reads CPU temperature of pi node
func (rfPiEmu *RFPiEmu) ReadCPUTemp() int {
	tempSensorPath := rfPiEmu.cfg.GetTempSensorPath()
	//tempSensorPath := "/sys/devices/virtual/thermal/thermal_zone0/temp"
	cpuTemp, err := ioutil.ReadFile(tempSensorPath)
	if err != nil {
		rfPiEmu.api.Logf(lib.LLERROR, "temperature sensor read error on pi node: %v", err)
	}

	cpuTempInt, e := strconv.Atoi(strings.TrimSuffix(string(cpuTemp), "\n"))
	if e != nil {
		rfPiEmu.api.Logf(lib.LLERROR, "ascci to int conversion error: %v", e)
	}
	rfPiEmu.api.Logf(lib.LLERROR, "TEMP: %v", "PATH: %v", cpuTemp, tempSensorPath)
	return cpuTempInt
}

// GetCPUTemp acquires CPU temperature
func (rfPiEmu *RFPiEmu) GetCPUTemp(w http.ResponseWriter, r *http.Request) {

	hostIP := rfPiEmu.GetNodeIPAddress()

	// Its a mockup CPU temperature
	cpuTempObj := new(CPUTempObj)
	cpuTempObj.TimeStamp = time.Now()
	cpuTempObj.HostAddress = hostIP

	tempVal := rfPiEmu.ReadCPUTemp()
	//tempVal := randTemperature(1, 100)
	cpuTempObj.CPUTemp = tempVal

	jsonObj, err := json.Marshal(cpuTempObj)

	if err != nil {
		rfPiEmu.api.Logf(lib.LLERROR, "%v", err)
	}
	w.Write(jsonObj)

}

// GetProcessorPowerUsage acquires CPU power usage
func (rfPiEmu *RFPiEmu) GetProcessorPowerUsage(w http.ResponseWriter, r *http.Request) {
	hostIP := rfPiEmu.GetNodeIPAddress()
	cpuPwrObj := new(CPUPowerObj)
	cpuPwrObj.TimeStamp = time.Now()
	cpuPwrObj.HostAddress = hostIP
	cpuPwrObj.CPUPowerUsage.Socket1CPUPowerUsage = 150.25
	cpuPwrObj.CPUPowerUsage.Socket2CPUPowerUsage = 80.30
	jsonObj, err := json.Marshal(cpuPwrObj)
	if err != nil {
		rfPiEmu.api.Logf(lib.LLERROR, "%v", err)
	}
	w.Write(jsonObj)

}

// GetMemoryPowerUsage simulate values for memory power usage
func (rfPiEmu *RFPiEmu) GetMemoryPowerUsage(w http.ResponseWriter, r *http.Request) {
	hostIP := rfPiEmu.GetNodeIPAddress()
	memPwrObj := new(MemPowerObj)
	memPwrObj.TimeStamp = time.Now()
	memPwrObj.HostAddress = hostIP
	memPwrObj.MemPowerUsage.Socket1MemPowerUsage = 60.99
	memPwrObj.MemPowerUsage.Socket2MemPowerUsage = 40.56
	jsonObj, err := json.Marshal(memPwrObj)
	if err != nil {
		rfPiEmu.api.Logf(lib.LLERROR, "%v", err)
	}
	w.Write(jsonObj)
}

// ScaleCPUFreq scale CPU frequency of pi node
func (rfPiEmu *RFPiEmu) ScaleCPUFreq(w http.ResponseWriter, r *http.Request) {

	var reqPayload CPUPerfScalingReq

	err := json.NewDecoder(r.Body).Decode(&reqPayload)
	if err != nil {
		rfPiEmu.api.Logf(lib.LLERROR, "%v", err)
	} else {

		// Extracting the desired scaling governor, scaling minimum, scaling maximum frequencies
		scalingGovernor := []byte(reqPayload.ScalingGovernor)
		scalingMaxFreq := []byte(strconv.Itoa(reqPayload.ScalingMaxFreq))
		scalingMinFreq := []byte(strconv.Itoa(reqPayload.ScalingMinFreq))

		basePath := rfPiEmu.cfg.GetFreqSensorPath()
		//basePath := "/sys/devices/system/cpu/cpufreq/policy0/"

		// Set the CPU frequency scaling parameters
		_ = ioutil.WriteFile(basePath+"scaling_governor", scalingGovernor, 0644)
		_ = ioutil.WriteFile(basePath+"scaling_max_freq", scalingMaxFreq, 0644)
		_ = ioutil.WriteFile(basePath+"scaling_min_freq", scalingMinFreq, 0644)

		// Get the CPU frequency scaling parameters
		scalingGovernor, _ = ioutil.ReadFile(basePath + "scaling_governor")
		scalingMaxFreq, _ = ioutil.ReadFile(basePath + "scaling_max_freq")
		scalingMinFreq, _ = ioutil.ReadFile(basePath + "scaling_min_freq")

		cpuCurFreq, _ := ioutil.ReadFile(basePath + "cpuinfo_cur_freq")
		cpuMinFreq, _ := ioutil.ReadFile(basePath + "cpuinfo_min_freq")
		cpuMaxFreq, _ := ioutil.ReadFile(basePath + "cpuinfo_max_freq")
		scalingCurFreq, _ := ioutil.ReadFile(basePath + "scaling_cur_freq")

		scalingMinFreqqInt, e := strconv.Atoi(strings.TrimSuffix(string(scalingMinFreq), "\n"))
		scalingMaxFreqqInt, e := strconv.Atoi(strings.TrimSuffix(string(scalingMaxFreq), "\n"))
		scalingCurFreqqInt, e := strconv.Atoi(strings.TrimSuffix(string(scalingCurFreq), "\n"))
		cpuCurFreqqInt, e := strconv.Atoi(strings.TrimSuffix(string(cpuCurFreq), "\n"))
		cpuMinFreqqInt, e := strconv.Atoi(strings.TrimSuffix(string(cpuMinFreq), "\n"))
		cpuMaxFreqqInt, e := strconv.Atoi(strings.TrimSuffix(string(cpuMaxFreq), "\n"))

		hostIP := rfPiEmu.GetNodeIPAddress()

		resPayload, e := json.Marshal(CPUPerfScalingResp{
			TimeStamp:          time.Now(),
			HostAddress:        hostIP,
			CurScalingGovernor: string(scalingGovernor),
			ScalingMinFreq:     scalingMinFreqqInt,
			ScalingMaxFreq:     scalingMaxFreqqInt,
			ScalingCurFreq:     scalingCurFreqqInt,

			CPUCurFreq: cpuCurFreqqInt,
			CPUMinFreq: cpuMinFreqqInt,
			CPUMaxFreq: cpuMaxFreqqInt,
		})
		if e == nil {

			w.Write(resPayload)
		} else {
			rfPiEmu.api.Logf(lib.LLERROR, "%v", e)
			w.Write([]byte(e.Error()))
		}

	}

}
