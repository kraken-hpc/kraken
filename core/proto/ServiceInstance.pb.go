// Code generated by protoc-gen-go. DO NOT EDIT.
// source: ServiceInstance.proto

package proto

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import any "github.com/golang/protobuf/ptypes/any"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type ServiceInstance_ServiceState int32

const (
	ServiceInstance_UNKNOWN ServiceInstance_ServiceState = 0
	ServiceInstance_INIT    ServiceInstance_ServiceState = 1
	ServiceInstance_STOP    ServiceInstance_ServiceState = 2
	ServiceInstance_RUN     ServiceInstance_ServiceState = 3
	ServiceInstance_ERROR   ServiceInstance_ServiceState = 4
)

var ServiceInstance_ServiceState_name = map[int32]string{
	0: "UNKNOWN",
	1: "INIT",
	2: "STOP",
	3: "RUN",
	4: "ERROR",
}
var ServiceInstance_ServiceState_value = map[string]int32{
	"UNKNOWN": 0,
	"INIT":    1,
	"STOP":    2,
	"RUN":     3,
	"ERROR":   4,
}

func (x ServiceInstance_ServiceState) String() string {
	return proto.EnumName(ServiceInstance_ServiceState_name, int32(x))
}
func (ServiceInstance_ServiceState) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_ServiceInstance_6e8ce0a08d2daa01, []int{0, 0}
}

type ServiceInstance struct {
	Id                   string                       `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Module               string                       `protobuf:"bytes,2,opt,name=module,proto3" json:"module,omitempty"`
	State                ServiceInstance_ServiceState `protobuf:"varint,3,opt,name=state,proto3,enum=proto.ServiceInstance_ServiceState" json:"state,omitempty"`
	Config               *any.Any                     `protobuf:"bytes,4,opt,name=config,proto3" json:"config,omitempty"`
	ErrorMsg             string                       `protobuf:"bytes,5,opt,name=error_msg,json=errorMsg,proto3" json:"error_msg,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                     `json:"-"`
	XXX_unrecognized     []byte                       `json:"-"`
	XXX_sizecache        int32                        `json:"-"`
}

func (m *ServiceInstance) Reset()         { *m = ServiceInstance{} }
func (m *ServiceInstance) String() string { return proto.CompactTextString(m) }
func (*ServiceInstance) ProtoMessage()    {}
func (*ServiceInstance) Descriptor() ([]byte, []int) {
	return fileDescriptor_ServiceInstance_6e8ce0a08d2daa01, []int{0}
}
func (m *ServiceInstance) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ServiceInstance.Unmarshal(m, b)
}
func (m *ServiceInstance) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ServiceInstance.Marshal(b, m, deterministic)
}
func (dst *ServiceInstance) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ServiceInstance.Merge(dst, src)
}
func (m *ServiceInstance) XXX_Size() int {
	return xxx_messageInfo_ServiceInstance.Size(m)
}
func (m *ServiceInstance) XXX_DiscardUnknown() {
	xxx_messageInfo_ServiceInstance.DiscardUnknown(m)
}

var xxx_messageInfo_ServiceInstance proto.InternalMessageInfo

func (m *ServiceInstance) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *ServiceInstance) GetModule() string {
	if m != nil {
		return m.Module
	}
	return ""
}

func (m *ServiceInstance) GetState() ServiceInstance_ServiceState {
	if m != nil {
		return m.State
	}
	return ServiceInstance_UNKNOWN
}

func (m *ServiceInstance) GetConfig() *any.Any {
	if m != nil {
		return m.Config
	}
	return nil
}

func (m *ServiceInstance) GetErrorMsg() string {
	if m != nil {
		return m.ErrorMsg
	}
	return ""
}

func init() {
	proto.RegisterType((*ServiceInstance)(nil), "proto.ServiceInstance")
	proto.RegisterEnum("proto.ServiceInstance_ServiceState", ServiceInstance_ServiceState_name, ServiceInstance_ServiceState_value)
}

