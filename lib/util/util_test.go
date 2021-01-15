package util

import (
	"net"
	"reflect"
	"testing"

	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	pb "github.com/hpc/kraken/core/proto"
	ipb "github.com/hpc/kraken/extensions/IPv4"
)

func TestDiff(t *testing.T) {
	n1 := &pb.Node{
		Nodename:  "node1",
		RunState:  pb.Node_INIT,
		PhysState: pb.Node_POWER_ON,
	}
	n2 := &pb.Node{
		Nodename:  "node2",
		RunState:  pb.Node_SYNC,
		PhysState: pb.Node_POWER_ON,
	}
	i1 := &ipb.IPv4{
		Ip:     net.ParseIP("192.168.1.1").To4(),
		Subnet: net.ParseIP("255.255.255.0").To4(),
	}
	ia1, _ := ptypes.MarshalAny(i1)
	i2 := &ipb.IPv4{
		Ip:     net.ParseIP("192.168.1.2").To4(),
		Subnet: net.ParseIP("255.255.255.0").To4(),
	}
	ia2, _ := ptypes.MarshalAny(i2)
	n1.Extensions = append(n1.Extensions, ia1)
	n2.Extensions = append(n2.Extensions, ia2)
	// note: ordering is important since we use DeepEqual
	s := []string{"/Nodename", "/RunState"}
	if proto.Equal(n1, n2) {
		t.Errorf("unequal nodes look equal")
	} else {
		t.Logf("nodes are not equal (and shouldn't be)")
	}

	d, e := MessageDiff(n1, n2, "")
	if e != nil {
		t.Error(e)
	}
	t.Logf("diff: %v", d)
	if !reflect.DeepEqual(d, s) {
		t.Error("Incorrect diff")
	}
}

func TestURLShift(t *testing.T) {
	type test struct {
		url  string
		root string
		sub  string
	}
	tests := []test{
		{
			url:  "This/is/a/test",
			root: "This",
			sub:  "is/a/test",
		},
		{
			url:  "/This/is/a/test",
			root: "This",
			sub:  "is/a/test",
		},
		{
			url:  "",
			root: "",
			sub:  "",
		},
		{
			url:  "test",
			root: "test",
			sub:  "",
		},
		{
			url:  "test/",
			root: "test",
			sub:  "",
		},
	}
	for _, v := range tests {
		t.Run(v.url,
			func(t *testing.T) {
				root, sub := URLShift(v.url)
				t.Logf("url: %s, root: %s, sub: %s", v.url, root, sub)
				if root != v.root {
					t.Errorf("url: %s, root: %s != %s", v.url, root, v.root)
				}
				if sub != v.sub {
					t.Errorf("url: %s, sub: %s != %s", v.url, sub, v.sub)
				}
			})
	}
}
