/* rffrequencyscaling.go: performs mutations related to scaling of CPU frequency to control CPU thermal conditions of HPC nodes via Redfish API using the RFAggregator (REST API server).
 *
 * This module boots HPC nodes with "schedutil" scaling governor and whenever CPU temperature reaches to high (warning) condition, module mutates the scaling governor to "powersave".
 * Current implementation handles critical CPU temperature same as high CPU temperature. Other actions such as node power off is under consideration.
 *
 * Additionally, there are many other mutations intended for different use cases (e.g. switching back to "schedutil" after "powersave") are under considerations and investinations.
 *
 * Authors: Ghazanfar Ali, ghazanfar.ali@ttu.edu; Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2019, Triad National Security, LLC
 * See LICENSE file for details.
 */

package rffrequencyscaling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	hostthpb "github.com/hpc/kraken/extensions/HostThermal/proto"
	"github.com/hpc/kraken/extensions/IPv4"
	rp3pb "github.com/hpc/kraken/extensions/RPi3/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/rffrequencyscaling/proto"
)

// CPUPerfScalingReq is payload for RFAggregator API call
type CPUPerfScalingReq struct {
	ScalingGovernor  string   `json:"scalinggovernor"`
	ScalingMinFreq   int32    `json:"scalingminfreq"`
	ScalingMaxFreq   int32    `json:"scalingmaxfreq"`
	NodesAddressList []string `json:"nodesaddresslist,omitempty"`
	Timeout          int32    `json:"timeout,omitempty"`
}

// CPUPerfScalingResp structure for API response from RFAggregator
type CPUPerfScalingResp struct {
	TimeStamp          time.Time `json:"timestamp"`
	HostAddress        string    `json:"hostaddress"`
	CurScalingGovernor string    `json:"curscalinggovernor"`
	ScalingMinFreq     int32     `json:"scalingminfreq"`
	ScalingMaxFreq     int32     `json:"scalingmaxfreq"`
	ScalingCurFreq     int32     `json:"scalingcurfreq"`

	CPUCurFreq int32 `json:"cpucurfreq"`
	CPUMinFreq int32 `json:"cpuminfreq"`
	CPUMaxFreq int32 `json:"cpumaxfreq"`
}

// CPUPerfScalingRespColl structure for collection of response
type CPUPerfScalingRespColl struct {
	CPUPerfScalingRespCollection []CPUPerfScalingResp `json:"cpuperfscalingrespcollection"`
}

const (
	// PxeURL refers to PXE object
	PxeURL string = "type.googleapis.com/proto.RPi3/Pxe"

	// ModuleStateURL refers to module state
	ModuleStateURL string = "/Services/rffrequencyscaling/State"

	// HostThermalStateURL points to Thermal extension
	HostThermalStateURL string = "type.googleapis.com/proto.HostThermal/State"

	// NodeIPURL provides node IP address
	NodeIPURL string = "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip"

	// AggURL provides URL for aggregator
	AggURL string = "type.googleapis.com/proto.RFAggregatorServer/RfAggregator"
)

// Structure for mutation defintion
type cfsmut struct {
	f       hostthpb.HostThermal_CPU_TEMP_STATE
	t       hostthpb.HostThermal_CPU_TEMP_STATE
	reqs    map[string]reflect.Value
	timeout string
	failTo  string
}

