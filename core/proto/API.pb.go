// Code generated by protoc-gen-go. DO NOT EDIT.
// source: API.proto

package proto

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import any "github.com/golang/protobuf/ptypes/any"
import _ "github.com/golang/protobuf/ptypes/empty"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type ServiceControl_Command int32

const (
	ServiceControl_STOP   ServiceControl_Command = 0
	ServiceControl_UPDATE ServiceControl_Command = 1
	ServiceControl_INIT   ServiceControl_Command = 2
)

var ServiceControl_Command_name = map[int32]string{
	0: "STOP",
	1: "UPDATE",
	2: "INIT",
}
var ServiceControl_Command_value = map[string]int32{
	"STOP":   0,
	"UPDATE": 1,
	"INIT":   2,
}

func (x ServiceControl_Command) String() string {
	return proto.EnumName(ServiceControl_Command_name, int32(x))
}
func (ServiceControl_Command) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{3, 0}
}

type MutationControl_Type int32

const (
	MutationControl_MUTATE    MutationControl_Type = 0
	MutationControl_INTERRUPT MutationControl_Type = 1
)

var MutationControl_Type_name = map[int32]string{
	0: "MUTATE",
	1: "INTERRUPT",
}
var MutationControl_Type_value = map[string]int32{
	"MUTATE":    0,
	"INTERRUPT": 1,
}

func (x MutationControl_Type) String() string {
	return proto.EnumName(MutationControl_Type_name, int32(x))
}
func (MutationControl_Type) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{4, 0}
}

type Query struct {
	URL string `protobuf:"bytes,1,opt,name=URL,proto3" json:"URL,omitempty"`
	// Types that are valid to be assigned to Payload:
	//	*Query_Node
	//	*Query_Value
	//	*Query_Text
	Payload              isQuery_Payload `protobuf_oneof:"payload"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *Query) Reset()         { *m = Query{} }
func (m *Query) String() string { return proto.CompactTextString(m) }
func (*Query) ProtoMessage()    {}
func (*Query) Descriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{0}
}
func (m *Query) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Query.Unmarshal(m, b)
}
func (m *Query) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Query.Marshal(b, m, deterministic)
}
func (dst *Query) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Query.Merge(dst, src)
}
func (m *Query) XXX_Size() int {
	return xxx_messageInfo_Query.Size(m)
}
func (m *Query) XXX_DiscardUnknown() {
	xxx_messageInfo_Query.DiscardUnknown(m)
}

var xxx_messageInfo_Query proto.InternalMessageInfo

func (m *Query) GetURL() string {
	if m != nil {
		return m.URL
	}
	return ""
}

type isQuery_Payload interface {
	isQuery_Payload()
}

type Query_Node struct {
	Node *Node `protobuf:"bytes,2,opt,name=node,proto3,oneof"`
}

type Query_Value struct {
	Value *ReflectValue `protobuf:"bytes,3,opt,name=value,proto3,oneof"`
}

type Query_Text struct {
	Text string `protobuf:"bytes,4,opt,name=text,proto3,oneof"`
}

func (*Query_Node) isQuery_Payload() {}

func (*Query_Value) isQuery_Payload() {}

func (*Query_Text) isQuery_Payload() {}

func (m *Query) GetPayload() isQuery_Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *Query) GetNode() *Node {
	if x, ok := m.GetPayload().(*Query_Node); ok {
		return x.Node
	}
	return nil
}

func (m *Query) GetValue() *ReflectValue {
	if x, ok := m.GetPayload().(*Query_Value); ok {
		return x.Value
	}
	return nil
}

func (m *Query) GetText() string {
	if x, ok := m.GetPayload().(*Query_Text); ok {
		return x.Text
	}
	return ""
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*Query) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _Query_OneofMarshaler, _Query_OneofUnmarshaler, _Query_OneofSizer, []interface{}{
		(*Query_Node)(nil),
		(*Query_Value)(nil),
		(*Query_Text)(nil),
	}
}

func _Query_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*Query)
	// payload
	switch x := m.Payload.(type) {
	case *Query_Node:
		b.EncodeVarint(2<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Node); err != nil {
			return err
		}
	case *Query_Value:
		b.EncodeVarint(3<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Value); err != nil {
			return err
		}
	case *Query_Text:
		b.EncodeVarint(4<<3 | proto.WireBytes)
		b.EncodeStringBytes(x.Text)
	case nil:
	default:
		return fmt.Errorf("Query.Payload has unexpected type %T", x)
	}
	return nil
}

func _Query_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*Query)
	switch tag {
	case 2: // payload.node
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(Node)
		err := b.DecodeMessage(msg)
		m.Payload = &Query_Node{msg}
		return true, err
	case 3: // payload.value
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(ReflectValue)
		err := b.DecodeMessage(msg)
		m.Payload = &Query_Value{msg}
		return true, err
	case 4: // payload.text
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		x, err := b.DecodeStringBytes()
		m.Payload = &Query_Text{x}
		return true, err
	default:
		return false, nil
	}
}

func _Query_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*Query)
	// payload
	switch x := m.Payload.(type) {
	case *Query_Node:
		s := proto.Size(x.Node)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Query_Value:
		s := proto.Size(x.Value)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Query_Text:
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(len(x.Text)))
		n += len(x.Text)
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

type QueryMulti struct {
	Queries              []*Query `protobuf:"bytes,1,rep,name=queries,proto3" json:"queries,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *QueryMulti) Reset()         { *m = QueryMulti{} }
func (m *QueryMulti) String() string { return proto.CompactTextString(m) }
func (*QueryMulti) ProtoMessage()    {}
func (*QueryMulti) Descriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{1}
}
func (m *QueryMulti) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_QueryMulti.Unmarshal(m, b)
}
func (m *QueryMulti) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_QueryMulti.Marshal(b, m, deterministic)
}
func (dst *QueryMulti) XXX_Merge(src proto.Message) {
	xxx_messageInfo_QueryMulti.Merge(dst, src)
}
func (m *QueryMulti) XXX_Size() int {
	return xxx_messageInfo_QueryMulti.Size(m)
}
func (m *QueryMulti) XXX_DiscardUnknown() {
	xxx_messageInfo_QueryMulti.DiscardUnknown(m)
}

