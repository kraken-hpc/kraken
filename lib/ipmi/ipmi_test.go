package ipmi

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"testing"
)

func TestPacker_Pack(t *testing.T) {
	p := Packer{
		ByteOrder: binary.BigEndian,
	}
	data := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80, 0x90}
	t.Run("RMCPHeader", func(t *testing.T) {
		r := RMCPHeader{
			Version:        0x01,
			SequenceNumber: 0x02,
			Class:          0x03,
			Data:           data,
		}
		b, es := p.Pack(&r)
		if len(es) > 0 {
			t.Logf("%v", es)
			t.Errorf("%v", es[len(es)-1])
		}
		t.Logf("%v", hex.Dump(b))
	})
	t.Run("ASFMessageHeader(len)", func(t *testing.T) {
		r := ASFMessageHeader{
			IANA: 0x11223344,
			Type: 0x01,
			Tag:  0x02,
			Data: data,
		}
		b, es := p.Pack(&r)
		if len(es) > 0 {
			t.Logf("%v", es)
			t.Errorf("%v", es[len(es)-1])
		}
		t.Logf("%v", hex.Dump(b))
	})
	t.Run("IPMIRequest(cksum2)", func(t *testing.T) {
		r := IPMIRequest{
			RqAddr: 0x01,
			RqSeq:  0x02,
			Cmd:    0x03,
			Data:   data,
		}
		b, es := p.Pack(&r)
		if len(es) > 0 {
			t.Logf("%v", es)
			t.Errorf("%v", es[len(es)-1])
		}
		t.Logf("%v", hex.Dump(b))
	})
	t.Run("ASFMessagePong(array)", func(t *testing.T) {
		r := ASFMessagePong{
			IANA:         0x1122,
			OEM:          0x3344,
			Entities:     0x01,
			Interactions: 0x02,
		}
		b, es := p.Pack(&r)
		if len(es) > 0 {
			t.Logf("%v", es)
			t.Errorf("%v", es[len(es)-1])
		}
		t.Logf("%v", hex.Dump(b))
	})
}

func TestPacker_Unpack(t *testing.T) {
	p := Packer{
		ByteOrder: binary.BigEndian,
	}
	t.Run("RMCHeader", func(t *testing.T) {
		b := []byte{0x01, 0x00, 0x02, 0x03, 0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80, 0x90}
		r := RMCPHeader{}
		es := p.Unpack(b, &r)
		if len(es) > 0 {
			t.Logf("%v", es)
			t.Errorf("%v", es[len(es)-1])
		}
		t.Logf("%v", r)
	})
	t.Run("ASFMessageHeader(len)", func(t *testing.T) {
		b := []byte{0x11, 0x22, 0x33, 0x44, 0x01, 0x02, 0x00, 0x09, 0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80, 0x90}
		r := ASFMessageHeader{}
		es := p.Unpack(b, &r)
		if len(es) > 0 {
			t.Logf("%v", es)
			t.Errorf("%v", es[len(es)-1])
		}
		t.Logf("%v", r)
	})
	t.Run("IPMIRequest(cksum2)", func(t *testing.T) {
		b := []byte{0x01, 0x02, 0x03, 0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80, 0x90, 0x2a}
		r := IPMIRequest{}
		es := p.Unpack(b, &r)
		if len(es) > 0 {
			t.Logf("%v", es)
			t.Errorf("%v", es[len(es)-1])
		}
		t.Logf("%v", r)
	})
	t.Run("ASFMessagePong(array)", func(t *testing.T) {
		b := []byte{0x00, 0x00, 0x11, 0x22, 0x00, 0x00, 0x33, 0x44, 0x01, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		r := ASFMessagePong{}
		es := p.Unpack(b, &r)
		if len(es) > 0 {
			t.Logf("%v", es)
			t.Errorf("%v", es[len(es)-1])
		}
		t.Logf("%v", r)
	})
}

func TestIPMISession(t *testing.T) {
	s := NewIPMISession(net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 623,
	})
	e := s.Start("admin", "admin")
	if e != nil {
		t.Errorf("%v", e)
	}
	cc, _, e := s.Send(IPMIFnChassisReq, IPMICmdChassisCtl, []byte{IPMIChassisCtlDown})
	if e != nil {
		t.Errorf("%v", e)
	}
	if cc != 0 {
		t.Errorf("bad completion code: %x", cc)
	}
	s.Close()
}