// Mutations supported by this module
var muts = map[string]cfsmut{
	"CPU_TEMP_NONEtoCPU_TEMP_UNKNOWN": {
		f:       hostthpb.HostThermal_CPU_TEMP_NONE,
		t:       hostthpb.HostThermal_CPU_TEMP_UNKNOWN,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_NONE.String(),
	},

	"CPU_TEMP_UNKNOWNtoCPU_TEMP_NORMAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_UNKNOWN,
		t:       hostthpb.HostThermal_CPU_TEMP_NORMAL,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_UNKNOWN.String(),
	},
	"CPU_TEMP_UNKNOWNtoCPU_TEMP_HIGH": {
		f:       hostthpb.HostThermal_CPU_TEMP_UNKNOWN,
		t:       hostthpb.HostThermal_CPU_TEMP_HIGH,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_UNKNOWN.String(),
	},
	"CPU_TEMP_UNKNOWNtoCPU_TEMP_CRITICAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_UNKNOWN,
		t:       hostthpb.HostThermal_CPU_TEMP_CRITICAL,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_UNKNOWN.String(),
	},

	"CPU_TEMP_NORMALtoCPU_TEMP_HIGH": {
		f:       hostthpb.HostThermal_CPU_TEMP_NORMAL,
		t:       hostthpb.HostThermal_CPU_TEMP_HIGH,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_NORMAL.String(),
	},

	"CPU_TEMP_HIGHtoCPU_TEMP_NORMAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_HIGH,
		t:       hostthpb.HostThermal_CPU_TEMP_NORMAL,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_HIGH.String(),
	},

	"CPU_TEMP_HIGHtoCPU_TEMP_CRITICAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_HIGH,
		t:       hostthpb.HostThermal_CPU_TEMP_CRITICAL,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_HIGH.String(),
	},

	"CPU_TEMP_CRITICALtoCPU_TEMP_HIGH": {
		f:       hostthpb.HostThermal_CPU_TEMP_CRITICAL,
		t:       hostthpb.HostThermal_CPU_TEMP_HIGH,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_CRITICAL.String(),
	},

	"CPU_TEMP_NORMALtoCPU_TEMP_CRITICAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_NORMAL,
		t:       hostthpb.HostThermal_CPU_TEMP_CRITICAL,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_NORMAL.String(),
	},

	"CPU_TEMP_CRITICALtoCPU_TEMP_NORMAL": {
		f:       hostthpb.HostThermal_CPU_TEMP_CRITICAL,
		t:       hostthpb.HostThermal_CPU_TEMP_NORMAL,
		reqs:    reqs,
		timeout: "60s",
		failTo:  hostthpb.HostThermal_CPU_TEMP_CRITICAL.String(),
	},
}

// NMut keep node CPU temerature mutations
type NMut struct {
	Node     lib.Node
	Mutation string
}

// CFS provides rfcpufreqscaling module capabilities
type CFS struct {
	api         lib.APIClient
	cfg         *pb.RFCPUFreqScalingConfig
	mutex       *sync.Mutex
	mchan       <-chan lib.Event
	dchan       chan<- lib.Event
	queue       map[string]map[string]NMut // map[<aggregator>][<NodeID>, {<Mutation><NODE>]
	checkTicker *time.Ticker
}

var _ lib.Module = (*CFS)(nil)
var _ lib.ModuleWithConfig = (*CFS)(nil)
var _ lib.ModuleWithMutations = (*CFS)(nil)
var _ lib.ModuleWithDiscovery = (*CFS)(nil)
var _ lib.ModuleSelfService = (*CFS)(nil)

// Name returns the FQDN of the module
func (*CFS) Name() string { return "github.com/hpc/kraken/modules/rffrequencyscaling" }

// NewConfig returns a fully initialized default config
func (*CFS) NewConfig() proto.Message {
	r := &pb.RFCPUFreqScalingConfig{
		IpUrl:         NodeIPURL,
		AggUrl:        AggURL,
		CheckInterval: "1s",
		Servers: map[string]*pb.RFCPUFreqScalingServer{
			"c4": {
				Name:       "c4",
				Ip:         "10.15.247.200",
				Port:       "8000",
				ReqTimeout: 250,
			},
		},

		FreqScalPolicies: map[string]*pb.RFCPUFreqScalingPolicy{
			"powersave": {
				ScalingGovernor: "powersave",
				ScalingMinFreq:  600000,
				ScalingMaxFreq:  1200000,
				NodeArch:        "",
				NodePlatform:    "",
			},
			"schedutil": {
				ScalingGovernor: "schedutil",
				ScalingMinFreq:  600000,
				ScalingMaxFreq:  1200000,
				NodeArch:        "",
				NodePlatform:    "",
			},
		},
	}
	return r
}

