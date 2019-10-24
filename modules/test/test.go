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

package test

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
	pb "github.com/hpc/kraken/modules/test/proto"
	//pb "github.com/hpc/kraken/modules/rfdiscovery/proto"
)

const (
	// ThermalStateURL points to Thermal extension
	ThermalStateURL = "type.googleapis.com/proto.Thermal/State"
	// SrvStateURL refers to module state
	SrvStateURL = "/Services/test/State"
)

var _ lib.Module = (*Test)(nil)
var _ lib.ModuleWithConfig = (*Test)(nil)
var _ lib.ModuleWithDiscovery = (*Test)(nil)
var _ lib.ModuleSelfService = (*Test)(nil)

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

// Test provides Test module capabilities
type Test struct {
	api   lib.APIClient
	cfg   *pb.TestConfig
	dchan chan<- lib.Event

	pollTicker *time.Ticker
}

// Name returns the FQDN of the module
func (*Test) Name() string { return "github.com/hpc/kraken/modules/test" }

// NewConfig returns a fully initialized default config
func (*Test) NewConfig() proto.Message {
	r := &pb.TestConfig{
		IpUrl:  "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		AggUrl: "type.googleapis.com/proto.RFAggregatorServer/ApiServer",
		Servers: map[string]*pb.TestServer{
			"testServer": {
				Name: "testServer",
				Ip:   "localhost",
				Port: 8269,
			},
		},
	}
	return r
}

// UpdateConfig updates the running config
func (t *Test) UpdateConfig(cfg proto.Message) (e error) {
	if tcfg, ok := cfg.(*pb.TestConfig); ok {
		t.cfg = tcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*Test) ConfigURL() string {
	cfg := &pb.TestConfig{}
	any, _ := ptypes.MarshalAny(cfg)
	return any.GetTypeUrl()
}

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (t *Test) SetDiscoveryChan(c chan<- lib.Event) { t.dchan = c }

// Entry is the module's executable entrypoint
func (t *Test) Entry() {
	url := lib.NodeURLJoin(t.api.Self().String(), SrvStateURL)
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
	dur, _ := time.ParseDuration("10s")
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
func (t *Test) discoverAll() {
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
		t.fakeDiscover(aggName, nodes)
	}
	// t.fakeDiscover(aggName, bySrv[aggName])
	// for _, n := range ns {
	// 	t.fakeDiscover(n)
	// }
}

func (t *Test) fakeDiscover(aggregatorName string, nodeList []lib.Node) {

	var idMap map[string]lib.NodeID
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
func (t *Test) Init(api lib.APIClient) {
	t.api = api
	t.cfg = t.NewConfig().(*pb.TestConfig)
}

// Stop should perform a graceful exit
func (t *Test) Stop() {
	os.Exit(0)
}

func init() {
	module := &Test{}
	discovers := make(map[string]map[string]reflect.Value)
	dtest := make(map[string]reflect.Value)

	dtest[thpb.Thermal_CPU_TEMP_NONE.String()] = reflect.ValueOf(thpb.Thermal_CPU_TEMP_NONE)
	dtest[thpb.Thermal_CPU_TEMP_NORMAL.String()] = reflect.ValueOf(thpb.Thermal_CPU_TEMP_NORMAL)
	dtest[thpb.Thermal_CPU_TEMP_HIGH.String()] = reflect.ValueOf(thpb.Thermal_CPU_TEMP_HIGH)
	dtest[thpb.Thermal_CPU_TEMP_CRITICAL.String()] = reflect.ValueOf(thpb.Thermal_CPU_TEMP_CRITICAL)

	discovers[ThermalStateURL] = dtest

	discovers[SrvStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}
	si := core.NewServiceInstance("test", module.Name(), module.Entry, nil)
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

// func getNodesAddress(c string) map[string][]string {
// 	ns := map[string][]string{}

// 	for i := 0; i < 150; i++ {
// 		n := "10.15.244." + strconv.Itoa(i)
// 		ns[c] = append(ns[c], n)
// 	}
// 	return ns
// }

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

	// for _, r := range rs.CPUTempList {
	// 	cpuTemp, cpuTempState, node := lambdaStateDiscovery(r)
	// 	fmt.Printf("\n NodeAddress: %s CPU Temperature: %dC and Temperature State: %s\n", node, cpuTemp, cpuTempState)
	//fmt.Println(r.CPUTemp)
	// url := lib.NodeURLJoin(idmap[c+"n"+r.ID], "/PhysState")
	// vid := "POWER_OFF"
	// if r.State == "on" {
	// 	vid = "POWER_ON"
	// }
	// v := core.NewEvent(
	// 	lib.Event_DISCOVERY,
	// 	url,
	// 	&core.DiscoveryEvent{
	// 		Module:  pp.Name(),
	// 		URL:     url,
	// 		ValueID: vid,
	// 	},
	// )
	// 	// pp.dchan <- v
	// }

}