var xxx_messageInfo_QueryMulti proto.InternalMessageInfo

func (m *QueryMulti) GetQueries() []*Query {
	if m != nil {
		return m.Queries
	}
	return nil
}

type ServiceInitRequest struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Module               string   `protobuf:"bytes,2,opt,name=module,proto3" json:"module,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ServiceInitRequest) Reset()         { *m = ServiceInitRequest{} }
func (m *ServiceInitRequest) String() string { return proto.CompactTextString(m) }
func (*ServiceInitRequest) ProtoMessage()    {}
func (*ServiceInitRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{2}
}
func (m *ServiceInitRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ServiceInitRequest.Unmarshal(m, b)
}
func (m *ServiceInitRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ServiceInitRequest.Marshal(b, m, deterministic)
}
func (dst *ServiceInitRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ServiceInitRequest.Merge(dst, src)
}
func (m *ServiceInitRequest) XXX_Size() int {
	return xxx_messageInfo_ServiceInitRequest.Size(m)
}
func (m *ServiceInitRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ServiceInitRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ServiceInitRequest proto.InternalMessageInfo

func (m *ServiceInitRequest) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *ServiceInitRequest) GetModule() string {
	if m != nil {
		return m.Module
	}
	return ""
}

type ServiceControl struct {
	Command              ServiceControl_Command `protobuf:"varint,1,opt,name=command,proto3,enum=proto.ServiceControl_Command" json:"command,omitempty"`
	Config               *any.Any               `protobuf:"bytes,2,opt,name=config,proto3" json:"config,omitempty"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *ServiceControl) Reset()         { *m = ServiceControl{} }
func (m *ServiceControl) String() string { return proto.CompactTextString(m) }
func (*ServiceControl) ProtoMessage()    {}
func (*ServiceControl) Descriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{3}
}
func (m *ServiceControl) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ServiceControl.Unmarshal(m, b)
}
func (m *ServiceControl) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ServiceControl.Marshal(b, m, deterministic)
}
func (dst *ServiceControl) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ServiceControl.Merge(dst, src)
}
func (m *ServiceControl) XXX_Size() int {
	return xxx_messageInfo_ServiceControl.Size(m)
}
func (m *ServiceControl) XXX_DiscardUnknown() {
	xxx_messageInfo_ServiceControl.DiscardUnknown(m)
}

