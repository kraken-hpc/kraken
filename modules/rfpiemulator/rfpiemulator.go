package rfpiemulator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/rfpiemulator/proto"
)

var count int

type CPUTempObj struct {
	TimeStamp   time.Time
	HostAddress string
	CPUTemp     int
}

type CPUPower struct {
	Socket1_CPUPowerUsage float64 `json:"socket1_cpupowerusage"`
	Socket2_CPUPowerUsage float64 `json:"socket2_cpupowerusage"`
}

type MemPower struct {
	Socket1_MemPowerUsage float64 `json:"socket1_mempowerusage"`
	Socket2_MemPowerUsage float64 `json:"socket2_mempowerusage"`
}

type CPUPowerObj struct {
	TimeStamp     time.Time
	HostAddress   string
	CPUPowerUsage CPUPower
}

type MemPowerObj struct {
	TimeStamp     time.Time `json:"timestamp"`
	HostAddress   string    `json:"hostaddress"`
	MemPowerUsage MemPower  `json:"mempowerusage"`
}

type payLoad struct {
	ResetType string
}

func check(e error) {
	if e != nil {
		println(e)
		//panic(e)
	}
}

// Data structure for CPU Frequency Scaling

// Payload for CPU performance scaling request
type CPUPerfScalingReq struct {
	ScalingGovernor  string   `json:"scalinggovernor"`
	ScalingMinFreq   int      `json:"scalingminfreq"`
	ScalingMaxFreq   int      `json:"scalingmaxfreq"`
	NodesAddressList []string `json:"nodesaddresslist,omitempty"`
	Timeout          int      `json:"timeout,omitempty"`
}

// CPU performance scaling response structure
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

func ReadCPUTemp() int {

	tempSensorPath := "/sys/devices/virtual/thermal/thermal_zone0/temp"
	cpuTemp, err := ioutil.ReadFile(tempSensorPath)
	check(err)
	cpuTempInt, e := strconv.Atoi(strings.TrimSuffix(string(cpuTemp), "\n"))
	check(e)
	return cpuTempInt
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
	//pollTicker *time.Ticker
}

// Name returns the FQDN of the module
func (*RFPiEmu) Name() string { return "github.com/hpc/kraken/modules/rfpiemulator" }

// NewConfig returns a fully initialized default config
func (*RFPiEmu) NewConfig() proto.Message {
	localIP := GetNodeIPAddress()
	r := &pb.HostDiscoveryConfig{
		EmulatorIp:   localIP,
		EmulatorPort: "8000",
	}
	return r
}

// UpdateConfig updates the running config
func (rfPiEmu *RFPiEmu) UpdateConfig(cfg proto.Message) (e error) {
	if rcfg, ok := cfg.(*pb.RfPiEmuoveryConfig); ok {
		rfPiEmu.cfg = rcfg
		return
	}
	return fmt.Errorf("invalid config type")
}

// ConfigURL gives the any resolver URL for the config
func (*RFPiEmu) ConfigURL() string {
	cfg := &pb.HostDiscoveryConfig{}
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

	router.HandleFunc("/redfish/v1/Systems/{SystemID}/Processors/power", GetProcessorPowerUsage).Methods(http.MethodGet)
	router.HandleFunc("/redfish/v1/Systems/{SystemID}/Memory/power", GetMemoryPowerUsage).Methods(http.MethodGet)

	router.HandleFunc("/redfish/v1/Chassis/{ChassisID}/Thermal", GetCPUTemp).Methods(http.MethodGet)
	router.HandleFunc("/redfish/v1/Chassis/{ChassisID}/ScaleCPUFreq", ScaleCPUFreq).Methods(http.MethodPut)
	router.HandleFunc("/redfish/v1/Systems/{SystemID}/Actions/Reset", NodePowerControl).Methods(http.MethodPut)

	piIPAddr := rfPiEmu.cfg.GetEmulatorIp()
	piPort := rfPiEmu.cfg.GetEmulatorPort()
	piSrv := net.JoinHostPort(piIPAddr, piPort)
	log.Fatal(http.ListenAndServe(piSrv, router))
}

/***********/

