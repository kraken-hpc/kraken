// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: cpuburn.proto

package cpuburn

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
	TempSensor           string   `protobuf:"bytes,1,opt,name=temp_sensor,json=tempSensor,proto3" json:"temp_sensor,omitempty"`
	ThermalThrottle      bool     `protobuf:"varint,2,opt,name=thermal_throttle,json=thermalThrottle,proto3" json:"thermal_throttle,omitempty"`
	ThermalPoll          uint32   `protobuf:"varint,3,opt,name=thermal_poll,json=thermalPoll,proto3" json:"thermal_poll,omitempty"`
	ThermalResume        uint32   `protobuf:"varint,4,opt,name=thermal_resume,json=thermalResume,proto3" json:"thermal_resume,omitempty"`
	ThermalCrit          uint32   `protobuf:"varint,5,opt,name=thermal_crit,json=thermalCrit,proto3" json:"thermal_crit,omitempty"`
	Workers              uint32   `protobuf:"varint,6,opt,name=workers,proto3" json:"workers,omitempty"`
	WorkersThrottled     uint32   `protobuf:"varint,7,opt,name=workers_throttled,json=workersThrottled,proto3" json:"workers_throttled,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Config) Reset()         { *m = Config{} }
func (m *Config) String() string { return proto.CompactTextString(m) }
func (*Config) ProtoMessage()    {}
func (*Config) Descriptor() ([]byte, []int) {
	return fileDescriptor_95f28937a9fd0ca5, []int{0}
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

func (m *Config) GetTempSensor() string {
	if m != nil {
		return m.TempSensor
	}
	return ""
}

func (m *Config) GetThermalThrottle() bool {
	if m != nil {
		return m.ThermalThrottle
	}
	return false
}

func (m *Config) GetThermalPoll() uint32 {
	if m != nil {
		return m.ThermalPoll
	}
	return 0
}

func (m *Config) GetThermalResume() uint32 {
	if m != nil {
		return m.ThermalResume
	}
	return 0
}

func (m *Config) GetThermalCrit() uint32 {
	if m != nil {
		return m.ThermalCrit
	}
	return 0
}

func (m *Config) GetWorkers() uint32 {
	if m != nil {
		return m.Workers
	}
	return 0
}

func (m *Config) GetWorkersThrottled() uint32 {
	if m != nil {
		return m.WorkersThrottled
	}
	return 0
}

func init() {
	proto.RegisterType((*Config)(nil), "CPUBurn.Config")
}

func init() { proto.RegisterFile("cpuburn.proto", fileDescriptor_95f28937a9fd0ca5) }

var fileDescriptor_95f28937a9fd0ca5 = []byte{
	// 226 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x54, 0x90, 0xc1, 0x4a, 0xc3, 0x40,
	0x10, 0x86, 0x49, 0xd5, 0xc4, 0x4e, 0x8c, 0xd6, 0x3d, 0xcd, 0xcd, 0x28, 0x08, 0x11, 0xa1, 0x17,
	0x8f, 0xde, 0x9a, 0x17, 0x28, 0x51, 0x2f, 0x5e, 0x8a, 0x6d, 0x57, 0x13, 0xdc, 0x64, 0x96, 0xd9,
	0x59, 0x7c, 0x05, 0x1f, 0x5b, 0x5c, 0x76, 0xc5, 0xde, 0x76, 0xbf, 0xf9, 0xf8, 0xf9, 0xf9, 0xa1,
	0xda, 0x59, 0xbf, 0xf5, 0x3c, 0x2d, 0x2d, 0x93, 0x90, 0x2a, 0xda, 0xf5, 0xcb, 0xca, 0xf3, 0x74,
	0xf3, 0x3d, 0x83, 0xbc, 0xa5, 0xe9, 0x7d, 0xf8, 0x50, 0x57, 0x50, 0x8a, 0x1e, 0xed, 0xc6, 0xe9,
	0xc9, 0x11, 0x63, 0x56, 0x67, 0xcd, 0xbc, 0x83, 0x5f, 0xf4, 0x14, 0x88, 0xba, 0x83, 0x85, 0xf4,
	0x9a, 0xc7, 0x37, 0xb3, 0x91, 0x9e, 0x49, 0xc4, 0x68, 0x9c, 0xd5, 0x59, 0x73, 0xda, 0x5d, 0x44,
	0xfe, 0x1c, 0xb1, 0xba, 0x86, 0xb3, 0xa4, 0x5a, 0x32, 0x06, 0x8f, 0xea, 0xac, 0xa9, 0xba, 0x32,
	0xb2, 0x35, 0x19, 0xa3, 0x6e, 0xe1, 0x3c, 0x29, 0xac, 0x9d, 0x1f, 0x35, 0x1e, 0x07, 0xa9, 0x8a,
	0xb4, 0x0b, 0xf0, 0x7f, 0xd2, 0x8e, 0x07, 0xc1, 0x93, 0x83, 0xa4, 0x96, 0x07, 0x51, 0x08, 0xc5,
	0x17, 0xf1, 0xa7, 0x66, 0x87, 0x79, 0xb8, 0xa6, 0xaf, 0xba, 0x87, 0xcb, 0xf8, 0xfc, 0x6b, 0xbc,
	0xc7, 0x22, 0x38, 0x8b, 0x78, 0x48, 0x95, 0xf7, 0xab, 0xf2, 0x75, 0xbe, 0x7c, 0x8c, 0x33, 0x6d,
	0xf3, 0xb0, 0xd3, 0xc3, 0x4f, 0x00, 0x00, 0x00, 0xff, 0xff, 0xdf, 0x2d, 0x07, 0x65, 0x38, 0x01,
	0x00, 0x00,
}
