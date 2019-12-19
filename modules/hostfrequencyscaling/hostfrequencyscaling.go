/* hostfrequencyscaling.go: performs mutations related to scaling of CPU frequency to control CPU thermal conditions of HPC node using in-band mechanism.
 *
 * This module boots HPC nodes with "schedutil" scaling governor and whenever CPU temperature reaches to high (warning) condition, module mutates the scaling governor to "powersave".
 * Current implementation handles critical CPU temperature same as high CPU temperature.
 *
 * Additionally, there are many other mutations intended for different use cases (e.g. switching back to "schedutil" after "powersave") are under considerations and investinations.
 *
 * Authors: Ghazanfar Ali, ghazanfar.ali@ttu.edu; Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2019, Triad National Security, LLC
 * See LICENSE file for details.
 */

package hostfrequencyscaling

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	scalpb "github.com/hpc/kraken/extensions/HostFrequencyScaler/proto"
	hostthpb "github.com/hpc/kraken/extensions/HostThermal/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/hostfrequencyscaling/proto"
)

// CPUPerfScalingReq is payload for RFAggregator API call
type CPUPerfScalingReq struct {
	ScalingGovernor  string   `json:"scalinggovernor"`
	ScalingMinFreq   string   `json:"scalingminfreq"`
	ScalingMaxFreq   string   `json:"scalingmaxfreq"`
	NodesAddressList []string `json:"nodesaddresslist,omitempty"`
	Timeout          int      `json:"timeout,omitempty"`
}

// CPUPerfScalingResp structure for API response from RFAggregator
type CPUPerfScalingResp struct {
	TimeStamp          time.Time `json:"timestamp"`
	HostAddress        string    `json:"hostaddress"`
	CurScalingGovernor string    `json:"curscalinggovernor"`
	ScalingMinFreq     string    `json:"scalingminfreq"`
	ScalingMaxFreq     string    `json:"scalingmaxfreq"`
	ScalingCurFreq     string    `json:"scalingcurfreq"`

	CPUCurFreq string `json:"cpucurfreq"`
	CPUMinFreq string `json:"cpuminfreq"`
	CPUMaxFreq string `json:"cpumaxfreq"`
}

// CPUPerfScalingRespColl structure for collection of response
type CPUPerfScalingRespColl struct {
	CPUPerfScalingRespCollection []CPUPerfScalingResp `json:"cpuperfscalingrespcollection"`
}

const (
	// PxeURL refers to PXE object
	pxeURL string = "type.googleapis.com/proto.RPi3/Pxe"

	// ModuleStateURL refers to module state
	moduleStateURL string = "/Services/hostfrequencyscaling/State"

	// HostThermalStateURL points to Thermal extension
	hostThermalStateURL string = "type.googleapis.com/proto.HostThermal/State"

	// NodeIPURL provides node IP address
	nodeIPURL string = "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip"

	// hostFreqScalerURL provides URL for host frequency scaler at host run time
	hostFreqScalerURL string = "type.googleapis.com/proto.HostFrequencyScaler/State"

	// freqSensorPath holds frequency sensor path on pi node
	freqSensorPath string = "/sys/devices/system/cpu/cpufreq/policy0/"
)

var profileMap = map[string]string{
	"performance": scalpb.HostFrequencyScaler_PERFORMANCE.String(),
	"powersave":   scalpb.HostFrequencyScaler_POWER_SAVE.String(),
}

type hfscalmut struct {
	f       scalpb.HostFrequencyScaler_ScalerState
	t       scalpb.HostFrequencyScaler_ScalerState
	reqs    map[string]reflect.Value
	timeout string
	failTo  string
}