func init() {
	proto.RegisterFile("ServiceInstance.proto", fileDescriptor_ServiceInstance_6e8ce0a08d2daa01)
}

var fileDescriptor_ServiceInstance_6e8ce0a08d2daa01 = []byte{
	// 256 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x5c, 0x8e, 0xcf, 0x4b, 0xfb, 0x40,
	0x14, 0xc4, 0xbf, 0xf9, 0xd9, 0xe6, 0xf5, 0x4b, 0x0d, 0x0f, 0x95, 0x55, 0x2f, 0xa1, 0x5e, 0x72,
	0x90, 0x2d, 0xd4, 0x93, 0x47, 0x11, 0x0f, 0x41, 0xdc, 0xc8, 0xa6, 0xc5, 0xa3, 0xa4, 0xc9, 0x76,
	0x09, 0xb4, 0xbb, 0xb2, 0x49, 0x85, 0xde, 0xfd, 0xc3, 0xa5, 0x9b, 0x08, 0xd2, 0xd3, 0xee, 0x0c,
	0x33, 0x6f, 0x3e, 0x70, 0x51, 0x08, 0xf3, 0xd5, 0x54, 0x22, 0x53, 0x6d, 0x57, 0xaa, 0x4a, 0xd0,
	0x4f, 0xa3, 0x3b, 0x8d, 0x81, 0x7d, 0xae, 0xaf, 0xa4, 0xd6, 0x72, 0x2b, 0xe6, 0x56, 0xad, 0xf7,
	0x9b, 0x79, 0xa9, 0x0e, 0x7d, 0x62, 0xf6, 0xed, 0xc2, 0xd9, 0x49, 0x17, 0xa7, 0xe0, 0x36, 0x35,
	0x71, 0x12, 0x27, 0x8d, 0xb8, 0xdb, 0xd4, 0x78, 0x09, 0xe1, 0x4e, 0xd7, 0xfb, 0xad, 0x20, 0xae,
	0xf5, 0x06, 0x85, 0x0f, 0x10, 0xb4, 0x5d, 0xd9, 0x09, 0xe2, 0x25, 0x4e, 0x3a, 0x5d, 0xdc, 0xf6,
	0x27, 0xe9, 0x29, 0xca, 0xa0, 0x8b, 0x63, 0x94, 0xf7, 0x0d, 0xbc, 0x83, 0xb0, 0xd2, 0x6a, 0xd3,
	0x48, 0xe2, 0x27, 0x4e, 0x3a, 0x59, 0x9c, 0xd3, 0x1e, 0x91, 0xfe, 0x22, 0xd2, 0x47, 0x75, 0xe0,
	0x43, 0x06, 0x6f, 0x20, 0x12, 0xc6, 0x68, 0xf3, 0xb1, 0x6b, 0x25, 0x09, 0x2c, 0xc3, 0xd8, 0x1a,
	0xaf, 0xad, 0x9c, 0x3d, 0xc1, 0xff, 0xbf, 0x0b, 0x38, 0x81, 0xd1, 0x8a, 0xbd, 0xb0, 0xfc, 0x9d,
	0xc5, 0xff, 0x70, 0x0c, 0x7e, 0xc6, 0xb2, 0x65, 0xec, 0x1c, 0x7f, 0xc5, 0x32, 0x7f, 0x8b, 0x5d,
	0x1c, 0x81, 0xc7, 0x57, 0x2c, 0xf6, 0x30, 0x82, 0xe0, 0x99, 0xf3, 0x9c, 0xc7, 0xfe, 0x3a, 0xb4,
	0xbb, 0xf7, 0x3f, 0x01, 0x00, 0x00, 0xff, 0xff, 0xf6, 0x93, 0xc6, 0xca, 0x48, 0x01, 0x00, 0x00,
}
