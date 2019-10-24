/* pxe.go: provides generic PXE/iPXE-boot capabilities
 *           this manages both DHCP and TFTP/HTTP services.
 *			 If <file> doesn't exist, but <file>.tpl does, tftp will fill it as as template.
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
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
	thpb "github.com/hpc/kraken/extensions/Thermal/proto"
	"github.com/hpc/kraken/lib"

	"github.com/hpc/kraken/extensions/IPv4"
	//pb "github.com/hpc/kraken/modules/test/proto"
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
var okNodes = 0
var highNodes = 0
var critNodes = 0

// PayLoad struct for collection of nodes
type PayLoad struct {
	NodesAddressList []string `json:"nodesaddresslist"`
	Timeout          int      `json:"timeout"`
}
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
	api   lib.APIClient
	cfg   *pb.RFDiscoveryConfig
	dchan chan<- lib.Event

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
func (t *RFD) UpdateConfig(cfg proto.Message) (e error) {
	if tcfg, ok := cfg.(*pb.RFDiscoveryConfig); ok {
		t.cfg = tcfg
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
func (t *RFD) SetDiscoveryChan(c chan<- lib.Event) { t.dchan = c }

// Entry is the module's executable entrypoint
func (t *RFD) Entry() {
	url := lib.NodeURLJoin(t.api.Self().String(), ModuleStateURL)
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  t.Name(),
			URL:     url,
			ValueID: "RUN",
		},
	)
	t.dchan <- ev

	// setup a ticker for polling discovery
	dur, _ := time.ParseDuration(t.cfg.GetPollingInterval())
	t.api.Logf(lib.LLERROR, "********* P O L L I N G  I N T E R V A L: %v", dur)
	//dur, _ := time.ParseDuration("10s")
	t.pollTicker = time.NewTicker(dur)

	// main loop
	for {
		select {
		case <-t.pollTicker.C:
			go t.discoverAll()
			break
		}
	}
}

// discoverAll is used to do polling discovery of power state
// Note: this is probably not extremely efficient for large systems
func (t *RFD) discoverAll() {
	t.api.Log(lib.LLDEBUG, "polling for node state")

	ns, e := t.api.QueryReadAll()
	if e != nil {
		t.api.Logf(lib.LLERROR, "polling node query failed: %v", e)
		return
	}
	bySrv := make(map[string][]lib.Node)

	// get ip addresses for nodes
	for _, n := range ns {
		v, e := n.GetValue(t.cfg.GetAggUrl())
		if e != nil {
			t.api.Logf(lib.LLERROR, "problem getting agg name for nodes")
		}
		// ip := IPv4.BytesToIP(vs[t.cfg.GetIpUrl()].Bytes())
		aggName := v.String()
		// ipmap[ip.String()] = n
		// idmap[name] = n.ID()
		if aggName != "" {
			bySrv[aggName] = append(bySrv[aggName], n)
		}
	}

	for aggName, nodes := range bySrv {
		t.tempDiscover(aggName, nodes)
	}
	// t.fakeDiscover(aggName, bySrv[aggName])
	// for _, n := range ns {
	// 	t.fakeDiscover(n)
	// }
}

func (t *RFD) tempDiscover(aggregatorName string, nodeList []lib.Node) {

	idMap := make(map[string]lib.NodeID)
	var ipList []string
	for _, n := range nodeList {
		v, _ := n.GetValue(t.cfg.GetIpUrl())
		ip := IPv4.BytesToIP(v.Bytes()).String()
		ipList = append(ipList, ip)
		idMap[ip] = n.ID()
	}
	t.api.Logf(lib.LLDEBUG, "got ip addresses: %v", ipList)

	srvs := t.cfg.GetServers()
	t.api.Logf(lib.LLDEBUG, "*****AGGREGATOR servers: %+v", srvs)

	srvIP := srvs[aggregatorName].GetIp()
	srvPort := srvs[aggregatorName].GetPort()
	t.api.Logf(lib.LLDEBUG, "*****AGGREGATOR IP: %v, PORT: %v", srvIP)

	// ************** call to aggregator start

	c := "4"
	//srvIP := t.cfg.Servers[srvName].GetIp()
	//srvPort := t.cfg.Servers[srvName].GetPort()
	aggregatorURL := fmt.Sprintf("http://%v:%v/redfish/v1/AggregationService/Chassis/%v/Thermal", srvIP, srvPort, c)
	rs, _ := aggregateCPUTemp(aggregatorURL, ipList)

	//var vid thpb.Thermal_CPU_TEMP_STATE

	for _, r := range rs.CPUTempList {
		cpuTemp, vid, ip := lambdaStateDiscovery(r)
		fmt.Printf("\n NodeAddress: %s CPU Temperature: %dC and Temperature State: %s\n", ip, cpuTemp, vid)
		//vid := thpb.Thermal_CPU_TEMP_STATE[cpuTempState]
		// ************** call to aggregator end

		//Thermal_CPU_TEMP_HIGH

		//for _, n := range nodeList {
		url := lib.NodeURLJoin(idMap[ip].String(), ThermalStateURL)
		v := core.NewEvent(
			lib.Event_DISCOVERY,
			url,
			&core.DiscoveryEvent{
				Module: t.Name(),
				URL:    url,
				//ValueID: vid.String(),
				ValueID: vid,
			},
		)
		t.dchan <- v
	}

}

// Init is used to intialize an executable module prior to entrypoint
func (t *RFD) Init(api lib.APIClient) {
	t.api = api
	t.cfg = t.NewConfig().(*pb.RFDiscoveryConfig)
}

// Stop should perform a graceful exit
func (t *RFD) Stop() {
	os.Exit(0)
}

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

func lambdaStateDiscovery(v nodeCPUTemp) (int, string, string) {
	cpuTemp := v.CPUTemp
	cpuTempState := thpb.Thermal_CPU_TEMP_NONE

	if cpuTemp <= 3000 || cpuTemp >= 70000 {
		//cpuTempState = "CPU_TEMP_CRITICAL"
		cpuTempState = thpb.Thermal_CPU_TEMP_CRITICAL
		critNodes++
	} else if cpuTemp >= 60000 && cpuTemp < 70000 {
		//cpuTempState = "CPU_TEMP_HIGH"
		cpuTempState = thpb.Thermal_CPU_TEMP_HIGH
		highNodes++
	} else if cpuTemp > 3000 && cpuTemp < 60000 {
		//cpuTempState = "CPU_TEMP_NORMAL"
		cpuTempState = thpb.Thermal_CPU_TEMP_NORMAL
		okNodes++
	}
	return cpuTemp, cpuTempState.String(), v.HostAddress

}

func aggregateCPUTemp(aggregatorAddress string, ns []string) (CPUTempCollection, error) {
	var rs CPUTempCollection
	payLoad, e := json.Marshal(PayLoad{
		NodesAddressList: ns,
		Timeout:          nodeReqTimeout,
	})

	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, aggregatorAddress, bytes.NewBuffer(payLoad))
	if err != nil {
		//pp.api.Logf(lib.LLERROR, "http PUT API request failed: %v", err)
		fmt.Println(err)
		return rs, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		//pp.api.Logf(lib.LLERROR, "http PUT API call failed: %v", err)
		fmt.Println(err)
		return rs, err
	}

	defer resp.Body.Close()
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		//pp.api.Logf(lib.LLERROR, "http PUT response failed to read body: %v", e)
		fmt.Println(e)
		return rs, e
	}

	e = json.Unmarshal(body, &rs)
	if e != nil {
		//pp.api.Logf(lib.LLERROR, "got invalid JSON response: %v", e)
		fmt.Println(e)
		return rs, e
	}
	fmt.Println("Total responses from Pi nodes:", len(rs.CPUTempList))
	return rs, nil
}
