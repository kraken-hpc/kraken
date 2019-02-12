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
	NodeNames            []string `protobuf:"bytes,1,rep,name=nodeNames,proto3" json:"nodeNames,omitempty"`
	PollingInterval      string   `protobuf:"bytes,2,opt,name=polling_interval,json=pollingInterval,proto3" json:"polling_interval,omitempty"`
	NameUrl              string   `protobuf:"bytes,3,opt,name=name_url,json=nameUrl,proto3" json:"name_url,omitempty"`
	ServerUrl            string   `protobuf:"bytes,4,opt,name=server_url,json=serverUrl,proto3" json:"server_url,omitempty"`
	UuidUrl              string   `protobuf:"bytes,5,opt,name=uuid_url,json=uuidUrl,proto3" json:"uuid_url,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
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

func (m *PMCConfig) GetNodeNames() []string {
	if m != nil {
		return m.NodeNames
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

func (m *PMCConfig) GetUuidUrl() string {
	if m != nil {
		return m.UuidUrl
	}
	return ""
}

func init() {
	proto.RegisterType((*PMCConfig)(nil), "proto.PMCConfig")
}

func init() { proto.RegisterFile("powermancontrol.proto", fileDescriptor_555afd2db12d586c) }

var fileDescriptor_555afd2db12d586c = []byte{
	// 183 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x2d, 0xc8, 0x2f, 0x4f,
	0x2d, 0xca, 0x4d, 0xcc, 0x4b, 0xce, 0xcf, 0x2b, 0x29, 0xca, 0xcf, 0xd1, 0x2b, 0x28, 0xca, 0x2f,
	0xc9, 0x17, 0x62, 0x05, 0x53, 0x4a, 0x2b, 0x19, 0xb9, 0x38, 0x03, 0x7c, 0x9d, 0x9d, 0xf3, 0xf3,
	0xd2, 0x32, 0xd3, 0x85, 0x64, 0xb8, 0x38, 0xf3, 0xf2, 0x53, 0x52, 0xfd, 0x12, 0x73, 0x53, 0x8b,
	0x25, 0x18, 0x15, 0x98, 0x35, 0x38, 0x83, 0x10, 0x02, 0x42, 0x9a, 0x5c, 0x02, 0x05, 0xf9, 0x39,
	0x39, 0x99, 0x79, 0xe9, 0xf1, 0x99, 0x79, 0x25, 0xa9, 0x45, 0x65, 0x89, 0x39, 0x12, 0x4c, 0x0a,
	0x8c, 0x1a, 0x9c, 0x41, 0xfc, 0x50, 0x71, 0x4f, 0xa8, 0xb0, 0x90, 0x24, 0x17, 0x47, 0x5e, 0x62,
	0x6e, 0x6a, 0x7c, 0x69, 0x51, 0x8e, 0x04, 0x33, 0x58, 0x09, 0x3b, 0x88, 0x1f, 0x5a, 0x94, 0x23,
	0x24, 0xcb, 0xc5, 0x55, 0x9c, 0x5a, 0x54, 0x96, 0x5a, 0x04, 0x96, 0x64, 0x01, 0x4b, 0x72, 0x42,
	0x44, 0x40, 0xd2, 0x92, 0x5c, 0x1c, 0xa5, 0xa5, 0x99, 0x29, 0x60, 0x49, 0x56, 0x88, 0x4e, 0x10,
	0x3f, 0xb4, 0x28, 0x27, 0x89, 0x0d, 0xec, 0x64, 0x63, 0x40, 0x00, 0x00, 0x00, 0xff, 0xff, 0xf1,
	0x67, 0xe1, 0x6b, 0xd2, 0x00, 0x00, 0x00,
}
