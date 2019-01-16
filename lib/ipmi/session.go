package ipmi

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

/*
 * Session sequence:
 * RMCP/ASF Ping/Pong - to verify IPMI support
 * GetChannelAuthCap - see what auth support exists
 * GetSessionChallenge - start auth
 * ActivateSession - finish auth/activate session
 * SetPrivLevel - set our privilege level
 * ...
 * CloseSession - bye!
 */

var packer = Packer{ByteOrder: binary.BigEndian}

type IPMISession struct {
	addr     *net.UDPAddr
	ch       uint8
	sid      uint32
	sseq     uint32
	rqseq    uint8
	authType uint8
	authID   []byte
	sessChal []byte
	ticker   time.Ticker // for keepalive pings
	active   bool
	passwd   [16]byte
	conn     *net.UDPConn
}

func NewIPMISession(addr *net.UDPAddr) *IPMISession {
	s := &IPMISession{
		addr:     addr,
		rqseq:    0,
		sseq:     0,
		sid:      0,
		authType: 0,
	}
	return s
}

func (s *IPMISession) Start(user, pass string) (e error) {
	s.conn, e = net.DialUDP("udp4", nil, s.addr)
	fmt.Println(s.addr)
	if e != nil {
		return e
	}
	if e = s.asfPing(); e != nil {
		return e
	}
	cc, data, e := s.ipmiChat(IPMIFnAppReq, 0, IPMICmdGetChanAuthCap, []byte{IPMIGetChanAuthCapGetChannel, IPMIPrivAdmin})
	if e != nil {
		return e
	}
	if cc != 0 {
		return fmt.Errorf("bad completion code on GetChanAuthCap: %x", cc)
	}
	s.ch = data[0]
	if data[1]&IPMIAuthTypeBFMD5 == 0 {
		return fmt.Errorf("channel does not support MD5")
	}
	username := append([]byte(user), make([]byte, 16-len([]byte(user)))...)
	cc, data, e = s.ipmiChat(IPMIFnAppReq, 0, IPMICmdGetSessionChal, append([]byte{IPMIAuthTypeMD5}, username...))
	if e != nil {
		return e
	}
	if cc != 0 {
		// 0x81: bad username
		// 0x82: null username not enabled
		return fmt.Errorf("bad completion code on GetSessionChal: %x", cc)
	}
	if len(data) != 20 {
		return fmt.Errorf("bad session challenge, wrong length")
	}
	s.sid = packer.ByteOrder.Uint32(data[0:4])
	s.sessChal = data[4:20]
	s.authType = IPMIAuthTypeMD5
	copy(s.passwd[:], []byte(pass))
	aData := []byte{IPMIAuthTypeMD5, IPMIPrivAdmin}
	aData = append(aData, s.sessChal...)
	seqBuf := make([]byte, 4)
	packer.ByteOrder.PutUint32(seqBuf, 1)
	aData = append(aData, seqBuf...)
	cc, data, e = s.ipmiChat(IPMIFnAppReq, 0, IPMICmdActivateSess, aData)
	if e != nil {
		return e
	}
	if cc != 0 {
		// 0x81: no slot available
		// 0x82: no slot available for given user
		// 0x83: no slot available to support user due to maximum privilege cap
		// 0x84: session sequence number out of range
		// 0x85: invalid session ID in rquest
		// 0x86: requested maximum privilege level exceeds user and/or channel privilege limit
		return fmt.Errorf("bad completion code on ActivateSess: %x", cc)
	}
	if len(data) != 10 {
		return fmt.Errorf("bad activate session response data, wrong length")
	}
	if data[9] < IPMIPrivAdmin {
		return fmt.Errorf("admin privileges were disallowed")
	}
	s.authType = data[0]
	s.sid = packer.ByteOrder.Uint32(data[1:5])
	s.sseq = packer.ByteOrder.Uint32(data[5:9])
	cc, data, e = s.ipmiChat(IPMIFnAppReq, 0, IPMICmdSetSessionPriv, []byte{IPMIPrivAdmin})
	if e != nil {
		return e
	}
	if cc != 0 {
		// 0x80: requested level not available to this user
		// 0x81: requested level exceeds channel/user privilege limit
		// 0x82: cannot disable user level authentication
		return fmt.Errorf("bad completion code on SetSessionPriv: %x", cc)
	}
	if data[0] != IPMIPrivAdmin {
		return fmt.Errorf("failed to set privilege level")
	}
	s.active = true
	return
}

func (s *IPMISession) Close() {
	if s.active {
		data := make([]byte, 4)
		packer.ByteOrder.PutUint32(data, s.sid)
		s.ipmiChat(IPMIFnAppReq, 0, IPMICmdCloseSess, data)
		// we don't even check to see if this fails
	}
}

