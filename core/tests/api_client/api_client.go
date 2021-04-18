package main

import (
	"fmt"
	"os"
	"reflect"
	"time"

	ptypes "github.com/gogo/protobuf/types"
	pbd "github.com/kraken-hpc/kraken-legacy/modules/dummy"
	"github.com/kraken-hpc/kraken/core"
	cpb "github.com/kraken-hpc/kraken/core/proto"
)

func main() {

	api := core.NewModuleAPIClient(os.Args[1])

	ns, e := api.QueryReadAll()
	if e != nil {
		fmt.Println(e)
	}
	for _, n := range ns {
		fmt.Printf("%v\n", string(n.JSON()))
	}

	fmt.Println("changing dummy0 config")
	dconf := &pbd.Config{
		Say:  []string{"one", "more", "test"},
		Wait: "1s",
	}
	any, _ := ptypes.MarshalAny(dconf)

	ns[0].SetValue("/Services/dummy0/Config", reflect.ValueOf(any))
	api.QueryUpdate(ns[0])

	fmt.Println("waiting 10s")
	time.Sleep(10 * time.Second)

	self := ns[0].ID().String()

	fmt.Println("stopping restapi")
	me, _ := api.QueryRead(self)
	me.SetValue("/Services/restapi/State", reflect.ValueOf(cpb.ServiceInstance_STOP))
	api.QueryUpdate(me)

	fmt.Println("waiting 10s")
	time.Sleep(10 * time.Second)

	fmt.Println("starting restapi")
	me.SetValue("/Services/restapi/State", reflect.ValueOf(cpb.ServiceInstance_RUN))
	api.QueryUpdate(me)
}
