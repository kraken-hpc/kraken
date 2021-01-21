package core

import (
	"encoding/hex"
	"reflect"
	"testing"

	. "github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/core/proto"
	ct "github.com/hpc/kraken/core/proto/customtypes"
)

func TestNewNodeWithID(t *testing.T) {
	nid := ct.NewNodeID("123e4567-e89b-12d3-a456-426655440000")
	n := NewNodeWithID("123e4567-e89b-12d3-a456-426655440000")
	if !n.ID().EqualTo(nid) {
		t.Errorf("failed to create equal NIDs on new node: %v", n)
	}
}

func TestNode_GetValue(t *testing.T) {
	n := NewNodeWithID("123e4567-e89b-12d3-a456-426655440000")
	v, e := n.GetValue("/Id")
	if e != nil {
		t.Error(e)
		return
	}
	if !v.IsValid() {
		t.Errorf("invalid value returned: %v", v)
		return
	}
	ref := ct.NewNodeID("123e4567-e89b-12d3-a456-426655440000")
	if !v.Interface().(ct.NodeID).Equal(*ref) {
		t.Errorf("result mismatch: %v != %v", v.Interface(), ref)
	}
}

func TestNode_SetValue(t *testing.T) {
	n := NewNodeWithID("123e4567-e89b-12d3-a456-426655440000")
	v, e := n.SetValue("/Nodename", reflect.ValueOf("testing"))
	if e != nil {
		t.Error(e)
		return
	}
	if v.Interface().(string) != "testing" {
		t.Errorf("repoted change mismatch: %s != testing", v.Interface().(string))
		return
	}
	v, e = n.GetValue("/Nodename")
	if e != nil {
		t.Error(e)
		return
	}
	if v.Interface().(string) != "testing" {
		t.Errorf("repoted change mismatch: %s != testing", v.Interface().(string))
		return
	}
}

func simpleNode() *Node {
	n := NewNodeWithID("123e4567-e89b-12d3-a456-426655440000")
	n.SetValues(
		map[string]reflect.Value{
			"/Nodename":  reflect.ValueOf("noname"),
			"/RunState":  reflect.ValueOf(pb.Node_INIT),
			"/PhysState": reflect.ValueOf(pb.Node_POWER_ON),
		})
	return n
}

func TestNode_Binary(t *testing.T) {
	n := simpleNode()

	out := n.Binary()
	if len(out) == 0 {
		t.Error("marshal failed")
	}
	t.Logf("Proto message (binary): \n%s\n", hex.Dump(out))
}

func TestNode_JSON(t *testing.T) {
	n := simpleNode()

	out := n.JSON()
	if len(out) == 0 {
		t.Error("zero length output")
		return
	}
	t.Logf("Proto message (JSON): \n%s\n", out)
}

func TestNode_Message(t *testing.T) {
	n := simpleNode()

	out := n.Message()
	t.Logf("Proto message: \n%s\n", out.String())
}

func TestNewNodeFromJSON(t *testing.T) {
	jin := []byte("{\"id\":\"123e4567-e89b-12d3-a456-426655440000\",\"nodename\":\"noname\",\"run_state\":\"INIT\",\"phys_state\":1}")
	t.Logf("in: %s", jin)

	n := NewNodeFromJSON(jin)
	if n.ID().Nil() {
		t.Error("invalid node")
	}

	out := n.JSON()
	if len(out) == 0 {
		t.Error("zero length output")
	}
	t.Logf("out: %s", out)
}

func TestNewNodeFromBinary(t *testing.T) {
	in := "0a10123e4567e89b12d3a45642665544000012066e6f6e616d6518012001"
	bin, err := hex.DecodeString(in)
	t.Logf("in: \n%s", hex.Dump(bin))
	if err != nil {
		t.Error(err)
	}
	n := NewNodeFromBinary(bin)
	if n.ID().Nil() {
		t.Error("invalid node")
	}

	out := n.Binary()
	if len(out) == 0 {
		t.Error(err)
	}
	t.Logf("out: \n%s", hex.Dump(out))
	if string(bin) != string(out) {
		t.Errorf("input does not match output")
	}
}