// blocking
func (s *IPMISession) asfPing() (e error) {
	pingASF := &ASFMessageHeader{
		IANA: ASFIANA,
		Type: ASFTypePing,
		Tag:  0x00,
	}
	pingRMCP := &RMCPHeader{
		Version:        RMCPVersion1_0,
		SequenceNumber: RMCPSeqNoACK,
		Class:          RMCPClassASF,
		Data:           packer.PackMust(pingASF),
	}
	packet := packer.PackMust(pingRMCP)
	wrote, e := s.conn.Write(packet)
	if e != nil {
		return e
	}
	if wrote != len(packet) {
		return fmt.Errorf("failed to send whole ping packet")
	}
	buff := make([]byte, 65515)
	rchan := make(chan []byte)
	go func() {
		s.conn.SetDeadline(time.Now().Add(time.Second * 10))
		read, e := s.conn.Read(buff)
		if e != nil {
			return
		}
		packet := make([]byte, read)
		copy(packet, buff[0:read])
		rchan <- packet
	}()

	timeout := time.NewTimer(time.Second * 10)
	success := false
	for success == false {
		select {
		case <-timeout.C:
			return fmt.Errorf("ASF ping timeout")
		case p := <-rchan:
			packer.Unpack(p, pingRMCP)
			if pingRMCP.Class != RMCPClassASF {
				continue
			}
			packer.Unpack(pingRMCP.Data, pingASF)
			if pingASF.Type != ASFTypePong {
				continue
			}
			pong := &ASFMessagePong{}
			packer.Unpack(pingASF.Data, pong)
			if pong.Entities&ASFEntitiesIPMISupport == 0 {
				return fmt.Errorf("remote host does not support IPMI")
			}
			success = true
		}
	}
	return
}

func (s *IPMISession) ipmiChat(NetFn, LUN, Cmd uint8, Data []byte) (cc uint8, data []byte, e error) {
	msg := &IPMIRequest{
		RqAddr: 0x81, //FIXME: don't hardcode
		RqSeq:  s.rqseq,
		Cmd:    Cmd,
		Data:   Data,
	}
	s.rqseq++
	msgHdr := &IPMIMessageHeader{
		RsAddr:   IMPIRsAddrBMCResponder,
		NetFnLun: (NetFn << 2) | (LUN & 0x03),
		Data:     packer.PackMust(msg),
	}
	sessHdr := &IPMISessionHeader{
		AuthType:              s.authType,
		SessionSequenceNumber: s.sseq,
		SessionID:             s.sid,
		Payload:               packer.PackMust(msgHdr),
	}
	if sessHdr.AuthType != IPMIAuthTypeNONE {
		if sessHdr.AuthType == IPMIAuthTypeMD5 {
			sessHdr.MsgAuthCode = s.MD5(sessHdr.Payload)
		}
	}
	if s.active {
		s.sseq++
	}
	rmcpHdr := &RMCPHeader{
		Version:        RMCPVersion1_0,
		SequenceNumber: RMCPSeqNoACK,
		Class:          RMCPClassIPMI,
		Data:           packer.PackMust(sessHdr),
	}
	packet := packer.PackMust(rmcpHdr)
	wrote, e := s.conn.Write(packet)
	if e != nil {
		return 0, nil, e
	}
	if wrote != len(packet) {
		e = fmt.Errorf("failed to send whole packet")
		return
	}
	buff := make([]byte, 65515)
	rchan := make(chan []byte)
	go func() {
		s.conn.SetDeadline(time.Now().Add(time.Second * 10))
		read, e := s.conn.Read(buff)
		if e != nil {
			return
		}
		packet := make([]byte, read)
		copy(packet, buff[0:read])
		rchan <- packet
	}()

	timeout := time.NewTimer(time.Second * 10)
	success := false
	for success == false {
		select {
		case <-timeout.C:
			e = fmt.Errorf("GetAuthCap timeout")
			return
		case p := <-rchan:
			rmcpHdr = &RMCPHeader{}
			es := packer.Unpack(p, rmcpHdr)
			if len(es) > 0 {
				e = es[0]
				return
			}
			if rmcpHdr.Class != RMCPClassIPMI {
				continue
			}
			sessHdr = &IPMISessionHeader{}
			es = packer.Unpack(rmcpHdr.Data, sessHdr)
			if len(es) > 0 {
				e = es[0]
				return
			}
			msgHdr = &IPMIMessageHeader{}
			es = packer.Unpack(sessHdr.Payload, msgHdr)
			if len(es) > 0 {
				e = es[0]
				return
			}
			if (msgHdr.NetFnLun>>2)%2 != 1 {
				//e = fmt.Errorf("got an unexpected IPMI request, not response")
				//return
				fmt.Printf("WARNING: got an unexpected IPMI request, not response: %x\n", msgHdr.NetFnLun>>2)
				continue
			}
			if (msgHdr.NetFnLun>>2)-1 != NetFn {
				fmt.Printf("WARNING: got a response packet with an incorrect NetFn: %x\n", msgHdr.NetFnLun>>2)
				//continue
			}
			msgResp := &IPMIResponse{}
			es = packer.Unpack(msgHdr.Data, msgResp)
			if len(es) > 0 {
				e = es[0]
				return
			}
			cc = msgResp.CompCode
			data = msgResp.Data
			success = true
		}
	}
	return
}

func (s *IPMISession) Send(NetFun, Cmd uint8, Data []byte) (cc uint8, data []byte, e error) {
	if !s.active {
		e = fmt.Errorf("called send on an session that has not been activated")
		return
	}
	return s.ipmiChat(NetFun, 0, Cmd, Data)
}

func (s *IPMISession) MD5(msgData []byte) []byte {
	var hStr []byte
	bSid := make([]byte, 4)
	bSeq := make([]byte, 4)
	packer.ByteOrder.PutUint32(bSid, s.sid)
	packer.ByteOrder.PutUint32(bSeq, s.sseq)
	hStr = append(hStr, s.passwd[:]...)
	hStr = append(hStr, bSid...)
	hStr = append(hStr, msgData...)
	hStr = append(hStr, bSeq...)
	hStr = append(hStr, s.passwd[:]...)
	sum := md5.Sum(hStr)
	return sum[:]
}