// UpdateConfig updates the running config
func (cfs *CFS) UpdateConfig(cfg proto.Message) (e error) {
	if rcfg, ok := cfg.(*pb.RFCPUFreqScalingConfig); ok {
		cfs.cfg = rcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*CFS) ConfigURL() string {
	cfg := &pb.RFCPUFreqScalingConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetMutationChan sets the current mutation channel
// this is generally done by the API
func (cfs *CFS) SetMutationChan(c <-chan lib.Event) { cfs.mchan = c }

// SetDiscoveryChan sets the current discovery channel
func (cfs *CFS) SetDiscoveryChan(d chan<- lib.Event) { cfs.dchan = d }

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/PhysState": reflect.ValueOf(cpb.Node_POWER_ON),
	"/RunState":  reflect.ValueOf(cpb.Node_SYNC),
	PxeURL:       reflect.ValueOf(rp3pb.RPi3_INIT),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

// Init is used to intialize an executable module prior to entrypoint
func (cfs *CFS) Init(api lib.APIClient) {
	cfs.api = api
	cfs.mutex = &sync.Mutex{}
	cfs.queue = make(map[string]map[string]NMut)
	cfs.cfg = cfs.NewConfig().(*pb.RFCPUFreqScalingConfig)
}

// Stop should perform a graceful exit
func (cfs *CFS) Stop() {
	os.Exit(0)
}

func init() {
	module := &CFS{}
	mutations := make(map[string]lib.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	hostThermDiscs := make(map[string]reflect.Value)

	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_NONE.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_NONE)
	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_UNKNOWN.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_UNKNOWN)
	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_NORMAL.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_NORMAL)
	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_HIGH.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_HIGH)
	hostThermDiscs[hostthpb.HostThermal_CPU_TEMP_CRITICAL.String()] = reflect.ValueOf(hostthpb.HostThermal_CPU_TEMP_CRITICAL)

	discovers[HostThermalStateURL] = hostThermDiscs

	discovers[ModuleStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	for k, m := range muts {
		dur, _ := time.ParseDuration(m.timeout)
		mutations[k] = core.NewStateMutation(
			map[string][2]reflect.Value{
				HostThermalStateURL: {
					reflect.ValueOf(m.f),
					reflect.ValueOf(m.t),
				},
			},
			m.reqs,
			excs,
			lib.StateMutationContext_CHILD,
			dur,
			[3]string{module.Name(), HostThermalStateURL, m.failTo},
		)
	}

	si := core.NewServiceInstance("rffrequencyscaling", module.Name(), module.Entry, nil)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterMutations(si, mutations)
	core.Registry.RegisterDiscoverable(si, discovers)
}

// Entry is the module's executable entrypoint
func (cfs *CFS) Entry() {

	url := lib.NodeURLJoin(cfs.api.Self().String(), ModuleStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{

			URL:     url,
			ValueID: "RUN",
		},
	)
	cfs.dchan <- ev

	// recieving and applying CPU temp. related mutations periodically
	for {

		dur, _ := time.ParseDuration(cfs.cfg.GetCheckInterval())
		cfs.checkTicker = time.NewTicker(dur)

		select {
		case m := <-cfs.mchan:
			// recieve CPU Temp. related mutations
			go cfs.rcvMutations(m)
			break
		case <-cfs.checkTicker.C:
			// mutate CPU frequencies based on recieved mutations
			go cfs.mutateCPUTemp()
			break
		}
	}
}

func (cfs *CFS) rcvMutations(m lib.Event) {
	if m.Type() != lib.Event_STATE_MUTATION {
		cfs.api.Log(lib.LLERROR, "got unexpected non-mutation event")
	}
	me := m.Data().(*core.MutationEvent)
	switch me.Type {
	case core.MutationEvent_MUTATE:

		switch me.Mutation[1] {
		case "CPU_TEMP_NONEtoCPU_TEMP_UNKNOWN":
			url := lib.NodeURLJoin(me.NodeCfg.ID().String(), HostThermalStateURL)
			ev := core.NewEvent(
				lib.Event_DISCOVERY,
				url,
				&core.DiscoveryEvent{

					URL:     url,
					ValueID: hostthpb.HostThermal_CPU_TEMP_UNKNOWN.String(),
				},
			)
			cfs.dchan <- ev

		case "CPU_TEMP_UNKNOWNtoCPU_TEMP_NORMAL":
			fallthrough
		case "CPU_TEMP_CRITICALtoCPU_TEMP_NORMAL":
			fallthrough
		case "CPU_TEMP_HIGHtoCPU_TEMP_NORMAL":
			aggURL, e := me.NodeCfg.GetValue(cfs.cfg.GetAggUrl())
			if e != nil {
				cfs.api.Logf(lib.LLERROR, "problem getting agg name for node: %s", e.Error())
			}
			// we store map<aggregatorname>map<nodeid>{<mutation>,<node>}
			nm := NMut{me.NodeCfg, me.Mutation[1]}
			_, found := cfs.queue[aggURL.String()]
			if found == false {
				cfs.queue[aggURL.String()] = make(map[string]NMut)
			}
			cfs.mutex.Lock()
			cfs.queue[aggURL.String()][me.NodeCfg.ID().String()] = nm
			cfs.mutex.Unlock()
		default:
			//REVERSE pp.api.Logf(lib.LLDEBUG, "unexpected event: %s", me.Mutation[1])
		}
	}
}

