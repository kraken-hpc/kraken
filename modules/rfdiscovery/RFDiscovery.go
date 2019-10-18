package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

// HTTP Request time out in milliseconds
var nodeReqTimeout = 250

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

func lambdaStateDiscovery(v nodeCPUTemp) (int, string, string) {
	cpu_temp := v.CPUTemp
	cpu_temp_state := "CPU_TEMP_NONDETERMINISTIC"

	if cpu_temp <= 3000 || cpu_temp >= 70000 {
		cpu_temp_state = "CPU_TEMP_CRITICAL"
	} else if cpu_temp >= 60000 && cpu_temp < 70000 {
		cpu_temp_state = "CPU_TEMP_HIGH"
	} else if cpu_temp > 3000 && cpu_temp < 60000 {
		cpu_temp_state = "CPU_TEMP_OK"
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
	println("\nTotal responses from Pi nodes:", len(rs.CPUTempList), "\n")
}

func discoverCPUTemp() {

	// generate node list for testing
	c := "4"
	//ns := make([]string, 0)
	// ns := map[string][]string{}

	// for i := 0; i < 150; i++ {
	// 	n := "10.15.244." + strconv.Itoa(i)
	// 	ns[c] = append(ns[c], n)
	// }
	ns := getNodesAddress(c)
	ip := GetNodeIPAddress()
	aggregators := []string{ip + ":8002"}
	//var wg sync.WaitGroup
	//wg.Add(len(aggregators))

	for _, aggregator := range aggregators {
		//go func() {
		//	defer wg.Done()
		// URL construction: chassis ip, port, identity
		// change hard coded "ip" with "srv.Ip" and "port" with strconv.Itoa(int(srv.Port))
		aggregatorURL := "http://" + aggregator + "/redfish/v1/AggregationService/Chassis/" + c + "/Thermal"
		aggregateCPUTemp(aggregatorURL, ns[c])
		//}()
	}
	//wg.Wait()

}

func main() {

	discoverCPUTemp()

}
