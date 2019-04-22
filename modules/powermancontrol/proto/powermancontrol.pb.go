// Code generated by protoc-gen-go. DO NOT EDIT.
// source: powermancontrol.proto

package proto

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type PMCConfig struct {
	Servers              map[string]*PMCServer `protobuf:"bytes,1,rep,name=servers,proto3" json:"servers,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	PollingInterval      string                `protobuf:"bytes,2,opt,name=polling_interval,json=pollingInterval,proto3" json:"polling_interval,omitempty"`
	NameUrl              string                `protobuf:"bytes,3,opt,name=name_url,json=nameUrl,proto3" json:"name_url,omitempty"`
	ServerUrl            string                `protobuf:"bytes,4,opt,name=server_url,json=serverUrl,proto3" json:"server_url,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *PMCConfig) Reset()         { *m = PMCConfig{} }
func (m *PMCConfig) String() string { return proto.CompactTextString(m) }
func (*PMCConfig) ProtoMessage()    {}
func (*PMCConfig) Descriptor() ([]byte, []int) {
	return fileDescriptor_555afd2db12d586c, []int{0}
}

func (m *PMCConfig) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PMCConfig.Unmarshal(m, b)
}
func (m *PMCConfig) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PMCConfig.Marshal(b, m, deterministic)
}
func (m *PMCConfig) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PMCConfig.Merge(m, src)
}
func (m *PMCConfig) XXX_Size() int {
	return xxx_messageInfo_PMCConfig.Size(m)
}
func (m *PMCConfig) XXX_DiscardUnknown() {
	xxx_messageInfo_PMCConfig.DiscardUnknown(m)
}

var xxx_messageInfo_PMCConfig proto.InternalMessageInfo

func (m *PMCConfig) GetServers() map[string]*PMCServer {
	if m != nil {
		return m.Servers
	}
	return nil
}

func (m *PMCConfig) GetPollingInterval() string {
	if m != nil {
		return m.PollingInterval
	}
	return ""
}

func (m *PMCConfig) GetNameUrl() string {
	if m != nil {
		return m.NameUrl
	}
	return ""
}

func (m *PMCConfig) GetServerUrl() string {
	if m != nil {
		return m.ServerUrl
	}
	return ""
}

type PMCServer struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Ip                   string   `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
	Port                 int32    `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PMCServer) Reset()         { *m = PMCServer{} }
func (m *PMCServer) String() string { return proto.CompactTextString(m) }
func (*PMCServer) ProtoMessage()    {}
func (*PMCServer) Descriptor() ([]byte, []int) {
	return fileDescriptor_555afd2db12d586c, []int{1}
}

func (m *PMCServer) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PMCServer.Unmarshal(m, b)
}
func (m *PMCServer) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PMCServer.Marshal(b, m, deterministic)
}
func (m *PMCServer) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PMCServer.Merge(m, src)
}
func (m *PMCServer) XXX_Size() int {
	return xxx_messageInfo_PMCServer.Size(m)
}
func (m *PMCServer) XXX_DiscardUnknown() {
	xxx_messageInfo_PMCServer.DiscardUnknown(m)
}

var xxx_messageInfo_PMCServer proto.InternalMessageInfo

func (m *PMCServer) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *PMCServer) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *PMCServer) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func init() {
	proto.RegisterType((*PMCConfig)(nil), "proto.PMCConfig")
	proto.RegisterMapType((map[string]*PMCServer)(nil), "proto.PMCConfig.ServersEntry")
	proto.RegisterType((*PMCServer)(nil), "proto.PMCServer")
}

func init() { proto.RegisterFile("powermancontrol.proto", fileDescriptor_555afd2db12d586c) }

var fileDescriptor_555afd2db12d586c = []byte{
	// 254 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x4c, 0x8f, 0x41, 0x4b, 0xc3, 0x40,
	0x10, 0x85, 0xd9, 0xa4, 0xb1, 0x66, 0x2a, 0x1a, 0x06, 0x84, 0x28, 0x14, 0x4a, 0x0f, 0x52, 0x2f,
	0x39, 0xd4, 0x83, 0xe2, 0x35, 0x78, 0x10, 0x14, 0x24, 0xd2, 0x73, 0x89, 0xb2, 0x96, 0xc5, 0xed,
	0xee, 0x32, 0xdd, 0x46, 0xf2, 0xcb, 0xbd, 0x4a, 0x66, 0x57, 0xed, 0x29, 0x93, 0xf7, 0xbe, 0x9d,
	0xf7, 0x06, 0xce, 0x9d, 0xfd, 0x92, 0xb4, 0x6d, 0xcd, 0xbb, 0x35, 0x9e, 0xac, 0xae, 0x1c, 0x59,
	0x6f, 0x31, 0xe3, 0xcf, 0xfc, 0x5b, 0x40, 0xfe, 0xf2, 0x5c, 0xd7, 0xd6, 0x7c, 0xa8, 0x0d, 0xde,
	0xc2, 0x78, 0x27, 0xa9, 0x93, 0xb4, 0x2b, 0xc5, 0x2c, 0x5d, 0x4c, 0x96, 0xd3, 0x40, 0x57, 0x7f,
	0x48, 0xf5, 0x1a, 0xfc, 0x07, 0xe3, 0xa9, 0x6f, 0x7e, 0x69, 0xbc, 0x86, 0xc2, 0x59, 0xad, 0x95,
	0xd9, 0xac, 0x95, 0xf1, 0x92, 0xba, 0x56, 0x97, 0xc9, 0x4c, 0x2c, 0xf2, 0xe6, 0x2c, 0xea, 0x8f,
	0x51, 0xc6, 0x0b, 0x38, 0x36, 0xed, 0x56, 0xae, 0xf7, 0xa4, 0xcb, 0x94, 0x91, 0xf1, 0xf0, 0xbf,
	0x22, 0x8d, 0x53, 0x80, 0xb0, 0x90, 0xcd, 0x11, 0x9b, 0x79, 0x50, 0x56, 0xa4, 0x2f, 0x9f, 0xe0,
	0xe4, 0x30, 0x1d, 0x0b, 0x48, 0x3f, 0x65, 0x5f, 0x0a, 0xe6, 0x86, 0x11, 0xaf, 0x20, 0xeb, 0x5a,
	0xbd, 0x97, 0x9c, 0x3d, 0x59, 0x16, 0xff, 0xed, 0xc3, 0xc3, 0x26, 0xd8, 0xf7, 0xc9, 0x9d, 0x98,
	0xd7, 0x7c, 0x78, 0xd0, 0x11, 0x61, 0x34, 0x94, 0x88, 0xbb, 0x78, 0xc6, 0x53, 0x48, 0x94, 0x8b,
	0x57, 0x24, 0xca, 0x0d, 0x8c, 0xb3, 0xe4, 0xb9, 0x74, 0xd6, 0xf0, 0xfc, 0x76, 0xc4, 0x01, 0x37,
	0x3f, 0x01, 0x00, 0x00, 0xff, 0xff, 0xc6, 0x26, 0x27, 0xe1, 0x65, 0x01, 0x00, 0x00,
}
