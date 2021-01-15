// Node.proto: describes the base Node object
//
// Author: J. Lowell Wofford <lowell@lanl.gov>
//
// This software is open source software available under the BSD-3 license.
// Copyright (c) 2018-2021, Triad National Security, LLC
// See LICENSE file for details.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.6.1
// source: Node.proto

package proto

import (
	proto "github.com/golang/protobuf/proto"
	any "github.com/golang/protobuf/ptypes/any"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type Node_RunState int32

const (
	Node_UNKNOWN Node_RunState = 0
	Node_INIT    Node_RunState = 1
	Node_SYNC    Node_RunState = 2
	Node_ERROR   Node_RunState = 3
)

// Enum value maps for Node_RunState.
var (
	Node_RunState_name = map[int32]string{
		0: "UNKNOWN",
		1: "INIT",
		2: "SYNC",
		3: "ERROR",
	}
	Node_RunState_value = map[string]int32{
		"UNKNOWN": 0,
		"INIT":    1,
		"SYNC":    2,
		"ERROR":   3,
	}
)

func (x Node_RunState) Enum() *Node_RunState {
	p := new(Node_RunState)
	*p = x
	return p
}

func (x Node_RunState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Node_RunState) Descriptor() protoreflect.EnumDescriptor {
	return file_Node_proto_enumTypes[0].Descriptor()
}

func (Node_RunState) Type() protoreflect.EnumType {
	return &file_Node_proto_enumTypes[0]
}

func (x Node_RunState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Node_RunState.Descriptor instead.
func (Node_RunState) EnumDescriptor() ([]byte, []int) {
	return file_Node_proto_rawDescGZIP(), []int{1, 0}
}

type Node_PhysState int32

const (
	Node_PHYS_UNKNOWN Node_PhysState = 0
	Node_POWER_OFF    Node_PhysState = 1
	Node_POWER_ON     Node_PhysState = 2
	Node_POWER_CYCLE  Node_PhysState = 3 // probably won't be used
	Node_PHYS_HANG    Node_PhysState = 4 // possibly recoverable (by reboot) physical hange
	Node_PHYS_ERROR   Node_PhysState = 5 // in a permanent, unrecoverable state
)

// Enum value maps for Node_PhysState.
var (
	Node_PhysState_name = map[int32]string{
		0: "PHYS_UNKNOWN",
		1: "POWER_OFF",
		2: "POWER_ON",
		3: "POWER_CYCLE",
		4: "PHYS_HANG",
		5: "PHYS_ERROR",
	}
	Node_PhysState_value = map[string]int32{
		"PHYS_UNKNOWN": 0,
		"POWER_OFF":    1,
		"POWER_ON":     2,
		"POWER_CYCLE":  3,
		"PHYS_HANG":    4,
		"PHYS_ERROR":   5,
	}
)

func (x Node_PhysState) Enum() *Node_PhysState {
	p := new(Node_PhysState)
	*p = x
	return p
}

func (x Node_PhysState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Node_PhysState) Descriptor() protoreflect.EnumDescriptor {
	return file_Node_proto_enumTypes[1].Descriptor()
}

func (Node_PhysState) Type() protoreflect.EnumType {
	return &file_Node_proto_enumTypes[1]
}

func (x Node_PhysState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Node_PhysState.Descriptor instead.
func (Node_PhysState) EnumDescriptor() ([]byte, []int) {
	return file_Node_proto_rawDescGZIP(), []int{1, 1}
}

type NodeList struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Nodes []*Node `protobuf:"bytes,1,rep,name=nodes,proto3" json:"nodes,omitempty"`
}

func (x *NodeList) Reset() {
	*x = NodeList{}
	if protoimpl.UnsafeEnabled {
		mi := &file_Node_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NodeList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeList) ProtoMessage() {}

func (x *NodeList) ProtoReflect() protoreflect.Message {
	mi := &file_Node_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeList.ProtoReflect.Descriptor instead.
func (*NodeList) Descriptor() ([]byte, []int) {
	return file_Node_proto_rawDescGZIP(), []int{0}
}

func (x *NodeList) GetNodes() []*Node {
	if x != nil {
		return x.Nodes
	}
	return nil
}

type Node struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id         []byte             `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Nodename   string             `protobuf:"bytes,2,opt,name=nodename,proto3" json:"nodename,omitempty"` // may or may not be the hostname
	RunState   Node_RunState      `protobuf:"varint,3,opt,name=run_state,json=runState,proto3,enum=proto.Node_RunState" json:"run_state,omitempty"`
	PhysState  Node_PhysState     `protobuf:"varint,4,opt,name=phys_state,json=physState,proto3,enum=proto.Node_PhysState" json:"phys_state,omitempty"`
	Arch       string             `protobuf:"bytes,5,opt,name=arch,proto3" json:"arch,omitempty"`
	Platform   string             `protobuf:"bytes,6,opt,name=platform,proto3" json:"platform,omitempty"`
	ParentId   []byte             `protobuf:"bytes,7,opt,name=parent_id,json=parentId,proto3" json:"parent_id,omitempty"`
	Services   []*ServiceInstance `protobuf:"bytes,14,rep,name=services,proto3" json:"services,omitempty"`
	Extensions []*any.Any         `protobuf:"bytes,15,rep,name=extensions,proto3" json:"extensions,omitempty"`
}

func (x *Node) Reset() {
	*x = Node{}
	if protoimpl.UnsafeEnabled {
		mi := &file_Node_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Node) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Node) ProtoMessage() {}

func (x *Node) ProtoReflect() protoreflect.Message {
	mi := &file_Node_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Node.ProtoReflect.Descriptor instead.
func (*Node) Descriptor() ([]byte, []int) {
	return file_Node_proto_rawDescGZIP(), []int{1}
}

func (x *Node) GetId() []byte {
	if x != nil {
		return x.Id
	}
	return nil
}

func (x *Node) GetNodename() string {
	if x != nil {
		return x.Nodename
	}
	return ""
}

func (x *Node) GetRunState() Node_RunState {
	if x != nil {
		return x.RunState
	}
	return Node_UNKNOWN
}

func (x *Node) GetPhysState() Node_PhysState {
	if x != nil {
		return x.PhysState
	}
	return Node_PHYS_UNKNOWN
}

func (x *Node) GetArch() string {
	if x != nil {
		return x.Arch
	}
	return ""
}

func (x *Node) GetPlatform() string {
	if x != nil {
		return x.Platform
	}
	return ""
}

func (x *Node) GetParentId() []byte {
	if x != nil {
		return x.ParentId
	}
	return nil
}

func (x *Node) GetServices() []*ServiceInstance {
	if x != nil {
		return x.Services
	}
	return nil
}

func (x *Node) GetExtensions() []*any.Any {
	if x != nil {
		return x.Extensions
	}
	return nil
}

var File_Node_proto protoreflect.FileDescriptor

var file_Node_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x4e, 0x6f, 0x64, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2f, 0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x2d, 0x0a, 0x08, 0x4e, 0x6f, 0x64, 0x65, 0x4c, 0x69, 0x73,
	0x74, 0x12, 0x21, 0x0a, 0x05, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x0b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x52, 0x05, 0x6e,
	0x6f, 0x64, 0x65, 0x73, 0x22, 0xfc, 0x03, 0x0a, 0x04, 0x4e, 0x6f, 0x64, 0x65, 0x12, 0x0e, 0x0a,
	0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1a, 0x0a,
	0x08, 0x6e, 0x6f, 0x64, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x6e, 0x6f, 0x64, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x31, 0x0a, 0x09, 0x72, 0x75, 0x6e,
	0x5f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x14, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x2e, 0x52, 0x75, 0x6e, 0x53, 0x74, 0x61,
	0x74, 0x65, 0x52, 0x08, 0x72, 0x75, 0x6e, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x34, 0x0a, 0x0a,
	0x70, 0x68, 0x79, 0x73, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x15, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x2e, 0x50, 0x68,
	0x79, 0x73, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x09, 0x70, 0x68, 0x79, 0x73, 0x53, 0x74, 0x61,
	0x74, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x72, 0x63, 0x68, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x61, 0x72, 0x63, 0x68, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f,
	0x72, 0x6d, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f,
	0x72, 0x6d, 0x12, 0x1b, 0x0a, 0x09, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x69, 0x64, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x49, 0x64, 0x12,
	0x32, 0x0a, 0x08, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x18, 0x0e, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x16, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x52, 0x08, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x73, 0x12, 0x34, 0x0a, 0x0a, 0x65, 0x78, 0x74, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e,
	0x73, 0x18, 0x0f, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52, 0x0a, 0x65,
	0x78, 0x74, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0x36, 0x0a, 0x08, 0x52, 0x75, 0x6e,
	0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e,
	0x10, 0x00, 0x12, 0x08, 0x0a, 0x04, 0x49, 0x4e, 0x49, 0x54, 0x10, 0x01, 0x12, 0x08, 0x0a, 0x04,
	0x53, 0x59, 0x4e, 0x43, 0x10, 0x02, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10,
	0x03, 0x22, 0x6a, 0x0a, 0x09, 0x50, 0x68, 0x79, 0x73, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x10,
	0x0a, 0x0c, 0x50, 0x48, 0x59, 0x53, 0x5f, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00,
	0x12, 0x0d, 0x0a, 0x09, 0x50, 0x4f, 0x57, 0x45, 0x52, 0x5f, 0x4f, 0x46, 0x46, 0x10, 0x01, 0x12,
	0x0c, 0x0a, 0x08, 0x50, 0x4f, 0x57, 0x45, 0x52, 0x5f, 0x4f, 0x4e, 0x10, 0x02, 0x12, 0x0f, 0x0a,
	0x0b, 0x50, 0x4f, 0x57, 0x45, 0x52, 0x5f, 0x43, 0x59, 0x43, 0x4c, 0x45, 0x10, 0x03, 0x12, 0x0d,
	0x0a, 0x09, 0x50, 0x48, 0x59, 0x53, 0x5f, 0x48, 0x41, 0x4e, 0x47, 0x10, 0x04, 0x12, 0x0e, 0x0a,
	0x0a, 0x50, 0x48, 0x59, 0x53, 0x5f, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x05, 0x4a, 0x04, 0x08,
	0x08, 0x10, 0x0e, 0x42, 0x09, 0x5a, 0x07, 0x2e, 0x3b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_Node_proto_rawDescOnce sync.Once
	file_Node_proto_rawDescData = file_Node_proto_rawDesc
)

func file_Node_proto_rawDescGZIP() []byte {
	file_Node_proto_rawDescOnce.Do(func() {
		file_Node_proto_rawDescData = protoimpl.X.CompressGZIP(file_Node_proto_rawDescData)
	})
	return file_Node_proto_rawDescData
}

var file_Node_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_Node_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_Node_proto_goTypes = []interface{}{
	(Node_RunState)(0),      // 0: proto.Node.RunState
	(Node_PhysState)(0),     // 1: proto.Node.PhysState
	(*NodeList)(nil),        // 2: proto.NodeList
	(*Node)(nil),            // 3: proto.Node
	(*ServiceInstance)(nil), // 4: proto.ServiceInstance
	(*any.Any)(nil),         // 5: google.protobuf.Any
}
var file_Node_proto_depIdxs = []int32{
	3, // 0: proto.NodeList.nodes:type_name -> proto.Node
	0, // 1: proto.Node.run_state:type_name -> proto.Node.RunState
	1, // 2: proto.Node.phys_state:type_name -> proto.Node.PhysState
	4, // 3: proto.Node.services:type_name -> proto.ServiceInstance
	5, // 4: proto.Node.extensions:type_name -> google.protobuf.Any
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_Node_proto_init() }
func file_Node_proto_init() {
	if File_Node_proto != nil {
		return
	}
	file_ServiceInstance_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_Node_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NodeList); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_Node_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Node); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_Node_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_Node_proto_goTypes,
		DependencyIndexes: file_Node_proto_depIdxs,
		EnumInfos:         file_Node_proto_enumTypes,
		MessageInfos:      file_Node_proto_msgTypes,
	}.Build()
	File_Node_proto = out.File
	file_Node_proto_rawDesc = nil
	file_Node_proto_goTypes = nil
	file_Node_proto_depIdxs = nil
}