var xxx_messageInfo_ServiceControl proto.InternalMessageInfo

func (m *ServiceControl) GetCommand() ServiceControl_Command {
	if m != nil {
		return m.Command
	}
	return ServiceControl_STOP
}

func (m *ServiceControl) GetConfig() *any.Any {
	if m != nil {
		return m.Config
	}
	return nil
}

type MutationControl struct {
	Module               string               `protobuf:"bytes,1,opt,name=module,proto3" json:"module,omitempty"`
	Id                   string               `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	Type                 MutationControl_Type `protobuf:"varint,3,opt,name=type,proto3,enum=proto.MutationControl_Type" json:"type,omitempty"`
	Cfg                  *Node                `protobuf:"bytes,4,opt,name=cfg,proto3" json:"cfg,omitempty"`
	Dsc                  *Node                `protobuf:"bytes,5,opt,name=dsc,proto3" json:"dsc,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *MutationControl) Reset()         { *m = MutationControl{} }
func (m *MutationControl) String() string { return proto.CompactTextString(m) }
func (*MutationControl) ProtoMessage()    {}
func (*MutationControl) Descriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{4}
}
func (m *MutationControl) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MutationControl.Unmarshal(m, b)
}
func (m *MutationControl) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MutationControl.Marshal(b, m, deterministic)
}
func (dst *MutationControl) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MutationControl.Merge(dst, src)
}
func (m *MutationControl) XXX_Size() int {
	return xxx_messageInfo_MutationControl.Size(m)
}
func (m *MutationControl) XXX_DiscardUnknown() {
	xxx_messageInfo_MutationControl.DiscardUnknown(m)
}

var xxx_messageInfo_MutationControl proto.InternalMessageInfo

func (m *MutationControl) GetModule() string {
	if m != nil {
		return m.Module
	}
	return ""
}

func (m *MutationControl) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *MutationControl) GetType() MutationControl_Type {
	if m != nil {
		return m.Type
	}
	return MutationControl_MUTATE
}

func (m *MutationControl) GetCfg() *Node {
	if m != nil {
		return m.Cfg
	}
	return nil
}

func (m *MutationControl) GetDsc() *Node {
	if m != nil {
		return m.Dsc
	}
	return nil
}

