/* RFDiscovery.go: performs monitoring of HPC nodes via Redfish API using the RFAggregator (REST API server).
 *
 * Author: Ghazanfar Ali, ghazanfar.ali@ttu.edu; J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/RFDiscovery.proto

/*
 * This module will manipulate the PhysState state field.
 * It will be restricted to Platform = vbox.
 */

package rfdiscovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/rfdiscovery/proto"
)

const (
	ThermalStateURL = "type.googleapis.com/proto.Thermal/State"

	VBMBase string = "/vboxmanage"
	VBMStat string = VBMBase + "/showvminfo"
	// VBMOn          string = VBMBase + "/startvm"
	// VBMOff         string = VBMBase + "/controlvm"
	PlatformString string = "rpi3"
)

// HTTP Request time out in milliseconds
var nodeReqTimeout = 250
var okNodes = 0
var highNodes = 0
var critNodes = 0

// payload struct for collection of nodes
type PayLoad struct {
	NodesAddressList []string `json:"nodesaddresslist"`
	Timeout          int      `json:"timeout"`
}
type nodeCPUTemp struct {
	TimeStamp   time.Time
	HostAddress string
	CPUTemp     int
}

type CPUTempCollection struct {
	CPUTempList []nodeCPUTemp `json:"cputemplist"`
}

// modify these if you want different requires for mutations
var reqs = map[string]reflect.Value{
	"/Platform": reflect.ValueOf(PlatformString),
}

// modify this if you want excludes
var excs = map[string]reflect.Value{}

////////////////////
// VBM Object /
//////////////////

// VBM provides a power on/off interface to the vboxmanage-rest-api interface
type RFD struct {
	api lib.APIClient
	cfg *pb.RFDiscoveryConfig
	//mchan      <-chan lib.Event
	dchan      chan<- lib.Event
	pollTicker *time.Ticker
}

/*
 *lib.Module
 */
var _ lib.Module = (*RFD)(nil)

// Name returns the FQDN of the module
func (*RFD) Name() string { return "github.com/hpc/kraken/modules/rfdiscovery" }

/*
 * lib.ModuleWithConfig
 */
var _ lib.ModuleWithConfig = (*RFD)(nil)

// NewConfig returns a fully initialized default config
func (*RFD) NewConfig() proto.Message {
	r := &pb.RFDiscoveryConfig{
		ServerUrl: "type.googleapis.com/proto.RFAggregator/ApiServer",
		NameUrl:   "type.googleapis.com/proto.RFAggregator/Name",
		UuidUrl:   "type.googleapis.com/proto.RFAggregator/Uuid",
		IpUrl:     "type.googleapis.com/proto.IPv4OverEthernet/Ifaces/0/Ip/Ip",
		Servers: map[string]*pb.RFAggregator{
			"rfa": {
				Name: "rfa",
				Ip:   "localhost",
				Port: 8002,
			},
		},
		PollingInterval: "30s",
	}
	return r
}

// UpdateConfig updates the running config
func (rfd *RFD) UpdateConfig(cfg proto.Message) (e error) {
	if rfdcfg, ok := cfg.(*pb.RFDiscoveryConfig); ok {
		rfd.cfg = rfdcfg
		if rfd.pollTicker != nil {
			rfd.pollTicker.Stop()
			dur, _ := time.ParseDuration(rfd.cfg.GetPollingInterval())
			rfd.pollTicker = time.NewTicker(dur)
		}
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

/*
 * lib.ModuleWithDiscovery
 */
var _ lib.ModuleWithDiscovery = (*RFD)(nil)

// SetDiscoveryChan sets the current discovery channel
// this is generally done by the API
func (rfd *RFD) SetDiscoveryChan(c chan<- lib.Event) { rfd.dchan = c }

/*
 * lib.ModuleSelfService
 */
var _ lib.ModuleSelfService = (*RFD)(nil)

// Entry is the module's executable entrypoint
func (rfd *RFD) Entry() {
	url := lib.NodeURLJoin(rfd.api.Self().String(), lib.URLPush(lib.URLPush("/Services", "rfdiscovery"), "State"))
	rfd.dchan <- core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  rfd.Name(),
			URL:     url,
			ValueID: "RUN",
		},
	)
	// setup a ticker for polling discovery
	dur, _ := time.ParseDuration(rfd.cfg.GetPollingInterval())
	rfd.pollTicker = time.NewTicker(dur)

	// main loop
	for {

		select {
		case <-rfd.pollTicker.C:
			go rfd.discoverAll()
			break
			// case m := <-pp.mchan: // mutation request
			// 	go pp.handleMutation(m)
			// 	break
		}
	}
}

