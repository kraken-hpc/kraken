package main

import (
	"context"
	"log"
	"time"

	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/types"

	"google.golang.org/grpc"
)

// should work
const simpleNode1 string = `
"id": "Ej5FZ+ibEtOkVkJmVUQAAA==",
"nodename": "",
"runState": "INIT",
"physState": "PHYS_UNKNOWN",
"extensions": [
  {
	"@type": "type.googleapis.com/proto.IPv4OverEthernet",
	"ifaces": [
	  {
		"ip": {
		  "ip": "CgkIBw=="
		}
	  }
	]
  }
]
}
`

// should fail on lack of ip extension
const simpleNode2 string = `
"id": "Ej5FZ+ibEtOkVkJmVUQAAB==",
"nodename": "",
"runState": "INIT",
"physState": "PHYS_UNKNOWN",
"extensions": [
	]
  }
]
}
`

// should fail because not in INIT
const simpleNode3 string = `
"id": "Ej5FZ+ibEtOkVkJmVUQAAA==",
"nodename": "",
"runState": "SYNC",
"physState": "PHYS_UNKNOWN",
"extensions": [
  {
	"@type": "type.googleapis.com/proto.IPv4OverEthernet",
	"ifaces": [
	  {
		"ip": {
		  "ip": "CgkIBw=="
		}
	  }
	]
  }
]
}
`

func send(n types.Node) {
	conn, e := grpc.Dial("127.0.0.1:31415", grpc.WithInsecure())
	if e != nil {
		log.Printf("did not connect: %v", e)
	}
	defer conn.Close()
	c := pb.NewStateSyncClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, e := c.RPCPhoneHome(ctx, &pb.PhoneHomeRequest{Id: n.ID().Binary()})
	if e != nil {
		log.Printf("could not phone home: %v", e)
	}
	log.Printf("%v\n", r)
}

func main() {
	n1 := core.NewNodeFromJSON([]byte(simpleNode1))
	n2 := core.NewNodeFromJSON([]byte(simpleNode2))
	n3 := core.NewNodeFromJSON([]byte(simpleNode3))

	send(n1)
	time.Sleep(time.Second * 3)
	send(n2)
	time.Sleep(time.Second * 3)
	send(n3)
}