// modify these if you want different requires for mutations
var scalerReqs = map[string]reflect.Value{
	"/PhysState":   reflect.ValueOf(cpb.Node_POWER_ON),
	"/RunState":    reflect.ValueOf(cpb.Node_SYNC),
	moduleStateURL: reflect.ValueOf(cpb.ServiceInstance_RUN),
}
var scalMuts = map[string]hfscalmut{
	"NONEtoPOWERSAVE": {
		f:       scalpb.HostFrequencyScaler_NONE,
		t:       scalpb.HostFrequencyScaler_POWER_SAVE,
		reqs:    scalerReqs,
		timeout: "60s",
		failTo:  scalpb.HostFrequencyScaler_NONE.String(),
	},
	"PERFORMANCEtoPOWERSAVE": {
		f:       scalpb.HostFrequencyScaler_PERFORMANCE,
		t:       scalpb.HostFrequencyScaler_POWER_SAVE,
		reqs:    scalerReqs,
		timeout: "60s",
		failTo:  scalpb.HostFrequencyScaler_PERFORMANCE.String(),
	},
	"NONEtoPERFORMANCE": {
		f:       scalpb.HostFrequencyScaler_NONE,
		t:       scalpb.HostFrequencyScaler_PERFORMANCE,
		reqs:    scalerReqs,
		timeout: "60s",
		failTo:  scalpb.HostFrequencyScaler_NONE.String(),
	},
	"POWERSAVEtoPERFORMANCE": {
		f: scalpb.HostFrequencyScaler_POWER_SAVE,
		t: scalpb.HostFrequencyScaler_PERFORMANCE,
		reqs: map[string]reflect.Value{
			"/PhysState":        reflect.ValueOf(cpb.Node_POWER_ON),
			"/RunState":         reflect.ValueOf(cpb.Node_SYNC),
			moduleStateURL:      reflect.ValueOf(cpb.ServiceInstance_RUN),
			hostThermalStateURL: reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_NORMAL),
		},
		timeout: "60s",
		failTo:  scalpb.HostFrequencyScaler_POWER_SAVE.String(),
	},
}

// Structure for mutation defintion
type hfsmut struct {
	f       hostthpb.HostThermalCpuState
	t       hostthpb.HostThermalCpuState
	reqs    map[string]reflect.Value
	timeout string
	failTo  string
}

// Mutations supported by this module
var muts = map[string]hfsmut{

	"CPU_TEMP_NONEtoCPU_TEMP_NORMAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_NONE,
		t:       hostthpb.HostThermal_CPU_TEMP_NORMAL,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_NONE.String(),
	},
	"CPU_TEMP_NONEtoCPU_TEMP_HIGH": {
		f:       hostthpb.HostThermal_CPU_TEMP_NONE,
		t:       hostthpb.HostThermal_CPU_TEMP_HIGH,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_NONE.String(),
	},
	"CPU_TEMP_NONEtoCPU_TEMP_CRITICAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_NONE,
		t:       hostthpb.HostThermal_CPU_TEMP_CRITICAL,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_NONE.String(),
	},

	"CPU_TEMP_HIGHtoCPU_TEMP_NORMAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_HIGH,
		t:       hostthpb.HostThermal_CPU_TEMP_NORMAL,
		reqs:    greqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_HIGH.String(),
	},

	"CPU_TEMP_CRITICALtoCPU_TEMP_HIGH": {
		f:       hostthpb.HostThermal_CPU_TEMP_CRITICAL,
		t:       hostthpb.HostThermal_CPU_TEMP_HIGH,
		reqs:    greqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_CRITICAL.String(),
	},

	"CPU_TEMP_CRITICALtoCPU_TEMP_NORMAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_CRITICAL,
		t:       hostthpb.HostThermal_CPU_TEMP_NORMAL,
		reqs:    greqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_CRITICAL.String(),
	},
}

// HFS provides rfcpufreqscaling module capabilities
type HFS struct {
	api lib.APIClient
	cfg *pb.HostFreqScalingConfig

	mchan <-chan lib.Event
	dchan chan<- lib.Event
}

var _ lib.Module = (*HFS)(nil)
var _ lib.ModuleWithConfig = (*HFS)(nil)
var _ lib.ModuleWithMutations = (*HFS)(nil)
var _ lib.ModuleWithDiscovery = (*HFS)(nil)
var _ lib.ModuleSelfService = (*HFS)(nil)

// Name returns the FQDN of the module
func (*HFS) Name() string { return "github.com/hpc/kraken/modules/hostfrequencyscaling" }

// NewConfig returns a fully initialized default config
func (*HFS) NewConfig() proto.Message {
	r := &pb.HostFreqScalingConfig{
		FreqSensorUrl:     freqSensorPath,
		ScalingFreqPolicy: hostFreqScalerURL,

		FreqScalPolicies: map[string]*pb.HostFreqScalingPolicy{
			"powersave": {
				ScalingGovernor: "powersave",
				ScalingMinFreq:  "600000",
				ScalingMaxFreq:  "1400000",
				NodeArch:        "",
				NodePlatform:    "",
			},
			"performance": {
				ScalingGovernor: "performance",
				ScalingMinFreq:  "600000",
				ScalingMaxFreq:  "1400000",
				NodeArch:        "",
				NodePlatform:    "",
			},
			"schedutil": {
				ScalingGovernor: "schedutil",
				ScalingMinFreq:  "600000",
				ScalingMaxFreq:  "1400000",
				NodeArch:        "",
				NodePlatform:    "",
			},
		},
	}
	return r
}

