// Code generated by protoc-gen-go. DO NOT EDIT.
// source: test.proto

package proto

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type TestConfig struct {
	Servers              map[string]*TestServer `protobuf:"bytes,1,rep,name=servers,proto3" json:"servers,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	IpUrl                string                 `protobuf:"bytes,2,opt,name=ip_url,json=ipUrl,proto3" json:"ip_url,omitempty"`
	AggUrl               string                 `protobuf:"bytes,3,opt,name=agg_url,json=aggUrl,proto3" json:"agg_url,omitempty"`
	PollingInterval      string                 `protobuf:"bytes,4,opt,name=polling_interval,json=pollingInterval,proto3" json:"polling_interval,omitempty"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *TestConfig) Reset()         { *m = TestConfig{} }
func (m *TestConfig) String() string { return proto.CompactTextString(m) }
func (*TestConfig) ProtoMessage()    {}
func (*TestConfig) Descriptor() ([]byte, []int) {
	return fileDescriptor_test_b42f5a1e5f80b8ef, []int{0}
}
func (m *TestConfig) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TestConfig.Unmarshal(m, b)
}
func (m *TestConfig) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TestConfig.Marshal(b, m, deterministic)
}
func (dst *TestConfig) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TestConfig.Merge(dst, src)
}
func (m *TestConfig) XXX_Size() int {
	return xxx_messageInfo_TestConfig.Size(m)
}
func (m *TestConfig) XXX_DiscardUnknown() {
	xxx_messageInfo_TestConfig.DiscardUnknown(m)
}

var xxx_messageInfo_TestConfig proto.InternalMessageInfo

func (m *TestConfig) GetServers() map[string]*TestServer {
	if m != nil {
		return m.Servers
	}
	return nil
}

func (m *TestConfig) GetIpUrl() string {
	if m != nil {
		return m.IpUrl
	}
	return ""
}

func (m *TestConfig) GetAggUrl() string {
	if m != nil {
		return m.AggUrl
	}
	return ""
}

func (m *TestConfig) GetPollingInterval() string {
	if m != nil {
		return m.PollingInterval
	}
	return ""
}

type TestServer struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Ip                   string   `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
	Port                 int32    `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *TestServer) Reset()         { *m = TestServer{} }
func (m *TestServer) String() string { return proto.CompactTextString(m) }
func (*TestServer) ProtoMessage()    {}
func (*TestServer) Descriptor() ([]byte, []int) {
	return fileDescriptor_test_b42f5a1e5f80b8ef, []int{1}
}
func (m *TestServer) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TestServer.Unmarshal(m, b)
}
func (m *TestServer) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TestServer.Marshal(b, m, deterministic)
}
func (dst *TestServer) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TestServer.Merge(dst, src)
}
func (m *TestServer) XXX_Size() int {
	return xxx_messageInfo_TestServer.Size(m)
}
func (m *TestServer) XXX_DiscardUnknown() {
	xxx_messageInfo_TestServer.DiscardUnknown(m)
}

var xxx_messageInfo_TestServer proto.InternalMessageInfo

func (m *TestServer) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *TestServer) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *TestServer) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func init() {
	proto.RegisterType((*TestConfig)(nil), "proto.TestConfig")
	proto.RegisterMapType((map[string]*TestServer)(nil), "proto.TestConfig.ServersEntry")
	proto.RegisterType((*TestServer)(nil), "proto.TestServer")
}

func init() { proto.RegisterFile("test.proto", fileDescriptor_test_b42f5a1e5f80b8ef) }

var fileDescriptor_test_b42f5a1e5f80b8ef = []byte{
	// 244 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x4c, 0x8f, 0x41, 0x4b, 0x03, 0x31,
	0x14, 0x84, 0xc9, 0x6e, 0x77, 0x8b, 0xaf, 0xa2, 0xf5, 0x81, 0xb8, 0x78, 0x90, 0xd2, 0x8b, 0xf5,
	0xb2, 0x87, 0x7a, 0x29, 0x5e, 0xd5, 0x83, 0x07, 0x2f, 0xab, 0x9e, 0x4b, 0x84, 0x18, 0x82, 0x31,
	0x09, 0x6f, 0xd3, 0x85, 0xfe, 0x6b, 0x7f, 0x82, 0xec, 0x4b, 0xa4, 0x3d, 0x65, 0x98, 0xf9, 0x92,
	0xc9, 0x00, 0x44, 0xd5, 0xc7, 0x36, 0x90, 0x8f, 0x1e, 0x2b, 0x3e, 0x96, 0xbf, 0x02, 0xe0, 0x5d,
	0xf5, 0xf1, 0xd1, 0xbb, 0x2f, 0xa3, 0x71, 0x03, 0xd3, 0x5e, 0xd1, 0xa0, 0xa8, 0x6f, 0xc4, 0xa2,
	0x5c, 0xcd, 0xd6, 0x37, 0x09, 0x6f, 0x0f, 0x4c, 0xfb, 0x96, 0x80, 0x67, 0x17, 0x69, 0xdf, 0xfd,
	0xe3, 0x78, 0x09, 0xb5, 0x09, 0xdb, 0x1d, 0xd9, 0xa6, 0x58, 0x88, 0xd5, 0x49, 0x57, 0x99, 0xf0,
	0x41, 0x16, 0xaf, 0x60, 0x2a, 0xb5, 0x66, 0xbf, 0x64, 0xbf, 0x96, 0x5a, 0x8f, 0xc1, 0x1d, 0xcc,
	0x83, 0xb7, 0xd6, 0x38, 0xbd, 0x35, 0x2e, 0x2a, 0x1a, 0xa4, 0x6d, 0x26, 0x4c, 0x9c, 0x67, 0xff,
	0x25, 0xdb, 0xd7, 0xaf, 0x70, 0x7a, 0xdc, 0x89, 0x73, 0x28, 0xbf, 0xd5, 0xbe, 0x11, 0x4c, 0x8f,
	0x12, 0x6f, 0xa1, 0x1a, 0xa4, 0xdd, 0x29, 0xee, 0x9e, 0xad, 0x2f, 0x8e, 0x3e, 0x9d, 0x6e, 0x76,
	0x29, 0x7f, 0x28, 0x36, 0x62, 0xf9, 0x94, 0x16, 0xa7, 0x00, 0x11, 0x26, 0x4e, 0xfe, 0xa8, 0xfc,
	0x1a, 0x6b, 0x3c, 0x83, 0xc2, 0x84, 0xbc, 0xa3, 0x30, 0x61, 0x64, 0x82, 0xa7, 0xc8, 0x0b, 0xaa,
	0x8e, 0xf5, 0x67, 0xcd, 0x15, 0xf7, 0x7f, 0x01, 0x00, 0x00, 0xff, 0xff, 0xf7, 0xba, 0x11, 0xc9,
	0x54, 0x01, 0x00, 0x00,
}