/*
func ipExtension() lib.ProtoMessage {
	i := &IPv4OverEthernet{}
	hwaddr0, _ := net.ParseMAC("00:11:22:33:44:55")
	hwaddr1, _ := net.ParseMAC("66:77:88:99:aa:bb")
	e := &pb.IPv4OverEthernet{
		Ifaces: []*pb.IPv4OverEthernet_ConfiguredInterface{
			{
				Eth: &pb.Ethernet{
					Iface:   "eth0",
					Mac:     hwaddr0,
					Mtu:     1500,
					Control: true,
				},
				Ip: &pb.IPv4{
					Ip:     net.ParseIP("192.168.1.1").To4(),
					Subnet: net.ParseIP("255.255.255.0").To4(),
				},
			},
			{
				Eth: &pb.Ethernet{
					Iface:   "eth1",
					Mac:     hwaddr1,
					Mtu:     9000,
					Control: true,
				},
				Ip: &pb.IPv4{
					Ip:     net.ParseIP("10.0.1.1").To4(),
					Subnet: net.ParseIP("255.255.0.0").To4(),
				},
			},
		},
		Routes:         []*pb.IPv4{},
		DnsNameservers: []*pb.IPv4{},
	}
	i.SetMessage(e)
	return i
}
func TestNodeAddExtension(t *testing.T) {
	n := simpleNode()
	i := ipExtension()
	err := n.AddExtension(i)
	if err != nil {
		t.Error(err)
	}
	j, err := n.MarshalJSON()
	if err != nil {
		t.Error(err)
	}
	t.Logf("out (JSON): \n%s", j)
}

func TestNodeGetExtensionUrls(t *testing.T) {
	n := simpleNode()
	n2 := simpleNode()
	i := ipExtension()
	err := n.AddExtension(i)
	if err != nil {
		t.Error(err)
	}
	err = n.AddExtension(&n2)
	if err != nil {
		t.Error(err)
	}
	r := n.GetExtensionUrls()
	t.Logf("urls: %v", r)
	if len(r) != 2 {
		t.Errorf("got incorrect number of extensions")
	}
}

func TestNodeDupAddExt(t *testing.T) {
	n := simpleNode()
	i := ipExtension()
	i2 := ipExtension()
	err := n.AddExtension(i)
	if err != nil {
		t.Error(err)
	}
	err = n.AddExtension(i2)
	if err == nil {
		t.Errorf("failed to error on duplicate extension add")
	}
}

func TestNodeGetExtension(t *testing.T) {
	n := simpleNode()
	i := ipExtension()
	err := n.AddExtension(i)
	if err != nil {
		t.Error(err)
	}
	pm := n.GetExtension("type.googleapis.com/IPv4.IPv4OverEthernet")
	if pm == nil {
		t.Errorf("failed to get extionsion that should have been there")
	}
	j, _ := pm.MarshalJSON()
	t.Logf("out: %s", j)
	pm = n.GetExtension("blatz")
	if pm != nil {
		t.Logf("got a non-nil result for a non-existent extension lookup")
	}
}

func TestNodeDelExtension(t *testing.T) {
	n := simpleNode()
	i := ipExtension()
	err := n.AddExtension(i)
	if err != nil {
		t.Error(err)
	}

	j, _ := n.MarshalJSON()
	t.Logf("pre-delete: %s", j)
	n.DelExtension("bad delete")

	j, _ = n.MarshalJSON()
	t.Logf("bad-delete: %s", j)

	n.DelExtension("type.googleapis.com/IPv4.IPv4OverEthernet")

	j, _ = n.MarshalJSON()
	t.Logf("post-delete: %s", j)
}

func TestNodeDiff(t *testing.T) {
	n1 := simpleNode()
	n2 := simpleNode()
	n2.GetMessage().(*pb.Node).Nodename = "node2"

	r, err := n1.Diff(&n2, "")
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v", r)
	if len(r) != 1 {
		t.Errorf("got %d different fields, was expecting 1", len(r))
	}
	j, _ := n1.MarshalJSON()
	t.Logf("%s", j)
	j, _ = n2.MarshalJSON()
	t.Logf("%s", j)
}

func TestNodeDiffWExt(t *testing.T) {
	n1 := simpleNode()
	i1 := ipExtension()
	n1.AddExtension(i1)

	n2 := simpleNode()
	n2.GetMessage().(*pb.Node).Nodename = "node2"
	i2 := ipExtension()
	i2.GetMessage().(*pb.IPv4OverEthernet).Ifaces[0].Eth.Mtu = 9000
	n2.AddExtension(i2)

	r, err := n1.Diff(&n2, "")
	t.Logf("different fields: %v", r)
	if err != nil {
		t.Error(err)
	}
	if len(r) != 2 {
		t.Errorf("got %d different fields, was expecting 2", len(r))
	}
}

func TestResolveURL(t *testing.T) {
	n := simpleNode()
	i := ipExtension()
	n.AddExtension(i)

	m := n.GetMessage().(*pb.Node)
	im := i.GetMessage().(*pb.IPv4OverEthernet)

	type test struct {
		url string
		res reflect.Value
		err error
	}
	tests := []test{
		{
			url: "/Nodename",
			res: reflect.ValueOf(m.Nodename),
			err: nil,
		},
		{
			url: "type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Eth/Mtu",
			res: reflect.ValueOf(im.Ifaces[0].Eth.Mtu),
			err: nil,
		},
		{
			url: "/type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Eth",
			res: reflect.ValueOf(im.Ifaces[0].Eth),
			err: nil,
		},
	}

	for _, v := range tests {
		res, err := n.GetValue(v.url)
		t.Logf("url: %s, result: %v, err: %v", v.url, res, err)
		// Don't == reflect.Values!  Doesn't do what you expect.
		if !reflect.DeepEqual(res.Interface(), v.res.Interface()) {
			t.Errorf("url: %s, %v != %v", v.url, res, v.res)
		}
		if v.err != err {
			t.Errorf("url: %s, %s != %s", v.url, err, v.err)
		}
	}
}
func TestNodeMerge(t *testing.T) {
	n1 := simpleNode()
	i1 := ipExtension()
	n1.AddExtension(i1)

	n2 := simpleNode()
	n2.GetMessage().(*pb.Node).Nodename = "node2"
	i2 := ipExtension()
	i2.GetMessage().(*pb.IPv4OverEthernet).Ifaces[0].Eth.Mtu = 9000
	n2.AddExtension(i2)
	j, _ := n1.MarshalJSON()
	t.Logf(string(j))

	c, err := n1.Merge(&n2, "")
	t.Logf("changed fields: %v", c)
	if err != nil {
		t.Error(err)
	}
	j, _ = n1.MarshalJSON()
	t.Logf(string(j))
	if len(c) != 2 {
		t.Errorf("merged %d changes, should have merged 2", len(c))
	}
	m := n1.GetMessage().(*pb.Node)
	i := n1.GetExtension("type.googleapis.com/IPv4.IPv4OverEthernet").GetMessage().(*pb.IPv4OverEthernet)
	if m.Nodename != "node2" {
		t.Errorf("nodename should be node2, is %s", m.Nodename)
	}
	if i.Ifaces[0].Eth.Mtu != 9000 {
		t.Errorf("Mtu should be 9000, is %d", i.Ifaces[0].Eth.Mtu)
	}
}

/* It's not clear that we want this....
func TestSparseNodeMerge(t *testing.T) {
	n1 := simpleNode()
	i1 := ipExtension()
	n1.AddExtension(i1)

	n2pb := pb.Node{
		Nodename: "testing",
	}
	n2 := Node{}
	n2.SetMessage(&n2pb)
	c, e := n1.Merge(&n2)
	t.Logf("%v %v", c, e)
}

func TestSetValue(t *testing.T) {
	n := simpleNode()
	i := ipExtension()
	n.AddExtension(i)

	j, _ := n.MarshalJSON()
	t.Log(string(j))

	var mtu uint32
	mtu = 9000
	v, e := n.SetValue("type.googleapis.com/IPv4.IPv4OverEthernet/Ifaces/kraken/Eth/Mtu", reflect.ValueOf(mtu))
	if e != nil {
		t.Error(e)
	}
	if v.Interface() != uint32(9000) {
		t.Errorf("failed to set value: %v should be %v", v, mtu)
	}
	j, _ = n.MarshalJSON()
	t.Log(string(j))
}

*/