// UpdateConfig updates the running config
func (hfs *HFS) UpdateConfig(cfg proto.Message) (e error) {
	if rcfg, ok := cfg.(*pb.HostFreqScalingConfig); ok {
		hfs.cfg = rcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*HFS) ConfigURL() string {
	cfg := &pb.HostFreqScalingConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (hfs *HFS) SetMutationChan(c <-chan lib.Event) { hfs.mchan = c }

// SetDiscoveryChan sets the current discovery channel
func (hfs *HFS) SetDiscoveryChan(d chan<- lib.Event) { hfs.dchan = d }

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/PhysState":   reflect.ValueOf(cpb.Node_POWER_ON),
	"/RunState":    reflect.ValueOf(cpb.Node_SYNC),
	moduleStateURL: reflect.ValueOf(cpb.ServiceInstance_RUN),
}

var greqs = map[string]reflect.Value{
	"/PhysState":      reflect.ValueOf(cpb.Node_POWER_ON),
	"/RunState":       reflect.ValueOf(cpb.Node_SYNC),
	moduleStateURL:    reflect.ValueOf(cpb.ServiceInstance_RUN),
	hostFreqScalerURL: reflect.ValueOf(scalpb.HostFrequencyScaler_POWER_SAVE),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

// Init is used to intialize an executable module prior to entrypoint
func (hfs *HFS) Init(api lib.APIClient) {
	hfs.api = api
	//hfs.mutex = &sync.Mutex{}
	// hfs.queue = make(map[string]map[string]NMut)
	hfs.cfg = hfs.NewConfig().(*pb.HostFreqScalingConfig)
}

// Stop should perform a graceful exit
func (hfs *HFS) Stop() {
	os.Exit(0)
}

func init() {
	module := &HFS{}
	mutations := make(map[string]lib.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	hostFreqScalerDiscs := make(map[string]reflect.Value)
	hostThermDiscs := make(map[string]reflect.Value)
	si := core.NewServiceInstance("hostfrequencyscaling", module.Name(), module.Entry, nil)

	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_NONE.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_NONE)
	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_NORMAL.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_NORMAL)
	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_HIGH.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_HIGH)
	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_CRITICAL.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_CRITICAL)

	discovers[hostThermalStateURL] = hostThermDiscs
	discovers[moduleStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	hostFreqScalerDiscs[scalpb.HostFrequencyScaler_NONE.String()] = reflect.ValueOf(scalpb.HostFrequencyScaler_NONE)
	hostFreqScalerDiscs[scalpb.HostFrequencyScaler_PERFORMANCE.String()] = reflect.ValueOf(scalpb.HostFrequencyScaler_PERFORMANCE)
	hostFreqScalerDiscs[scalpb.HostFrequencyScaler_POWER_SAVE.String()] = reflect.ValueOf(scalpb.HostFrequencyScaler_POWER_SAVE)
	discovers[hostFreqScalerURL] = hostFreqScalerDiscs

	for k, m := range muts {
		dur, _ := time.ParseDuration(m.timeout)
		mutations[k] = core.NewStateMutation(
			map[string][2]reflect.Value{
				hostThermalStateURL: {
					reflect.ValueOf(m.f),
					reflect.ValueOf(m.t),
				},
			},
			m.reqs,
			excs,
			lib.StateMutationContext_SELF,
			dur,
			[3]string{si.ID(), hostThermalStateURL, m.failTo},
		)
	}

	for k, m := range scalMuts {
		dur, _ := time.ParseDuration(m.timeout)
		mutations[k] = core.NewStateMutation(
			map[string][2]reflect.Value{
				hostFreqScalerURL: {
					reflect.ValueOf(m.f),
					reflect.ValueOf(m.t),
				},
			},
			m.reqs,
			map[string]reflect.Value{
				hostThermalStateURL: reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_NONE),
			},
			lib.StateMutationContext_SELF,
			dur,
			[3]string{si.ID(), hostFreqScalerURL, m.failTo},
		)
	}

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterMutations(si, mutations)
	core.Registry.RegisterDiscoverable(si, discovers)
}

// Entry is the module's executable entrypoint
func (hfs *HFS) Entry() {

	url := lib.NodeURLJoin(hfs.api.Self().String(), moduleStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{

			URL:     url,
			ValueID: "RUN",
		},
	)
	hfs.dchan <- ev

	for {

		select {
		case m := <-hfs.mchan:

			go hfs.mutateCPUFreq(m)
			break

		}
	}
}

