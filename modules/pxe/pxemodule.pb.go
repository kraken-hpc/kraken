// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: pxemodule.proto

package pxe

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
	SrvIfaceUrl          string   `protobuf:"bytes,1,opt,name=srv_iface_url,json=srvIfaceUrl,proto3" json:"srv_iface_url,omitempty"`
	SrvIpUrl             string   `protobuf:"bytes,2,opt,name=srv_ip_url,json=srvIpUrl,proto3" json:"srv_ip_url,omitempty"`
	IpUrl                string   `protobuf:"bytes,3,opt,name=ip_url,json=ipUrl,proto3" json:"ip_url,omitempty"`
	NmUrl                string   `protobuf:"bytes,4,opt,name=nm_url,json=nmUrl,proto3" json:"nm_url,omitempty"`
	MacUrl               string   `protobuf:"bytes,5,opt,name=mac_url,json=macUrl,proto3" json:"mac_url,omitempty"`
	SubnetUrl            string   `protobuf:"bytes,6,opt,name=subnet_url,json=subnetUrl,proto3" json:"subnet_url,omitempty"`
	TftpDir              string   `protobuf:"bytes,7,opt,name=tftp_dir,json=tftpDir,proto3" json:"tftp_dir,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Config) Reset()         { *m = Config{} }
func (m *Config) String() string { return proto.CompactTextString(m) }
func (*Config) ProtoMessage()    {}
func (*Config) Descriptor() ([]byte, []int) {
	return fileDescriptor_9cfe76d7f2ebcdfb, []int{0}
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

func (m *Config) GetSrvIfaceUrl() string {
	if m != nil {
		return m.SrvIfaceUrl
	}
	return ""
}

func (m *Config) GetSrvIpUrl() string {
	if m != nil {
		return m.SrvIpUrl
	}
	return ""
}

func (m *Config) GetIpUrl() string {
	if m != nil {
		return m.IpUrl
	}
	return ""
}

func (m *Config) GetNmUrl() string {
	if m != nil {
		return m.NmUrl
	}
	return ""
}

func (m *Config) GetMacUrl() string {
	if m != nil {
		return m.MacUrl
	}
	return ""
}

func (m *Config) GetSubnetUrl() string {
	if m != nil {
		return m.SubnetUrl
	}
	return ""
}

func (m *Config) GetTftpDir() string {
	if m != nil {
		return m.TftpDir
	}
	return ""
}

func init() {
	proto.RegisterType((*Config)(nil), "PXEModule.Config")
}

func init() { proto.RegisterFile("pxemodule.proto", fileDescriptor_9cfe76d7f2ebcdfb) }

var fileDescriptor_9cfe76d7f2ebcdfb = []byte{
	// 203 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x34, 0xcf, 0xbb, 0x8a, 0xc2, 0x40,
	0x14, 0xc6, 0x71, 0xb2, 0xbb, 0x99, 0x49, 0xce, 0xb2, 0x2c, 0x04, 0xc4, 0x08, 0x0a, 0x92, 0xca,
	0x2a, 0x8d, 0xa5, 0x9d, 0x97, 0xc2, 0x42, 0x10, 0x41, 0x10, 0x9b, 0x90, 0xcb, 0x44, 0x06, 0x32,
	0x17, 0x26, 0x93, 0x90, 0x77, 0xf4, 0xa5, 0x64, 0x4e, 0xb4, 0x3c, 0xbf, 0xff, 0xd7, 0x1c, 0xf8,
	0xd7, 0x03, 0x13, 0xaa, 0xea, 0x1a, 0x96, 0x6a, 0xa3, 0xac, 0x8a, 0xc2, 0xf3, 0xed, 0x70, 0x42,
	0x48, 0x9e, 0x1e, 0x90, 0x9d, 0x92, 0x35, 0x7f, 0x44, 0x09, 0xfc, 0xb5, 0xa6, 0xcf, 0x78, 0x9d,
	0x97, 0x2c, 0xeb, 0x4c, 0x13, 0x7b, 0x4b, 0x6f, 0x15, 0x5e, 0x7e, 0x5b, 0xd3, 0x1f, 0x9d, 0x5d,
	0x4d, 0x13, 0xcd, 0x01, 0x70, 0xa3, 0x71, 0xf0, 0x85, 0x83, 0xc0, 0x0d, 0xb4, 0xab, 0x13, 0x20,
	0xef, 0xf2, 0x8d, 0xc5, 0xe7, 0x1f, 0x96, 0x02, 0xf9, 0x67, 0x64, 0x29, 0x1c, 0x4f, 0x81, 0x8a,
	0xbc, 0x44, 0xf7, 0xd1, 0x89, 0xc8, 0x4b, 0x17, 0x16, 0x00, 0x6d, 0x57, 0x48, 0x66, 0xb1, 0x11,
	0x6c, 0xe1, 0x28, 0x2e, 0xcf, 0x20, 0xb0, 0xb5, 0xd5, 0x59, 0xc5, 0x4d, 0x4c, 0x31, 0x52, 0x77,
	0xef, 0xb9, 0xd9, 0xd2, 0xbb, 0x9f, 0x6e, 0xf4, 0xc0, 0x0a, 0x82, 0x8f, 0xae, 0x5f, 0x01, 0x00,
	0x00, 0xff, 0xff, 0xd1, 0x6b, 0x75, 0x3a, 0xfb, 0x00, 0x00, 0x00,
}
