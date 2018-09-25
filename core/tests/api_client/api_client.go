package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	_ "github.com/hpc/kraken/extensions"
	"github.com/hpc/kraken/lib"
	_ "github.com/hpc/kraken/modules"
	pbd "github.com/hpc/kraken/modules/dummy/proto"
)

func main() {

	var s lib.ServiceInstance
	api := core.NewAPIClient(os.Args[1])

	ns, e := api.QueryReadAll()
	if e != nil {
		fmt.Println(e)
	}
	for _, n := range ns {
		fmt.Printf("%v\n", string(n.JSON()))
	}

	fmt.Println("changing dummy0 config")
	dconf := &pbd.DummyConfig{
		Say:  []string{"one", "more", "test"},
		Wait: "1s",
	}
	any, _ := ptypes.MarshalAny(dconf)

	s = ns[0].GetService("dummy0")
	s.UpdateConfig(any)
	api.QueryUpdate(ns[0])

	fmt.Println("waiting 10s")
	time.Sleep(10 * time.Second)

	self := ns[0].ID().String()

	fmt.Println("stopping dummy0")
	me, _ := api.QueryRead(self)
	s = me.GetService("dummy0")
	s.SetState(lib.Service_STOP)
	api.QueryUpdate(me)

	fmt.Println("waiting 10s")
	time.Sleep(10 * time.Second)

	fmt.Println("starting dummy0")
	s = me.GetService("dummy0")
	s.SetState(lib.Service_RUN)
	api.QueryUpdate(me)
}