type DiscoveryEvent struct {
	Module               string   `protobuf:"bytes,1,opt,name=module,proto3" json:"module,omitempty"`
	Url                  string   `protobuf:"bytes,2,opt,name=url,proto3" json:"url,omitempty"`
	ValueId              string   `protobuf:"bytes,3,opt,name=value_id,json=valueId,proto3" json:"value_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DiscoveryEvent) Reset()         { *m = DiscoveryEvent{} }
func (m *DiscoveryEvent) String() string { return proto.CompactTextString(m) }
func (*DiscoveryEvent) ProtoMessage()    {}
func (*DiscoveryEvent) Descriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{5}
}
func (m *DiscoveryEvent) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DiscoveryEvent.Unmarshal(m, b)
}
func (m *DiscoveryEvent) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DiscoveryEvent.Marshal(b, m, deterministic)
}
func (dst *DiscoveryEvent) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DiscoveryEvent.Merge(dst, src)
}
func (m *DiscoveryEvent) XXX_Size() int {
	return xxx_messageInfo_DiscoveryEvent.Size(m)
}
func (m *DiscoveryEvent) XXX_DiscardUnknown() {
	xxx_messageInfo_DiscoveryEvent.DiscardUnknown(m)
}

var xxx_messageInfo_DiscoveryEvent proto.InternalMessageInfo

func (m *DiscoveryEvent) GetModule() string {
	if m != nil {
		return m.Module
	}
	return ""
}

func (m *DiscoveryEvent) GetUrl() string {
	if m != nil {
		return m.Url
	}
	return ""
}

func (m *DiscoveryEvent) GetValueId() string {
	if m != nil {
		return m.ValueId
	}
	return ""
}

type ReflectValue struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReflectValue) Reset()         { *m = ReflectValue{} }
func (m *ReflectValue) String() string { return proto.CompactTextString(m) }
func (*ReflectValue) ProtoMessage()    {}
func (*ReflectValue) Descriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{6}
}
func (m *ReflectValue) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReflectValue.Unmarshal(m, b)
}
func (m *ReflectValue) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReflectValue.Marshal(b, m, deterministic)
}
func (dst *ReflectValue) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReflectValue.Merge(dst, src)
}
func (m *ReflectValue) XXX_Size() int {
	return xxx_messageInfo_ReflectValue.Size(m)
}
func (m *ReflectValue) XXX_DiscardUnknown() {
	xxx_messageInfo_ReflectValue.DiscardUnknown(m)
}

var xxx_messageInfo_ReflectValue proto.InternalMessageInfo

type LogMessage struct {
	Origin               string   `protobuf:"bytes,1,opt,name=origin,proto3" json:"origin,omitempty"`
	Level                uint32   `protobuf:"varint,2,opt,name=level,proto3" json:"level,omitempty"`
	Msg                  string   `protobuf:"bytes,3,opt,name=msg,proto3" json:"msg,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *LogMessage) Reset()         { *m = LogMessage{} }
func (m *LogMessage) String() string { return proto.CompactTextString(m) }
func (*LogMessage) ProtoMessage()    {}
func (*LogMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_API_bac828d1e5f84257, []int{7}
}
func (m *LogMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LogMessage.Unmarshal(m, b)
}
func (m *LogMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LogMessage.Marshal(b, m, deterministic)
}
func (dst *LogMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LogMessage.Merge(dst, src)
}
func (m *LogMessage) XXX_Size() int {
	return xxx_messageInfo_LogMessage.Size(m)
}
func (m *LogMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_LogMessage.DiscardUnknown(m)
}

var xxx_messageInfo_LogMessage proto.InternalMessageInfo

func (m *LogMessage) GetOrigin() string {
	if m != nil {
		return m.Origin
	}
	return ""
}

func (m *LogMessage) GetLevel() uint32 {
	if m != nil {
		return m.Level
	}
	return 0
}

func (m *LogMessage) GetMsg() string {
	if m != nil {
		return m.Msg
	}
	return ""
}

func init() {
	proto.RegisterType((*Query)(nil), "proto.Query")
	proto.RegisterType((*QueryMulti)(nil), "proto.QueryMulti")
	proto.RegisterType((*ServiceInitRequest)(nil), "proto.ServiceInitRequest")
	proto.RegisterType((*ServiceControl)(nil), "proto.ServiceControl")
	proto.RegisterType((*MutationControl)(nil), "proto.MutationControl")
	proto.RegisterType((*DiscoveryEvent)(nil), "proto.DiscoveryEvent")
	proto.RegisterType((*ReflectValue)(nil), "proto.ReflectValue")
	proto.RegisterType((*LogMessage)(nil), "proto.LogMessage")
	proto.RegisterEnum("proto.ServiceControl_Command", ServiceControl_Command_name, ServiceControl_Command_value)
	proto.RegisterEnum("proto.MutationControl_Type", MutationControl_Type_name, MutationControl_Type_value)
}

func init() { proto.RegisterFile("API.proto", fileDescriptor_API_bac828d1e5f84257) }

var fileDescriptor_API_bac828d1e5f84257 = []byte{
	// 705 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x54, 0x5d, 0x4e, 0xdb, 0x40,
	0x10, 0xb6, 0xf3, 0x4b, 0x26, 0x21, 0xa4, 0x5b, 0x8a, 0x42, 0x10, 0x12, 0xec, 0x43, 0x15, 0x04,
	0x32, 0x55, 0x5a, 0xa9, 0x0f, 0xb4, 0x95, 0x42, 0x12, 0x89, 0x48, 0x84, 0xa6, 0x4b, 0xd2, 0xd7,
	0xca, 0xd8, 0x1b, 0xcb, 0x92, 0xe3, 0x0d, 0xf6, 0x3a, 0xaa, 0xaf, 0xd0, 0x63, 0xf4, 0x3a, 0xbd,
	0x43, 0xcf, 0x52, 0xed, 0x7a, 0x0d, 0x0e, 0x85, 0xa6, 0x3c, 0xd9, 0xb3, 0xf3, 0xcd, 0xf7, 0xcd,
	0x67, 0xcf, 0x2c, 0x54, 0xba, 0xe3, 0xa1, 0xb1, 0x08, 0x18, 0x67, 0xa8, 0x28, 0x1f, 0x2d, 0xb8,
	0x62, 0x36, 0x4d, 0x8e, 0x5a, 0xbb, 0x0e, 0x63, 0x8e, 0x47, 0x4f, 0x65, 0x74, 0x13, 0xcd, 0x4e,
	0x4d, 0x3f, 0x56, 0xa9, 0xbd, 0x87, 0xa9, 0xc1, 0x7c, 0xc1, 0x55, 0x12, 0xff, 0xd0, 0xa1, 0xf8,
	0x25, 0xa2, 0x41, 0x8c, 0x1a, 0x90, 0x9f, 0x92, 0xcb, 0xa6, 0x7e, 0xa0, 0xb7, 0x2b, 0x44, 0xbc,
	0xa2, 0x43, 0x28, 0xf8, 0xcc, 0xa6, 0xcd, 0xdc, 0x81, 0xde, 0xae, 0x76, 0xaa, 0x49, 0x85, 0x21,
	0x44, 0x2f, 0x34, 0x22, 0x53, 0xe8, 0x18, 0x8a, 0x4b, 0xd3, 0x8b, 0x68, 0x33, 0x2f, 0x31, 0x2f,
	0x15, 0x86, 0xd0, 0x99, 0x47, 0x2d, 0xfe, 0x55, 0xa4, 0x2e, 0x34, 0x92, 0x60, 0xd0, 0x36, 0x14,
	0x38, 0xfd, 0xce, 0x9b, 0x05, 0x21, 0x21, 0x28, 0x44, 0x74, 0x5e, 0x81, 0xf2, 0xc2, 0x8c, 0x3d,
	0x66, 0xda, 0xf8, 0x1d, 0x80, 0xec, 0x65, 0x14, 0x79, 0xdc, 0x45, 0xaf, 0xa1, 0x7c, 0x1b, 0xd1,
	0xc0, 0xa5, 0x61, 0x53, 0x3f, 0xc8, 0xb7, 0xab, 0x9d, 0x9a, 0x62, 0x97, 0x18, 0x92, 0x26, 0xf1,
	0x07, 0x40, 0xd7, 0x34, 0x58, 0xba, 0x16, 0x1d, 0xfa, 0x2e, 0x27, 0xf4, 0x36, 0xa2, 0x21, 0x47,
	0x75, 0xc8, 0xb9, 0xb6, 0x72, 0x93, 0x73, 0x6d, 0xb4, 0x03, 0xa5, 0x39, 0xb3, 0x23, 0x2f, 0xb1,
	0x53, 0x21, 0x2a, 0xc2, 0x3f, 0x75, 0xa8, 0xab, 0xf2, 0x1e, 0xf3, 0x79, 0xc0, 0x3c, 0xf4, 0x1e,
	0xca, 0x16, 0x9b, 0xcf, 0x4d, 0x3f, 0xa9, 0xaf, 0x77, 0xf6, 0x95, 0xf0, 0x2a, 0xce, 0xe8, 0x25,
	0x20, 0x92, 0xa2, 0xd1, 0x09, 0x94, 0x2c, 0xe6, 0xcf, 0x5c, 0x47, 0x7d, 0xb2, 0x6d, 0x23, 0xf9,
	0xf4, 0x46, 0xfa, 0xe9, 0x8d, 0xae, 0x1f, 0x13, 0x85, 0xc1, 0x47, 0x50, 0x56, 0x0c, 0x68, 0x03,
	0x0a, 0xd7, 0x93, 0xcf, 0xe3, 0x86, 0x86, 0x00, 0x4a, 0xd3, 0x71, 0xbf, 0x3b, 0x19, 0x34, 0x74,
	0x71, 0x3a, 0xbc, 0x1a, 0x4e, 0x1a, 0x39, 0xfc, 0x4b, 0x87, 0xad, 0x51, 0xc4, 0x4d, 0xee, 0x32,
	0x3f, 0xed, 0xf2, 0xde, 0x90, 0x9e, 0x35, 0xa4, 0x8c, 0xe7, 0xee, 0x8c, 0x9f, 0x42, 0x81, 0xc7,
	0x8b, 0xe4, 0x0f, 0xd5, 0x3b, 0x7b, 0xca, 0xca, 0x03, 0x36, 0x63, 0x12, 0x2f, 0x28, 0x91, 0x40,
	0xb4, 0x0f, 0x79, 0x6b, 0xe6, 0xc8, 0xbf, 0xb4, 0xfa, 0xd7, 0x89, 0x38, 0x17, 0x69, 0x3b, 0xb4,
	0x9a, 0xc5, 0x47, 0xd2, 0x76, 0x68, 0xe1, 0x43, 0x28, 0x08, 0x2e, 0x61, 0x64, 0x34, 0x9d, 0x08,
	0x23, 0x1a, 0xda, 0x84, 0xca, 0xf0, 0x6a, 0x32, 0x20, 0x64, 0x3a, 0x9e, 0x34, 0x74, 0x3c, 0x85,
	0x7a, 0xdf, 0x0d, 0x2d, 0xb6, 0xa4, 0x41, 0x3c, 0x58, 0x52, 0x9f, 0x3f, 0xe9, 0xa5, 0x01, 0xf9,
	0x28, 0xf0, 0x94, 0x19, 0xf1, 0x8a, 0x76, 0x61, 0x43, 0x0e, 0xd3, 0x37, 0xd7, 0x96, 0x8e, 0x2a,
	0xa4, 0x2c, 0xe3, 0xa1, 0x8d, 0xeb, 0x50, 0xcb, 0xce, 0x1d, 0xbe, 0x04, 0xb8, 0x64, 0xce, 0x88,
	0x86, 0xa1, 0xe9, 0x50, 0x21, 0xc1, 0x02, 0xd7, 0x71, 0xfd, 0x54, 0x22, 0x89, 0xd0, 0x36, 0x14,
	0x3d, 0xba, 0xa4, 0x89, 0xc8, 0x26, 0x49, 0x02, 0x21, 0x3c, 0x0f, 0x1d, 0xa5, 0x20, 0x5e, 0x3b,
	0xbf, 0x8b, 0x90, 0xef, 0x8e, 0x87, 0xe8, 0x18, 0xaa, 0x72, 0xfe, 0x7a, 0x01, 0x35, 0x39, 0x45,
	0x2b, 0x33, 0xd9, 0x5a, 0x89, 0xb0, 0x86, 0x8e, 0xa0, 0x92, 0x0c, 0x2b, 0x35, 0xed, 0x35, 0xd0,
	0x13, 0xa8, 0xdd, 0x41, 0xfb, 0xa1, 0xb5, 0x06, 0x9d, 0x76, 0x31, 0x5d, 0xd8, 0xeb, 0xbb, 0x30,
	0xa0, 0x9e, 0x01, 0xff, 0x3f, 0x79, 0x9f, 0x7a, 0x74, 0x2d, 0xf9, 0x59, 0xa6, 0xef, 0xae, 0xe7,
	0xa1, 0x9d, 0xbf, 0x66, 0x5e, 0x5e, 0x37, 0xad, 0x17, 0xd9, 0x3a, 0xb9, 0xe0, 0x58, 0x43, 0x9f,
	0x60, 0x2b, 0x5b, 0x2c, 0x5a, 0x7b, 0x56, 0xfd, 0x47, 0xe5, 0x2c, 0xe9, 0xf4, 0xd9, 0xf2, 0x3d,
	0xa8, 0x66, 0x6e, 0x0e, 0xb4, 0xbb, 0xba, 0xe6, 0x99, 0xdb, 0xa4, 0xf5, 0xea, 0xd1, 0x1b, 0x00,
	0x6b, 0x6f, 0x74, 0x34, 0x80, 0x5a, 0xba, 0x4c, 0xeb, 0x58, 0x76, 0x1e, 0x5f, 0x3e, 0x49, 0x73,
	0x0e, 0x9b, 0x77, 0x4b, 0x21, 0x79, 0x52, 0xc9, 0xd5, 0x55, 0x69, 0x3d, 0x61, 0x10, 0x6b, 0x6d,
	0x1d, 0x9d, 0xc9, 0x89, 0x77, 0x68, 0x20, 0x09, 0x52, 0xcb, 0xf7, 0x4b, 0xf0, 0xaf, 0xe2, 0x9b,
	0x92, 0x3c, 0x7b, 0xfb, 0x27, 0x00, 0x00, 0xff, 0xff, 0xe5, 0x77, 0x51, 0x20, 0x68, 0x06, 0x00,
	0x00,
}