// Init is used to intialize an executable module prior to entrypoint
func (rfd *RFD) Init(api lib.APIClient) {
	rfd.api = api
	rfd.cfg = rfd.NewConfig().(*pb.RFDiscoveryConfig)
}

// Stop should perform a graceful exit
func (rf *RFD) Stop() {
	os.Exit(0)
}

////////////////////////
// Unexported methods /
//////////////////////

func (rfd *RFD) vmDiscover(srvName, name string, id lib.NodeID) {
	srv, ok := rfd.cfg.Servers[srvName]
	if !ok {
		rfd.api.Logf(lib.LLERROR, "cannot control power for unknown API server: %s", srvName)
		return
	}
	addr := srv.Ip + ":" + strconv.Itoa(int(srv.Port))

	url := "http://" + addr + VBMStat + "/" + name
	resp, e := http.Get(url)
	if e != nil {
		rfd.api.Logf(lib.LLERROR, "error dialing api: %v", e)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		rfd.api.Logf(lib.LLERROR, "error dialing api: HTTP %v", resp.StatusCode)
		return
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		rfd.api.Logf(lib.LLERROR, "error reading api response body: %v", e)
		return
	}
	var rs struct {
		Name  string
		Uuid  string
		Ram   string
		Vram  string
		State string
	}
	e = json.Unmarshal(body, &rs)
	if e != nil {
		rfd.api.Logf(lib.LLERROR, "error unmarshaling json: %v", e)
		return
	}
	var vid string
	switch rs.State {
	case "paused":
		fallthrough
	case "powered off":
		vid = "POWER_OFF"
	case "running":
		vid = "POWER_ON"
	default:
		vid = "PHYS_UNKNOWN"
	}

	url = lib.NodeURLJoin(id.String(), ThermalStateURL)
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

// discoverAll is used to do polling discovery of CPU temperature
// Note: this is probably not extremely efficient for large systems
func (rfd *RFD) discoverAll() {
	rfd.api.Log(lib.LLDEBUG, "polling for node CPU temperature")
	ns, e := rfd.api.QueryReadAll()
	if e != nil {
		rfd.api.Logf(lib.LLERROR, "polling CPU temperature query failed: %v", e)
		return
	}
	idmap := make(map[string]lib.NodeID)
	bySrv := make(map[string][]string)

	// build lists
	for _, n := range ns {
		vs := n.GetValues([]string{"/Platform", rfd.cfg.GetNameUrl(), rfd.cfg.GetServerUrl(), rfd.cfg.GetIpUrl()})
		if len(vs) != 3 {
			rfd.api.Logf(lib.LLDEBUG, "skipping node %s, doesn't have complete Aggregator info", n.ID().String())
			continue
		}
		if vs["/Platform"].String() != PlatformString { // Note: this may need to be more flexible in the future
			continue
		}
		//name := vs[rfd.cfg.GetNameUrl()].String()
		srv := vs[rfd.cfg.GetServerUrl()].String()
		ip := vs[rfd.cfg.GetIpUrl()].String()
		//idmap[name] = n.ID()
		idmap[ip] = n.ID()
		bySrv[srv] = append(bySrv[srv], ip)
	}

	// This is not very efficient, but we assume that this module won't be used for huge amounts of vms
	for _, ns := range bySrv {
		// rfd.discoverCPUTemp(s, ns, idmap)
		rfd.api.Logf(lib.LLDEBUG, "LIST OF IPs: %v", ns)
		// for _, n := range ns {
		// 	rfd.vmDiscover(s, n, idmap[n])
		// }
	}
}

// initialization
func init() {
	module := &RFD{}
	//mutations := make(map[string]lib.StateMutation)
	discovers := make(map[string]map[string]reflect.Value)
	drstate := make(map[string]reflect.Value)

	// for m := range muts {
	// 	dur, _ := time.ParseDuration(muts[m].timeout)
	// 	mutations[m] = core.NewStateMutation(
	// 		map[string][2]reflect.Value{
	// 			"/PhysState": {
	// 				reflect.ValueOf(muts[m].f),
	// 				reflect.ValueOf(muts[m].t),
	// 			},
	// 		},
	// 		reqs,
	// 		excs,
	// 		lib.StateMutationContext_CHILD,
	// 		dur,
	// 		[3]string{module.Name(), "/PhysState", "PHYS_HANG"},
	// 	)
	// 	drstate[cpb.Node_PhysState_name[int32(muts[m].t)]] = reflect.ValueOf(muts[m].t)
	// }
	discovers["/PhysState"] = drstate
	discovers["/PhysState"]["PHYS_UNKNOWN"] = reflect.ValueOf(cpb.Node_PHYS_UNKNOWN)
	discovers["/RunState"] = map[string]reflect.Value{
		"RUN_UK": reflect.ValueOf(cpb.Node_UNKNOWN),
	}
	discovers["/Services/vboxmanage/State"] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}
	si := core.NewServiceInstance("vboxmanage", module.Name(), module.Entry, nil)

	// Register it all
	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(module, discovers)
	//core.Registry.RegisterMutations(module, mutations)
}

