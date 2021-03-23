package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib/util"
	"google.golang.org/grpc"
)

func usage() {
	fmt.Printf("Usage: %s <free|busy>\n", os.Args[0])
}

var socketFile = "unix:/tmp/kraken.sock"
var sid = "imageapi"
var cmdline = "/proc/cmdline"

var cmdRE = regexp.MustCompile(`kraken.id=([0-9a-zA-Z\-]+)`)

func getUID() string {
	d, e := ioutil.ReadFile(cmdline)
	if e != nil {
		log.Fatalf("failed to extract node ID; could not read kernel commandline: %v", e)
	}
	m := cmdRE.FindAllStringSubmatch(string(d), 1)
	if len(m) == 0 {
		log.Fatal("failed to extract node ID: no match foudn in kernel commandline")
	}
	return m[0][1]
}

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "free" && os.Args[1] != "busy") {
		usage()
		os.Exit(1)
	}

	var stream pb.ModuleAPI_DiscoveryInitClient
	var conn *grpc.ClientConn
	var e error
	if conn, e = grpc.Dial(socketFile, grpc.WithInsecure()); e != nil {
		log.Fatalf("failed to dial API: %v", e)
	}
	client := pb.NewModuleAPIClient(conn)
	if stream, e = client.DiscoveryInit(context.Background()); e != nil {
		log.Fatalf("failed to establish API stream: %v", e)
	}

	log.Printf("discoverying busy = %s", os.Args[1])
	uid := getUID()
	url := util.NodeURLJoin(uid, "/Busy")
	event := &pb.DiscoveryEvent{
		Id:      sid,
		Url:     url,
		ValueId: strings.ToUpper(os.Args[1]),
	}
	if e = stream.Send(event); e != nil {
		log.Fatalf("failed to send discovery event: %v", e)
	}
	stream.CloseAndRecv()
	conn.Close()
}