func NodePowerControl(w http.ResponseWriter, r *http.Request) {

	var reset payLoad
	var result []byte
	var msg string

	err := json.NewDecoder(r.Body).Decode(&reset)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
	} else {
		resetType := reset.ResetType
		if resetType == "r" || resetType == "reboot" {
			command := "/bbin/shutdown reboot"
			log.Printf("\nExecuting Command: %v\n", command)
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
			log.Println(msg)
			result, err = json.Marshal(msg)
			if err != nil {
				log.Println(fmt.Sprintf("Could not marshal the response data: %v", err))
			}
			w.Write(result)
		} else if resetType == "h" || resetType == "halt" {
			command := "/bbin/shutdown halt"
			log.Printf("\nExecuting Command: %v\n", command)
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
			log.Println(msg)
			result, err = json.Marshal(msg)
			if err != nil {
				log.Println(fmt.Sprintf("Could not marshal the response data: %v", err))
			}
			w.Write(result)
		}
	}

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
func randTemperature(min, max float64) float64 {
	rand.Seed(time.Now().UnixNano())
	return math.Floor((min+rand.Float64()*(max-min))*100) / 100
}

// CPU power usage
func GetCPUTemp(w http.ResponseWriter, r *http.Request) {

	hostIP := GetNodeIPAddress()

	// Its a mockup CPU temperature
	cpuTempObj := new(CPUTempObj)
	cpuTempObj.TimeStamp = time.Now()
	cpuTempObj.HostAddress = hostIP

	tempVal := ReadCPUTemp()
	//tempVal := randTemperature(1, 100)
	cpuTempObj.CPUTemp = tempVal

	jsonObj, err := json.Marshal(cpuTempObj)

	if err != nil {
		log.Println(fmt.Sprintf("Could not marshal the response data: %v", err))
	}
	w.Write(jsonObj)

}

// CPU power usage
func GetProcessorPowerUsage(w http.ResponseWriter, r *http.Request) {
	hostIP := GetNodeIPAddress()
	//log.Println("\nProcessor Power Usage")

	cpuPwrObj := new(CPUPowerObj)
	cpuPwrObj.TimeStamp = time.Now()
	cpuPwrObj.HostAddress = hostIP
	cpuPwrObj.CPUPowerUsage.Socket1_CPUPowerUsage = 150.25
	cpuPwrObj.CPUPowerUsage.Socket2_CPUPowerUsage = 80.30
	jsonObj, err := json.Marshal(cpuPwrObj)
	if err != nil {
		log.Println(fmt.Sprintf("Could not marshal the response data: %v", err))
	}
	w.Write(jsonObj)

}

// Memory power usage
func GetMemoryPowerUsage(w http.ResponseWriter, r *http.Request) {
	hostIP := GetNodeIPAddress()
	log.Println("\nMemory Power Usage")
	memPwrObj := new(MemPowerObj)
	memPwrObj.TimeStamp = time.Now()
	memPwrObj.HostAddress = hostIP
	memPwrObj.MemPowerUsage.Socket1_MemPowerUsage = 60.99
	memPwrObj.MemPowerUsage.Socket2_MemPowerUsage = 40.56
	jsonObj, err := json.Marshal(memPwrObj)
	if err != nil {
		log.Println(fmt.Sprintf("Could not marshal the response data: %v", err))
	}
	w.Write(jsonObj)
}

func respondWithError(response http.ResponseWriter, statusCode int, msg string) {
	respondWithJSON(response, statusCode, map[string]string{"error": msg})
}

func respondWithJSON(response http.ResponseWriter, statusCode int, data interface{}) {
	result, _ := json.Marshal(data)
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(statusCode)
	response.Write(result)
}

// Scale CPU Performance
func ScaleCPUFreq(w http.ResponseWriter, r *http.Request) {

	var reqPayload CPUPerfScalingReq

	err := json.NewDecoder(r.Body).Decode(&reqPayload)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
	} else {

		// Extracting the desired scaling governor, scaling minimum, scaling maximum frequencies
		scalingGovernor := []byte(reqPayload.ScalingGovernor)
		scalingMaxFreq := []byte(strconv.Itoa(reqPayload.ScalingMaxFreq))
		scalingMinFreq := []byte(strconv.Itoa(reqPayload.ScalingMinFreq))

		basePath := "/sys/devices/system/cpu/cpufreq/policy0/"
		// basePath := "/Users/gali/go/src/cpufreq/"

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

		hostIP := GetNodeIPAddress()

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
			w.Write([]byte(e.Error()))
		}

	}

}
