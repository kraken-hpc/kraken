// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: vboxmanage.proto

package vboxmanage

import (
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type Config struct {
	Servers              map[string]*Server `protobuf:"bytes,1,rep,name=servers,proto3" json:"servers,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	PollingInterval      string             `protobuf:"bytes,2,opt,name=polling_interval,json=pollingInterval,proto3" json:"polling_interval,omitempty"`
	NameUrl              string             `protobuf:"bytes,3,opt,name=name_url,json=nameUrl,proto3" json:"name_url,omitempty"`
	ServerUrl            string             `protobuf:"bytes,4,opt,name=server_url,json=serverUrl,proto3" json:"server_url,omitempty"`
	UuidUrl              string             `protobuf:"bytes,5,opt,name=uuid_url,json=uuidUrl,proto3" json:"uuid_url,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *Config) Reset()         { *m = Config{} }
func (m *Config) String() string { return proto.CompactTextString(m) }
func (*Config) ProtoMessage()    {}
func (*Config) Descriptor() ([]byte, []int) {
	return fileDescriptor_96b0aa3846a76f79, []int{0}
}
func (m *Config) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Config.Unmarshal(m, b)
}
func (m *Config) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Config.Marshal(b, m, deterministic)
}
func (m *Config) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Config.Merge(m, src)
}
func (m *Config) XXX_Size() int {
	return xxx_messageInfo_Config.Size(m)
}
func (m *Config) XXX_DiscardUnknown() {
	xxx_messageInfo_Config.DiscardUnknown(m)
}

var xxx_messageInfo_Config proto.InternalMessageInfo

func (m *Config) GetServers() map[string]*Server {
	if m != nil {
		return m.Servers
	}
	return nil
}

func (m *Config) GetPollingInterval() string {
	if m != nil {
		return m.PollingInterval
	}
	return ""
}

func (m *Config) GetNameUrl() string {
	if m != nil {
		return m.NameUrl
	}
	return ""
}

func (m *Config) GetServerUrl() string {
	if m != nil {
		return m.ServerUrl
	}
	return ""
}

func (m *Config) GetUuidUrl() string {
	if m != nil {
		return m.UuidUrl
	}
	return ""
}

type Server struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Ip                   string   `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
	Port                 int32    `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Server) Reset()         { *m = Server{} }
func (m *Server) String() string { return proto.CompactTextString(m) }
func (*Server) ProtoMessage()    {}
func (*Server) Descriptor() ([]byte, []int) {
	return fileDescriptor_96b0aa3846a76f79, []int{1}
}
func (m *Server) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Server.Unmarshal(m, b)
}
func (m *Server) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Server.Marshal(b, m, deterministic)
}
func (m *Server) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Server.Merge(m, src)
}
func (m *Server) XXX_Size() int {
	return xxx_messageInfo_Server.Size(m)
}
func (m *Server) XXX_DiscardUnknown() {
	xxx_messageInfo_Server.DiscardUnknown(m)
}

var xxx_messageInfo_Server proto.InternalMessageInfo

func (m *Server) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Server) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *Server) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func init() {
	proto.RegisterType((*Config)(nil), "VBoxManage.Config")
	proto.RegisterMapType((map[string]*Server)(nil), "VBoxManage.Config.ServersEntry")
	proto.RegisterType((*Server)(nil), "VBoxManage.Server")
}

func init() { proto.RegisterFile("vboxmanage.proto", fileDescriptor_96b0aa3846a76f79) }

var fileDescriptor_96b0aa3846a76f79 = []byte{
	// 276 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x4c, 0x90, 0x3f, 0x6f, 0xc2, 0x30,
	0x10, 0xc5, 0x95, 0x84, 0x04, 0x38, 0x10, 0x8d, 0x6e, 0x0a, 0x95, 0xaa, 0x22, 0xa6, 0x74, 0xc9,
	0x40, 0x97, 0xfe, 0x59, 0x2a, 0xaa, 0x0e, 0x1d, 0xda, 0x21, 0x15, 0x1d, 0xba, 0x20, 0xa3, 0xba,
	0x91, 0x55, 0x63, 0x5b, 0x26, 0x89, 0xe0, 0x73, 0xf4, 0x0b, 0x57, 0x3e, 0x07, 0xc1, 0x76, 0xf7,
	0xde, 0xef, 0x4e, 0xf7, 0x0e, 0xd2, 0x76, 0xa3, 0xf7, 0x5b, 0xa6, 0x58, 0xc5, 0x0b, 0x63, 0x75,
	0xad, 0x11, 0x3e, 0x97, 0x7a, 0xff, 0x46, 0xca, 0xfc, 0x2f, 0x84, 0xe4, 0x59, 0xab, 0x1f, 0x51,
	0xe1, 0x3d, 0xf4, 0x77, 0xdc, 0xb6, 0xdc, 0xee, 0xb2, 0x60, 0x16, 0xe5, 0xa3, 0xc5, 0x75, 0x71,
	0x02, 0x0b, 0x0f, 0x15, 0x1f, 0x9e, 0x78, 0x51, 0xb5, 0x3d, 0x94, 0x47, 0x1e, 0x6f, 0x20, 0x35,
	0x5a, 0x4a, 0xa1, 0xaa, 0xb5, 0x50, 0x35, 0xb7, 0x2d, 0x93, 0x59, 0x38, 0x0b, 0xf2, 0x61, 0x79,
	0xd1, 0xe9, 0xaf, 0x9d, 0x8c, 0x53, 0x18, 0x28, 0xb6, 0xe5, 0xeb, 0xc6, 0xca, 0x2c, 0x22, 0xa4,
	0xef, 0xfa, 0x95, 0x95, 0x78, 0x05, 0xe0, 0x17, 0x92, 0xd9, 0x23, 0x73, 0xe8, 0x15, 0x67, 0x4f,
	0x61, 0xd0, 0x34, 0xe2, 0x9b, 0xcc, 0xd8, 0x4f, 0xba, 0x7e, 0x65, 0xe5, 0xe5, 0x3b, 0x8c, 0xcf,
	0x0f, 0xc3, 0x14, 0xa2, 0x5f, 0x7e, 0xc8, 0x02, 0xa2, 0x5c, 0x89, 0x39, 0xc4, 0x2d, 0x93, 0x0d,
	0xa7, 0xb3, 0x46, 0x0b, 0x3c, 0x8f, 0xe6, 0x47, 0x4b, 0x0f, 0x3c, 0x84, 0x77, 0xc1, 0xfc, 0x09,
	0x12, 0x2f, 0x22, 0x42, 0xcf, 0x9d, 0xd7, 0xad, 0xa2, 0x1a, 0x27, 0x10, 0x0a, 0xd3, 0xe5, 0x0b,
	0x85, 0x71, 0x8c, 0xd1, 0xb6, 0xa6, 0x38, 0x71, 0x49, 0xf5, 0x72, 0xf2, 0x35, 0x2e, 0x1e, 0x4f,
	0x9f, 0xdf, 0x24, 0xf4, 0xfa, 0xdb, 0xff, 0x00, 0x00, 0x00, 0xff, 0xff, 0x85, 0xbc, 0x1f, 0x0f,
	0x8e, 0x01, 0x00, 0x00,
}
