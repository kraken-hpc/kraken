/* rfthermaldiscovery.go: performs monitoring of HPC nodes via Redfish API using the RFAggregator (REST API server).
 *
 * Author: Ghazanfar Ali, ghazanfar.ali@ttu.edu; Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2019, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. rfthermaldiscovery.proto

package rfthermaldiscovery

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
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/ipv4"
	thpb "github.com/hpc/kraken/extensions/rfthermal"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

const (
	// ThermalStateURL points to Thermal extension
	ThermalStateURL = "type.googleapis.com/RFThermal.Temp/State"
	// ModuleStateURL refers to module state
	ModuleStateURL = "/Services/rfthermaldiscovery/State"
)

var _ types.Module = (*RFD)(nil)
var _ types.ModuleWithConfig = (*RFD)(nil)
var _ types.ModuleWithDiscovery = (*RFD)(nil)
var _ types.ModuleSelfService = (*RFD)(nil)

// PayLoad struct for collection of nodes
type PayLoad struct {
	NodesAddressList []string `json:"nodesaddresslist"`
	Timeout          int32    `json:"timeout"`
}

// nodeCPUTemp is structure for node temp
type nodeCPUTemp struct {
	TimeStamp   time.Time
	HostAddress string
	CPUTemp     int32
}

//CPUTempCollection is array of CPU Temperature responses
type CPUTempCollection struct {
	CPUTempList []nodeCPUTemp `json:"cputemplist"`
}

// RFD provides rfdiscovery module capabilities
type RFD struct {
	api        types.ModuleAPIClient
	cfg        *Config
	dchan      chan<- types.Event
	pollTicker *time.Ticker
}

// Name returns the FQDN of the module
func (*RFD) Name() string { return "github.com/hpc/kraken/modules/rfthermaldiscovery" }

// NewConfig returns a fully initialized default config
func (*RFD) NewConfig() proto.Message {
	r := &Config{
		IpUrl:  "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Ip/Ip",
		AggUrl: "type.googleapis.com/RFAggregator.Server/RfAggregator",
		Servers: map[string]*Server{
			"c4": {
				Name:       "c4",
				Ip:         "10.15.247.200",
				Port:       "8000",
				ReqTimeout: 250,
			},
		},
		PollingInterval: "10s",
		RfThermalThresholds: map[string]*Thresholds{
			"RFCPUThermalThresholds": {
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
func (rfd *RFD) UpdateConfig(cfg proto.Message) (e error) {
	if rcfg, ok := cfg.(*Config); ok {
		rfd.cfg = rcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*RFD) ConfigURL() string {
	cfg := &Config{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (rfd *RFD) SetDiscoveryChan(c chan<- types.Event) { rfd.dchan = c }

func init() {
	module := &RFD{}
	discovers := make(map[string]map[string]reflect.Value)
	dtest := make(map[string]reflect.Value)

	dtest[thpb.Temp_CPU_TEMP_NONE.String()] = reflect.ValueOf(thpb.Temp_CPU_TEMP_NONE)
	dtest[thpb.Temp_CPU_TEMP_NORMAL.String()] = reflect.ValueOf(thpb.Temp_CPU_TEMP_NORMAL)
	dtest[thpb.Temp_CPU_TEMP_HIGH.String()] = reflect.ValueOf(thpb.Temp_CPU_TEMP_HIGH)
	dtest[thpb.Temp_CPU_TEMP_CRITICAL.String()] = reflect.ValueOf(thpb.Temp_CPU_TEMP_CRITICAL)

	discovers[ThermalStateURL] = dtest

	discovers[ModuleStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	si := core.NewServiceInstance("rfthermaldiscovery", module.Name(), module.Entry)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
}

// Init is used to intialize an executable module prior to entrypoint
func (rfd *RFD) Init(api types.ModuleAPIClient) {
	rfd.api = api
	rfd.cfg = rfd.NewConfig().(*Config)
}

// Stop should perform a graceful exit
func (rfd *RFD) Stop() {
	os.Exit(0)
}

// Entry is the module's executable entrypoint
func (rfd *RFD) Entry() {
	url := util.NodeURLJoin(rfd.api.Self().String(), ModuleStateURL)
	ev := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{

			URL:     url,
			ValueID: "RUN",
		},
	)
	rfd.dchan <- ev

	// setup a ticker for polling discovery
	dur, _ := time.ParseDuration(rfd.cfg.GetPollingInterval())
	rfd.pollTicker = time.NewTicker(dur)

	// main loop
	for {
		select {
		case <-rfd.pollTicker.C:
			go rfd.discoverAllCPUTemp()
			break
		}
	}
}

// discoverAll is used to do polling discovery of power state
// Note: this is probably not extremely efficient for large systems
func (rfd *RFD) discoverAllCPUTemp() {

	ns, e := rfd.api.QueryReadAll()
	if e != nil {
		rfd.api.Logf(types.LLERROR, "polling node query failed: %v", e)
		return
	}
	bySrv := make(map[string][]types.Node)

	// get ip addresses for nodes
	for _, n := range ns {
		v, e := n.GetValue(rfd.cfg.GetAggUrl())
		if e != nil {
			rfd.api.Logf(types.LLERROR, "problem getting agg name for nodes")
		}
		aggName := v.String()
		if aggName != "" {
			bySrv[aggName] = append(bySrv[aggName], n)
		}
	}

	for aggName, nodes := range bySrv {
		rfd.aggCPUTempDiscover(aggName, nodes)
	}
}

// Discover CPU temperatures of nodes associated to an aggregator
func (rfd *RFD) aggCPUTempDiscover(aggregatorName string, nodeList []types.Node) {

	idMap := make(map[string]types.NodeID)
	var ipList []string
	for _, n := range nodeList {
		v, _ := n.GetValue(rfd.cfg.GetIpUrl())
		ip := v.Interface().(*ipv4.IP).String()
		ipList = append(ipList, ip)
		idMap[ip] = n.ID()
	}
	srvs := rfd.cfg.GetServers()
	srvIP := srvs[aggregatorName].GetIp()
	srvPort := srvs[aggregatorName].GetPort()
	srvName := srvs[aggregatorName].GetName()

	path := fmt.Sprintf("redfish/v1/AggregationService/Chassis/%v/Thermal", srvName)

	aggregatorURL := &url.URL{
		Scheme: "http",
		User:   url.UserPassword("", ""),
		Host:   net.JoinHostPort(srvIP, srvPort),
		Path:   path,
	}

	respc, errc := make(chan CPUTempCollection), make(chan error)

	// Make http rest calls to aggregator in go routine
	go func(aggregator string, IPList []string) {
		resp, err := rfd.aggregateCPUTemp(aggregator, IPList)
		if err != nil {
			errc <- err
			return
		}
		respc <- resp
	}(aggregatorURL.String(), ipList)

	var rs CPUTempCollection
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
			rfd.api.Logf(types.LLERROR, "Request to RFAggregator failed: %v", reqError)
			break L
		}
	}

	for _, r := range rs.CPUTempList {
		vid, ip := rfd.lambdaStateDiscovery(r)
		url := util.NodeURLJoin(idMap[ip].String(), ThermalStateURL)
		v := core.NewEvent(
			types.Event_DISCOVERY,
			url,
			&core.DiscoveryEvent{

				URL:     url,
				ValueID: vid,
			},
		)
		rfd.dchan <- v
	}

}

// REST API call to relevant aggregator
func (rfd *RFD) aggregateCPUTemp(aggregatorAddress string, ns []string) (CPUTempCollection, error) {

	var rs CPUTempCollection
	nodeReqTimeout := rfd.cfg.GetServers()["c4"].GetReqTimeout()
	payLoad, e := json.Marshal(PayLoad{
		NodesAddressList: ns,
		Timeout:          nodeReqTimeout,
	})
	if e != nil {
		rfd.api.Logf(types.LLERROR, "http PUT API request failed: %v", e)
		return rs, e
	}

	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, aggregatorAddress, bytes.NewBuffer(payLoad))
	if err != nil {
		rfd.api.Logf(types.LLERROR, "http PUT API request failed: %v", err)
		return rs, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		rfd.api.Logf(types.LLERROR, "http PUT API call failed: %v", err)
		return rs, err
	}

	defer resp.Body.Close()
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		rfd.api.Logf(types.LLERROR, "http PUT response failed to read body: %v", e)
		return rs, e
	}

	e = json.Unmarshal(body, &rs)
	if e != nil {
		rfd.api.Logf(types.LLERROR, "got invalid JSON response: %v", e)
		return rs, e
	}
	return rs, nil

}

// Discovers state of the CPU based on CPU temperature thresholds
func (rfd *RFD) lambdaStateDiscovery(v nodeCPUTemp) (string, string) {
	cpuTemp := v.CPUTemp
	cpuTempState := thpb.Temp_CPU_TEMP_NONE

	cpuThermalThresholds := rfd.cfg.GetRfThermalThresholds()
	lowerNormal := cpuThermalThresholds["RFCPUThermalThresholds"].GetLowerNormal()
	upperNormal := cpuThermalThresholds["RFCPUThermalThresholds"].GetUpperNormal()

	lowerHigh := cpuThermalThresholds["RFCPUThermalThresholds"].GetLowerHigh()
	upperHigh := cpuThermalThresholds["RFCPUThermalThresholds"].GetUpperHigh()

	lowerCritical := cpuThermalThresholds["RFCPUThermalThresholds"].GetLowerCritical()
	upperCritical := cpuThermalThresholds["RFCPUThermalThresholds"].GetUpperCritical()

	if cpuTemp <= lowerCritical || cpuTemp >= upperCritical {
		cpuTempState = thpb.Temp_CPU_TEMP_CRITICAL
	} else if cpuTemp >= lowerHigh && cpuTemp < upperHigh {
		cpuTempState = thpb.Temp_CPU_TEMP_HIGH
	} else if cpuTemp > lowerNormal && cpuTemp < upperNormal {
		cpuTempState = thpb.Temp_CPU_TEMP_NORMAL
	}
	return cpuTempState.String(), v.HostAddress

}