func (cfs *CFS) mutateCPUTemp() {
	// check whether any mutation in queue
	if len(cfs.queue) != 0 {
		// normal represents mutations cpu_temp_unknownTOcpu_temp_normal
		normal := map[string][]lib.Node{}

		// high represents mutations cpu_temp_normalTOcpu_temp_high
		high := map[string][]lib.Node{}

		// normal represents mutations cpu_temp_highTOcpu_temp_critical
		critical := map[string][]lib.Node{}

		cfs.mutex.Lock()
		for agg, nms := range cfs.queue {
			for _, m := range nms {
				switch m.Mutation {
				case "CPU_TEMP_UNKNOWNtoCPU_TEMP_NORMAL":
					normal[agg] = append(normal[agg], m.Node)
					break
				case "CPU_TEMP_HIGHtoCPU_TEMP_NORMAL":
					high[agg] = append(high[agg], m.Node)
					break
				case "CPU_TEMP_CRITICALtoCPU_TEMP_NORMAL":
					critical[agg] = append(critical[agg], m.Node)
					break
				}
			}

		}

		// re-initialize the queue to acquire fresh mutations
		cfs.queue = make(map[string]map[string]NMut)
		cfs.mutex.Unlock()

		// After boot CPU frequency policy
		bPolicy := "schedutil"

		// high CPU temperature frequency policy
		hPolicy := "powersave"

		// for the time being, critical CPU temperature frequency policy is set to "powersave"
		cPolicy := "powersave"

		for ag := range normal {
			cfs.aggregateHandler(ag, normal[ag], "CPU_TEMP_UNKNOWNtoCPU_TEMP_NORMAL", bPolicy)
		}

		for ag := range high {
			cfs.aggregateHandler(ag, high[ag], "CPU_TEMP_HIGHtoCPU_TEMP_NORMAL", hPolicy)
		}

		for ag := range critical {
			cfs.aggregateHandler(ag, critical[ag], "CPU_TEMP_CRITICALtoCPU_TEMP_NORMAL", cPolicy)
		}

	}
}

// aggregateHandler makes calls to aggregator for the given nodes with related mutation and frequecy scaling policy
func (cfs *CFS) aggregateHandler(aggregatorName string, nodeList []lib.Node, mutation string, freqScalPolicy string) {

	switch mutation {
	case "CPU_TEMP_UNKNOWNtoCPU_TEMP_NORMAL":
		cfs.CPUFrequencyScaling(aggregatorName, nodeList, freqScalPolicy)
		break
	case "CPU_TEMP_CRITICALtoCPU_TEMP_NORMAL":
		cfs.CPUFrequencyScaling(aggregatorName, nodeList, freqScalPolicy)
		break
	case "CPU_TEMP_HIGHtoCPU_TEMP_NORMAL":
		cfs.CPUFrequencyScaling(aggregatorName, nodeList, freqScalPolicy)
		break
	}

}

