// Code generated by protoc-gen-go. DO NOT EDIT.
// source: PXE.proto

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

type PXE_Method int32

const (
	PXE_PXE  PXE_Method = 0
	PXE_iPXE PXE_Method = 1
)

var PXE_Method_name = map[int32]string{
	0: "PXE",
	1: "iPXE",
}
var PXE_Method_value = map[string]int32{
	"PXE":  0,
	"iPXE": 1,
}

func (x PXE_Method) String() string {
	return proto.EnumName(PXE_Method_name, int32(x))
}
func (PXE_Method) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_PXE_4f878577b5d74e50, []int{0, 0}
}

type PXE_State int32

const (
	PXE_NONE PXE_State = 0
	PXE_WAIT PXE_State = 1
	PXE_INIT PXE_State = 2
	PXE_COMP PXE_State = 3
)

var PXE_State_name = map[int32]string{
	0: "NONE",
	1: "WAIT",
	2: "INIT",
	3: "COMP",
}
var PXE_State_value = map[string]int32{
	"NONE": 0,
	"WAIT": 1,
	"INIT": 2,
	"COMP": 3,
}

func (x PXE_State) String() string {
	return proto.EnumName(PXE_State_name, int32(x))
}
func (PXE_State) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_PXE_4f878577b5d74e50, []int{0, 1}
}

type PXE struct {
	Method               PXE_Method `protobuf:"varint,1,opt,name=method,proto3,enum=proto.PXE_Method" json:"method,omitempty"`
	State                PXE_State  `protobuf:"varint,2,opt,name=state,proto3,enum=proto.PXE_State" json:"state,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *PXE) Reset()         { *m = PXE{} }
func (m *PXE) String() string { return proto.CompactTextString(m) }
func (*PXE) ProtoMessage()    {}
func (*PXE) Descriptor() ([]byte, []int) {
	return fileDescriptor_PXE_4f878577b5d74e50, []int{0}
}
func (m *PXE) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PXE.Unmarshal(m, b)
}
func (m *PXE) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PXE.Marshal(b, m, deterministic)
}
func (dst *PXE) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PXE.Merge(dst, src)
}
func (m *PXE) XXX_Size() int {
	return xxx_messageInfo_PXE.Size(m)
}
func (m *PXE) XXX_DiscardUnknown() {
	xxx_messageInfo_PXE.DiscardUnknown(m)
}

var xxx_messageInfo_PXE proto.InternalMessageInfo

func (m *PXE) GetMethod() PXE_Method {
	if m != nil {
		return m.Method
	}
	return PXE_PXE
}

func (m *PXE) GetState() PXE_State {
	if m != nil {
		return m.State
	}
	return PXE_NONE
}

func init() {
	proto.RegisterType((*PXE)(nil), "proto.PXE")
	proto.RegisterEnum("proto.PXE_Method", PXE_Method_name, PXE_Method_value)
	proto.RegisterEnum("proto.PXE_State", PXE_State_name, PXE_State_value)
}

func init() { proto.RegisterFile("PXE.proto", fileDescriptor_PXE_4f878577b5d74e50) }

var fileDescriptor_PXE_4f878577b5d74e50 = []byte{
	// 157 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x0c, 0x88, 0x70, 0xd5,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x05, 0x53, 0x4a, 0xcb, 0x18, 0xb9, 0x98, 0x03, 0x22,
	0x5c, 0x85, 0x34, 0xb9, 0xd8, 0x72, 0x53, 0x4b, 0x32, 0xf2, 0x53, 0x24, 0x18, 0x15, 0x18, 0x35,
	0xf8, 0x8c, 0x04, 0x21, 0xca, 0xf4, 0x40, 0x1a, 0x7c, 0xc1, 0x12, 0x41, 0x50, 0x05, 0x42, 0x6a,
	0x5c, 0xac, 0xc5, 0x25, 0x89, 0x25, 0xa9, 0x12, 0x4c, 0x60, 0x95, 0x02, 0x48, 0x2a, 0x83, 0x41,
	0xe2, 0x41, 0x10, 0x69, 0x25, 0x69, 0x2e, 0x36, 0x88, 0x4e, 0x21, 0x76, 0xb0, 0x1d, 0x02, 0x0c,
	0x42, 0x1c, 0x5c, 0x2c, 0x99, 0x20, 0x16, 0xa3, 0x92, 0x3e, 0x17, 0x2b, 0x58, 0x31, 0x48, 0xc8,
	0xcf, 0xdf, 0x0f, 0x2a, 0x19, 0xee, 0xe8, 0x19, 0x22, 0xc0, 0x08, 0x62, 0x79, 0xfa, 0x79, 0x86,
	0x08, 0x30, 0x81, 0x58, 0xce, 0xfe, 0xbe, 0x01, 0x02, 0xcc, 0x49, 0x6c, 0x60, 0x5b, 0x8c, 0x01,
	0x01, 0x00, 0x00, 0xff, 0xff, 0xb7, 0x30, 0xeb, 0x00, 0xc3, 0x00, 0x00, 0x00,
}