func lambdaStateDiscovery(v nodeCPUTemp) (int, string, string) {
	cpu_temp := v.CPUTemp
	cpu_temp_state := "CPU_TEMP_NONE"

	if cpu_temp <= 3000 || cpu_temp >= 70000 {
		cpu_temp_state = "CPU_TEMP_CRITICAL"
		critNodes++
	} else if cpu_temp >= 60000 && cpu_temp < 70000 {
		cpu_temp_state = "CPU_TEMP_HIGH"
		highNodes++
	} else if cpu_temp > 3000 && cpu_temp < 60000 {
		cpu_temp_state = "CPU_TEMP_NORMAL"
		okNodes++
	}
	return cpu_temp, cpu_temp_state, v.HostAddress

}
func getNodesAddress(c string) map[string][]string {
	ns := map[string][]string{}

	for i := 0; i < 150; i++ {
		n := "10.15.244." + strconv.Itoa(i)
		ns[c] = append(ns[c], n)
	}
	return ns
}
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

	/*
		     hostname, err := os.Hostname()
		     if err != nil {
		     	hostname = "nil"
			log.Fatalf("could not obtain hostname: %v", err)
		     }
		     return hostname
	*/
}

func aggregateCPUTemp(aggregatorAddress string, ns []string) {

	payLoad, e := json.Marshal(PayLoad{
		NodesAddressList: ns,
		Timeout:          nodeReqTimeout,
	})

	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, aggregatorAddress, bytes.NewBuffer(payLoad))
	if err != nil {
		//pp.api.Logf(lib.LLERROR, "http PUT API request failed: %v", err)
		fmt.Println(err)
		return
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		//pp.api.Logf(lib.LLERROR, "http PUT API call failed: %v", err)
		fmt.Println(err)
		return
	}

	defer resp.Body.Close()
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		//pp.api.Logf(lib.LLERROR, "http PUT response failed to read body: %v", e)
		fmt.Println(e)
		return
	}
	var rs CPUTempCollection
	e = json.Unmarshal(body, &rs)
	if e != nil {
		//pp.api.Logf(lib.LLERROR, "got invalid JSON response: %v", e)
		fmt.Println(e)
		return
	}
	for _, r := range rs.CPUTempList {
		cpuTemp, cpuTempState, node := lambdaStateDiscovery(r)
		fmt.Printf("\n NodeAddress: %s CPU Temperature: %dC and Temperature State: %s\n", node, cpuTemp, cpuTempState)
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
		// pp.dchan <- v
	}
	fmt.Println("Total responses from Pi nodes:", len(rs.CPUTempList))
	fmt.Println("Total OK Temp. Nodes: ", okNodes, ", Total High Temp. Nodes: ", highNodes, ", Total Critical Temp. Nodes: ", critNodes)
}

func (rfd *RFD) discoverCPUTemp(srvName string, ns []string, idmap map[string]lib.NodeID) {

	// generate node list for testing
	c := "4"
	//ns := make([]string, 0)
	// ns := map[string][]string{}

	// for i := 0; i < 150; i++ {
	// 	n := "10.15.244." + strconv.Itoa(i)
	// 	ns[c] = append(ns[c], n)
	// }
	// ns := getNodesAddress(c)
	// ip := GetNodeIPAddress()
	// aggregators := []string{ip + ":8002"}
	//var wg sync.WaitGroup
	//wg.Add(len(aggregators))l
	srvIp := rfd.cfg.Servers[srvName].GetIp()
	srvPort := rfd.cfg.Servers[srvName].GetPort()
	// aggregatorURL := "http://" + srvIp + "/redfish/v1/AggregationService/Chassis/" + c + "/Thermal"
	aggregatorURL := fmt.Sprintf("http://%v:%v/redfish/v1/AggregationService/Chassis/%v/Thermal", srvIp, srvPort, c)

	aggregateCPUTemp(aggregatorURL, ns)

	// for _, aggregator := range aggregators {
	// 	//go func() {
	// 	//	defer wg.Done()
	// 	// URL construction: chassis ip, port, identity
	// 	// change hard coded "ip" with "srv.Ip" and "port" with strconv.Itoa(int(srv.Port))
	// 	aggregatorURL := "http://" + aggregator + "/redfish/v1/AggregationService/Chassis/" + c + "/Thermal"
	// 	aggregateCPUTemp(aggregatorURL, ns)
	// 	//}()
	// }
	//wg.Wait()

}

// func main() {

// 	discoverCPUTemp()

// }