// CPUFrequencyScaling scales CPU frequency according to given parameters
func (cfs *CFS) CPUFrequencyScaling(aggregatorName string, nodeList []lib.Node, freqScalPolicy string) {

	freqScalPolicies := cfs.cfg.GetFreqScalPolicies()

	scalingGovernor := freqScalPolicies[freqScalPolicy].GetScalingGovernor()
	scalingMinFreq := freqScalPolicies[freqScalPolicy].GetScalingMinFreq()
	scalingMaxFreq := freqScalPolicies[freqScalPolicy].GetScalingMaxFreq()

	idMap := make(map[string]lib.NodeID)
	var ipList []string
	for _, n := range nodeList {
		v, _ := n.GetValue(cfs.cfg.GetIpUrl())
		ip := IPv4.BytesToIP(v.Bytes()).String()

		ipList = append(ipList, ip)
		idMap[ip] = n.ID()
	}
	srvs := cfs.cfg.GetServers()
	srvIP := srvs[aggregatorName].GetIp()
	srvPort := srvs[aggregatorName].GetPort()
	srvName := srvs[aggregatorName].GetName()
	reqTimeout := srvs[aggregatorName].GetReqTimeout()
	path := fmt.Sprintf("redfish/v1/AggregationService/%v/ScaleCPUFreq", srvName)

	aggregatorURL := &url.URL{
		Scheme: "http",
		User:   url.UserPassword("", ""),
		Host:   net.JoinHostPort(srvIP, srvPort),
		Path:   path,
	}

	respc, errc := make(chan CPUPerfScalingRespColl), make(chan error)

	// Make http rest calls to aggregator in go routine
	go func(aggregator string, IPList []string, cpuScalingGovernor string, cpuScalingMinFreq int32, cpuScalingMaxFreq int32, nodeReqTimeout int32) {
		resp, err := cfs.aggregateCPUPerformScaling(aggregator, IPList, cpuScalingGovernor, cpuScalingMinFreq, cpuScalingMaxFreq, nodeReqTimeout)
		if err != nil {
			errc <- err
			return
		}
		respc <- resp
	}(aggregatorURL.String(), ipList, scalingGovernor, scalingMinFreq, scalingMaxFreq, reqTimeout)

	var rs CPUPerfScalingRespColl
	var reqError string
	// define label "L" break to break from external forever "for" loop
L:
	for {
		select {
		case res := <-respc:
			rs = res
			break L
		case e := <-errc:
			reqError = e.Error()
			cfs.api.Logf(lib.LLERROR, "Request to RFAggregator failed: %v", reqError)
			break L
		}
	}

	for _, r := range rs.CPUPerfScalingRespCollection {
		fmt.Println("Host:", r.HostAddress, "Scaling Governor:", r.CurScalingGovernor, "Scaling Current Freq:", r.ScalingCurFreq, "CPU Current Freq:", r.CPUCurFreq, "Scaling Min Freq:", r.ScalingMinFreq, "Scaling Max Freq:", r.ScalingMaxFreq, "CPU Min Freq:", r.CPUMinFreq, "CPU Max Freq:", r.CPUMaxFreq)
		fmt.Println("")
	}
	fmt.Println("Total Responses from Noedes via Aggregator: ", len(rs.CPUPerfScalingRespCollection))
}

func (cfs *CFS) aggregateCPUPerformScaling(aggregatorAddress string, ns []string, scalingGovernor string, scalingMinFreq int32, scalingMaxFreq int32, reqTimeout int32) (CPUPerfScalingRespColl, error) {

	// to recieve aggregated response from RFAggregator
	var rs CPUPerfScalingRespColl

	// build payload for aggregated http request to RFAggregator
	reqPayLoad, e := json.Marshal(CPUPerfScalingReq{
		ScalingGovernor:  scalingGovernor,
		ScalingMinFreq:   scalingMinFreq,
		ScalingMaxFreq:   scalingMaxFreq,
		NodesAddressList: ns,
		Timeout:          reqTimeout,
	})

	if e != nil {
		cfs.api.Logf(lib.LLERROR, "CPU frequency scaling object marshalling failed: %v", e)
		return rs, e
	}

	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, aggregatorAddress, bytes.NewBuffer(reqPayLoad))
	if err != nil {
		cfs.api.Logf(lib.LLERROR, "http PUT API request failed: %v", err)
		return rs, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		cfs.api.Logf(lib.LLERROR, "http PUT API call failed: %v", err)
		return rs, err
	}

	defer resp.Body.Close()
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		cfs.api.Logf(lib.LLERROR, "http PUT response failed to read body: %v", e)
		return rs, e
	}

	e = json.Unmarshal(body, &rs)
	if e != nil {
		cfs.api.Logf(lib.LLERROR, "got invalid JSON response: %v", e)
		return rs, e
	}
	return rs, nil
}
