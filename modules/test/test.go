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
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/golang/protobuf/ptypes"

	"github.com/golang/protobuf/proto"
	"github.com/hpc/kraken/core"
	cpb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/extensions/IPv4"
	thpb "github.com/hpc/kraken/extensions/Thermal/proto"
	"github.com/hpc/kraken/lib"
	pb "github.com/hpc/kraken/modules/test/proto"
)

const (
	ThermalStateURL = "type.googleapis.com/proto.Thermal/State"
	SrvStateURL     = "/Services/test/State"
)

var _ lib.Module = (*Test)(nil)
var _ lib.ModuleWithConfig = (*Test)(nil)
var _ lib.ModuleWithDiscovery = (*Test)(nil)
var _ lib.ModuleSelfService = (*Test)(nil)

// PXE provides PXE-boot capabilities
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
	ipmap := make(map[string]lib.NodeID)
	bySrv := make(map[string][]string)

	// get ip addresses for nodes
	for _, n := range ns {
		vs := n.GetValues([]string{t.cfg.GetIpUrl(), t.cfg.GetAggUrl()})
		if len(vs) != 2 {
			t.api.Logf(lib.LLERROR, "problem getting ip addresses and agg name for nodes")
		}
		ip := IPv4.BytesToIP(vs[t.cfg.GetIpUrl()].Bytes())
		aggName := vs[t.cfg.GetAggUrl()].String()
		ipmap[ip.String()] = n.ID()
		bySrv[aggName] = append(bySrv[aggName], ip.String())
	}

	t.api.Logf(lib.LLDEBUG, "got ip addresses: %v", bySrv)
	for _, n := range ns {
		t.fakeDiscover(n)
	}
}

func (t *Test) fakeDiscover(node lib.Node) {

	var vid thpb.Thermal_CPU_TEMP_STATE
	vid = thpb.Thermal_CPU_TEMP_HIGH

	url := lib.NodeURLJoin(node.ID().String(), ThermalStateURL)
	v := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			Module:  t.Name(),
			URL:     url,
			ValueID: vid.String(),
		},
	)
	t.dchan <- v
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
