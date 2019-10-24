/* RFDiscovery.go: performs monitoring of HPC nodes via Redfish API using the RFAggregator (REST API server).
 *
 * Author: Ghazanfar Ali, ghazanfar.ali@ttu.edu; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/pxe.proto

package rfdiscovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/IPv4"
	thpb "github.com/hpc/kraken/extensions/Thermal/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/rfdiscovery/proto"
)

const (
	// ThermalStateURL points to Thermal extension
	ThermalStateURL = "type.googleapis.com/proto.Thermal/State"
	// ModuleStateURL refers to module state
	ModuleStateURL = "/Services/rfdiscovery/State"
)

var _ lib.Module = (*RFD)(nil)
var _ lib.ModuleWithConfig = (*RFD)(nil)
var _ lib.ModuleWithDiscovery = (*RFD)(nil)
var _ lib.ModuleSelfService = (*RFD)(nil)

// HTTP Request time out in milliseconds
var nodeReqTimeout = 250

// PayLoad struct for collection of nodes
type PayLoad struct {
	NodesAddressList []string `json:"nodesaddresslist"`
	Timeout          int      `json:"timeout"`
}

// nodeCPUTemp is structure for node temp
type nodeCPUTemp struct {
	TimeStamp   time.Time
	HostAddress string
	CPUTemp     int
}

//CPUTempCollection is array of CPU Temperature responses
type CPUTempCollection struct {
	CPUTempList []nodeCPUTemp `json:"cputemplist"`
}

// RFD provides rfdiscovery module capabilities
type RFD struct {
	api        lib.APIClient
	cfg        *pb.RFDiscoveryConfig
	dchan      chan<- lib.Event
	pollTicker *time.Ticker
}

// Name returns the FQDN of the module
func (*RFD) Name() string { return "github.com/hpc/kraken/modules/rfdiscovery" }

// NewConfig returns a fully initialized default config
func (*RFD) NewConfig() proto.Message {
	r := &pb.RFDiscoveryConfig{
		IpUrl:  "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		AggUrl: "type.googleapis.com/proto.RFAggregatorServer/ApiServer",
		Servers: map[string]*pb.RFDiscoveryServer{
			"rfdiscoveryServer": {
				Name: "rfdiscoveryServer",
				Ip:   "localhost",
				Port: 8269,
			},
		},
		PollingInterval: "10s",
	}
	return r
}

// UpdateConfig updates the running config
func (rfd *RFD) UpdateConfig(cfg proto.Message) (e error) {
	if rcfg, ok := cfg.(*pb.RFDiscoveryConfig); ok {
		rfd.cfg = rcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*RFD) ConfigURL() string {
	cfg := &pb.RFDiscoveryConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (rfd *RFD) SetDiscoveryChan(c chan<- lib.Event) { rfd.dchan = c }

func init() {
	module := &RFD{}
	discovers := make(map[string]map[string]reflect.Value)
	dtest := make(map[string]reflect.Value)

	dtest[thpb.Thermal_CPU_TEMP_NONE.String()] = reflect.ValueOf(thpb.Thermal_CPU_TEMP_NONE)
	dtest[thpb.Thermal_CPU_TEMP_NORMAL.String()] = reflect.ValueOf(thpb.Thermal_CPU_TEMP_NORMAL)
	dtest[thpb.Thermal_CPU_TEMP_HIGH.String()] = reflect.ValueOf(thpb.Thermal_CPU_TEMP_HIGH)
	dtest[thpb.Thermal_CPU_TEMP_CRITICAL.String()] = reflect.ValueOf(thpb.Thermal_CPU_TEMP_CRITICAL)

	discovers[ThermalStateURL] = dtest

	discovers[ModuleStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	si := core.NewServiceInstance("rfdiscovery", module.Name(), module.Entry, nil)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(module, discovers)
}

// Init is used to intialize an executable module prior to entrypoint
func (rfd *RFD) Init(api lib.APIClient) {
	rfd.api = api
	rfd.cfg = rfd.NewConfig().(*pb.RFDiscoveryConfig)
}

// Stop should perform a graceful exit
func (rfd *RFD) Stop() {
	os.Exit(0)
}

// Entry is the module's executable entrypoint
func (rfd *RFD) Entry() {
	url := lib.NodeURLJoin(rfd.api.Self().String(), ModuleStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  rfd.Name(),
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
		rfd.api.Logf(lib.LLERROR, "polling node query failed: %v", e)
		return
	}
	bySrv := make(map[string][]lib.Node)

	// get ip addresses for nodes
	for _, n := range ns {
		v, e := n.GetValue(rfd.cfg.GetAggUrl())
		if e != nil {
			rfd.api.Logf(lib.LLERROR, "problem getting agg name for nodes")
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
func (rfd *RFD) aggCPUTempDiscover(aggregatorName string, nodeList []lib.Node) {

	idMap := make(map[string]lib.NodeID)
	var ipList []string
	for _, n := range nodeList {
		v, _ := n.GetValue(rfd.cfg.GetIpUrl())
		ip := IPv4.BytesToIP(v.Bytes()).String()
		ipList = append(ipList, ip)
		idMap[ip] = n.ID()
	}
	srvs := rfd.cfg.GetServers()
	srvIP := srvs[aggregatorName].GetIp()
	srvPort := srvs[aggregatorName].GetPort()
	srvName := srvs[aggregatorName].GetName()

	aggregatorURL := fmt.Sprintf("http://%v:%v/redfish/v1/AggregationService/Chassis/%v/Thermal", srvIP, srvPort, srvName)
	rs := rfd.aggregateCPUTemp(aggregatorURL, ipList)

	for _, r := range rs.CPUTempList {
		vid, ip := lambdaStateDiscovery(r)
		url := lib.NodeURLJoin(idMap[ip].String(), ThermalStateURL)
		v := core.NewEvent(
			lib.Event_DISCOVERY,
			url,
			&core.DiscoveryEvent{
				Module:  rfd.Name(),
				URL:     url,
				ValueID: vid,
			},
		)
		rfd.dchan <- v
	}

}

// REST API call to relevant aggregator
func (rfd *RFD) aggregateCPUTemp(aggregatorAddress string, ns []string) CPUTempCollection {

	var rs CPUTempCollection

	payLoad, e := json.Marshal(PayLoad{
		NodesAddressList: ns,
		Timeout:          nodeReqTimeout,
	})
	if e != nil {
		rfd.api.Logf(lib.LLERROR, "http PUT API request failed: %v", e)
		return rs
	}

	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, aggregatorAddress, bytes.NewBuffer(payLoad))
	if err != nil {
		rfd.api.Logf(lib.LLERROR, "http PUT API request failed: %v", err)
		return rs
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		rfd.api.Logf(lib.LLERROR, "http PUT API call failed: %v", err)
		return rs
	}

	defer resp.Body.Close()
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		rfd.api.Logf(lib.LLERROR, "http PUT response failed to read body: %v", e)
		return rs
	}

	e = json.Unmarshal(body, &rs)
	if e != nil {
		rfd.api.Logf(lib.LLERROR, "got invalid JSON response: %v", e)
		return rs
	}
	return rs
}

// Discovers state of the CPU based on CPU temperature thresholds
func lambdaStateDiscovery(v nodeCPUTemp) (string, string) {
	cpuTemp := v.CPUTemp
	cpuTempState := thpb.Thermal_CPU_TEMP_NONE

	if cpuTemp <= 3000 || cpuTemp >= 70000 {
		cpuTempState = thpb.Thermal_CPU_TEMP_CRITICAL
	} else if cpuTemp >= 60000 && cpuTemp < 70000 {
		cpuTempState = thpb.Thermal_CPU_TEMP_HIGH
	} else if cpuTemp > 3000 && cpuTemp < 60000 {
		cpuTempState = thpb.Thermal_CPU_TEMP_NORMAL
	}
	return cpuTempState.String(), v.HostAddress

}
