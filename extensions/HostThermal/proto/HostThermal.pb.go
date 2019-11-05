// Code generated by protoc-gen-go. DO NOT EDIT.
// source: HostThermal.proto

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

type HostThermal_CPU_TEMP_STATE int32

const (
	HostThermal_CPU_TEMP_NONE     HostThermal_CPU_TEMP_STATE = 0
	HostThermal_CPU_TEMP_NORMAL   HostThermal_CPU_TEMP_STATE = 1
	HostThermal_CPU_TEMP_HIGH     HostThermal_CPU_TEMP_STATE = 2
	HostThermal_CPU_TEMP_CRITICAL HostThermal_CPU_TEMP_STATE = 3
)

var HostThermal_CPU_TEMP_STATE_name = map[int32]string{
	0: "CPU_TEMP_NONE",
	1: "CPU_TEMP_NORMAL",
	2: "CPU_TEMP_HIGH",
	3: "CPU_TEMP_CRITICAL",
}
var HostThermal_CPU_TEMP_STATE_value = map[string]int32{
	"CPU_TEMP_NONE":     0,
	"CPU_TEMP_NORMAL":   1,
	"CPU_TEMP_HIGH":     2,
	"CPU_TEMP_CRITICAL": 3,
}

func (x HostThermal_CPU_TEMP_STATE) String() string {
	return proto.EnumName(HostThermal_CPU_TEMP_STATE_name, int32(x))
}
func (HostThermal_CPU_TEMP_STATE) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_HostThermal_bcd73781475c578b, []int{0, 0}
}

type HostThermal struct {
	State                HostThermal_CPU_TEMP_STATE `protobuf:"varint,1,opt,name=state,proto3,enum=proto.HostThermal_CPU_TEMP_STATE" json:"state,omitempty"`
	CPU_Temperature      uint32                     `protobuf:"varint,2,opt,name=CPU_Temperature,json=CPUTemperature,proto3" json:"CPU_Temperature,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                   `json:"-"`
	XXX_unrecognized     []byte                     `json:"-"`
	XXX_sizecache        int32                      `json:"-"`
}

func (m *HostThermal) Reset()         { *m = HostThermal{} }
func (m *HostThermal) String() string { return proto.CompactTextString(m) }
func (*HostThermal) ProtoMessage()    {}
func (*HostThermal) Descriptor() ([]byte, []int) {
	return fileDescriptor_HostThermal_bcd73781475c578b, []int{0}
}
func (m *HostThermal) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_HostThermal.Unmarshal(m, b)
}
func (m *HostThermal) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_HostThermal.Marshal(b, m, deterministic)
}
func (dst *HostThermal) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HostThermal.Merge(dst, src)
}
func (m *HostThermal) XXX_Size() int {
	return xxx_messageInfo_HostThermal.Size(m)
}
func (m *HostThermal) XXX_DiscardUnknown() {
	xxx_messageInfo_HostThermal.DiscardUnknown(m)
}

var xxx_messageInfo_HostThermal proto.InternalMessageInfo

func (m *HostThermal) GetState() HostThermal_CPU_TEMP_STATE {
	if m != nil {
		return m.State
	}
	return HostThermal_CPU_TEMP_NONE
}

func (m *HostThermal) GetCPU_Temperature() uint32 {
	if m != nil {
		return m.CPU_Temperature
	}
	return 0
}

func init() {
	proto.RegisterType((*HostThermal)(nil), "proto.HostThermal")
	proto.RegisterEnum("proto.HostThermal_CPU_TEMP_STATE", HostThermal_CPU_TEMP_STATE_name, HostThermal_CPU_TEMP_STATE_value)
}

func init() { proto.RegisterFile("HostThermal.proto", fileDescriptor_HostThermal_bcd73781475c578b) }

var fileDescriptor_HostThermal_bcd73781475c578b = []byte{
	// 181 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0xf4, 0xc8, 0x2f, 0x2e,
	0x09, 0xc9, 0x48, 0x2d, 0xca, 0x4d, 0xcc, 0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x05,
	0x53, 0x4a, 0x97, 0x19, 0xb9, 0xb8, 0x91, 0x24, 0x85, 0xcc, 0xb9, 0x58, 0x8b, 0x4b, 0x12, 0x4b,
	0x52, 0x25, 0x18, 0x15, 0x18, 0x35, 0xf8, 0x8c, 0x14, 0x21, 0xaa, 0xf5, 0x90, 0xf5, 0x3b, 0x07,
	0x84, 0xc6, 0x87, 0xb8, 0xfa, 0x06, 0xc4, 0x07, 0x87, 0x38, 0x86, 0xb8, 0x06, 0x41, 0xd4, 0x0b,
	0xa9, 0x73, 0xf1, 0x83, 0x25, 0x52, 0x73, 0x0b, 0x52, 0x8b, 0x12, 0x4b, 0x4a, 0x8b, 0x52, 0x25,
	0x98, 0x14, 0x18, 0x35, 0x78, 0x83, 0xf8, 0x9c, 0x03, 0x42, 0x91, 0x44, 0x95, 0x92, 0xb8, 0xf8,
	0x50, 0x4d, 0x10, 0x12, 0xe4, 0xe2, 0x85, 0x8b, 0xf8, 0xf9, 0xfb, 0xb9, 0x0a, 0x30, 0x08, 0x09,
	0x43, 0x4d, 0x83, 0x08, 0x05, 0xf9, 0x3a, 0xfa, 0x08, 0x30, 0xa2, 0xa8, 0xf3, 0xf0, 0x74, 0xf7,
	0x10, 0x60, 0x12, 0x12, 0xe5, 0x12, 0x84, 0x0b, 0x39, 0x07, 0x79, 0x86, 0x78, 0x3a, 0x3b, 0xfa,
	0x08, 0x30, 0x27, 0xb1, 0x81, 0x5d, 0x6d, 0x0c, 0x08, 0x00, 0x00, 0xff, 0xff, 0x18, 0x3c, 0x61,
	0x00, 0xf8, 0x00, 0x00, 0x00,
}