// aggregateHandler makes calls to aggregator for the given nodes with related mutation and frequecy scaling policy
func (hfs *HFS) mutateCPUFreq(m lib.Event) {

	if m.Type() != lib.Event_STATE_MUTATION {
		hfs.api.Log(lib.LLERROR, "got unexpected non-mutation event")
		return
	}
	me := m.Data().(*core.MutationEvent)

	switch me.Mutation[1] {
	case "NONEtoPOWERSAVE":
		fallthrough
	case "PERFORMANCEtoPOWERSAVE":
		hfs.HostFrequencyScaling(me.NodeCfg, "powersave")
		break
	case "NONEtoPERFORMANCE":
		fallthrough
	case "POWERSAVEtoPERFORMANCE":
		hfs.HostFrequencyScaling(me.NodeCfg, "performance")
		break
	}

}

// HostFrequencyScaling scales CPU frequency according to given parameters
func (hfs *HFS) HostFrequencyScaling(node lib.Node, freqScalPolicy string) {

	hfs.api.Log(lib.LLERROR, "POLICY: %s", freqScalPolicy)

	freqScalPolicies := hfs.cfg.GetFreqScalPolicies()

	scalingGovernor := freqScalPolicies[freqScalPolicy].GetScalingGovernor()
	scalingMinFreq := freqScalPolicies[freqScalPolicy].GetScalingMinFreq()
	scalingMaxFreq := freqScalPolicies[freqScalPolicy].GetScalingMaxFreq()

	basePath := hfs.cfg.GetFreqSensorUrl() //"/sys/devices/system/cpu/cpufreq/policy0/"

	// Set the CPU frequency scaling parameters
	_ = ioutil.WriteFile(basePath+"scaling_governor", []byte(scalingGovernor), 0644)
	_ = ioutil.WriteFile(basePath+"scaling_max_freq", []byte(scalingMaxFreq), 0644)
	_ = ioutil.WriteFile(basePath+"scaling_min_freq", []byte(scalingMinFreq), 0644)

	// Get the CPU frequency scaling parameters
	bscalingGovernor, _ := ioutil.ReadFile(basePath + "scaling_governor")
	bscalingMaxFreq, _ := ioutil.ReadFile(basePath + "scaling_max_freq")
	bscalingMinFreq, _ := ioutil.ReadFile(basePath + "scaling_min_freq")

	cpuCurFreq, _ := ioutil.ReadFile(basePath + "cpuinfo_cur_freq")
	cpuMinFreq, _ := ioutil.ReadFile(basePath + "cpuinfo_min_freq")
	cpuMaxFreq, _ := ioutil.ReadFile(basePath + "cpuinfo_max_freq")
	scalingCurFreq, _ := ioutil.ReadFile(basePath + "scaling_cur_freq")

	fscalingGovernor := strings.TrimSuffix(string(bscalingGovernor), "\n")

	scalingMinFreqq := strings.TrimSuffix(string(bscalingMinFreq), "\n")
	scalingMaxFreqq := strings.TrimSuffix(string(bscalingMaxFreq), "\n")
	scalingCurFreqq := strings.TrimSuffix(string(scalingCurFreq), "\n")
	cpuCurFreqq := strings.TrimSuffix(string(cpuCurFreq), "\n")
	cpuMinFreqq := strings.TrimSuffix(string(cpuMinFreq), "\n")
	cpuMaxFreqq := strings.TrimSuffix(string(cpuMaxFreq), "\n")

	hostIP := GetNodeIPAddress()

	currentScalingConfig := CPUPerfScalingResp{
		TimeStamp:          time.Now(),
		HostAddress:        hostIP,
		CurScalingGovernor: fscalingGovernor,
		ScalingMinFreq:     scalingMinFreqq,
		ScalingMaxFreq:     scalingMaxFreqq,
		ScalingCurFreq:     scalingCurFreqq,

		CPUCurFreq: cpuCurFreqq,
		CPUMinFreq: cpuMinFreqq,
		CPUMaxFreq: cpuMaxFreqq,
	}

	//if bBoot == true {
	url := lib.NodeURLJoin(node.ID().String(), hostFreqScalerURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: profileMap[currentScalingConfig.CurScalingGovernor],
		},
	)
	hfs.dchan <- ev
	// } else {
	// 	url := lib.NodeURLJoin(node.ID().String(), hostHightoLowFreqScalerURL)
	// 	ev := core.NewEvent(
	// 		lib.Event_DISCOVERY,
	// 		url,
	// 		&core.DiscoveryEvent{
	// 			URL:     url,
	// 			ValueID: currentScalingConfig.CurScalingGovernor,
	// 		},
	// 	)
	// 	hfs.dchan <- ev
	// }

}

// GetNodeIPAddress acquires node IP address
func GetNodeIPAddress() string {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalf("could not obtain host IP address: %v", err)
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

}
