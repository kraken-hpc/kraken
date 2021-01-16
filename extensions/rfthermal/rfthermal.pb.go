// RFThermal.proto: enumeration for determining thermal states of node compoenents.
//
// Author: Ghazanfar Ali <ghazanfar.ali@ttu.edu>;Kevin Pelzel <kevinpelzel22@gmail.com>; J. Lowell Wofford <lowell@lanl.gov>
//
// This software is open source software available under the BSD-3 license.
// Copyright (c) 2019, Triad National Security, LLC
// See LICENSE file for details.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.6.1
// source: rfthermal.proto

package rfthermal

import (
	proto "github.com/golang/protobuf/proto"
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

type Temp_CPUThermalState int32

const (
	Temp_CPU_TEMP_NONE     Temp_CPUThermalState = 0
	Temp_CPU_TEMP_NORMAL   Temp_CPUThermalState = 1
	Temp_CPU_TEMP_HIGH     Temp_CPUThermalState = 2
	Temp_CPU_TEMP_CRITICAL Temp_CPUThermalState = 3
)

// Enum value maps for Temp_CPUThermalState.
var (
	Temp_CPUThermalState_name = map[int32]string{
		0: "CPU_TEMP_NONE",
		1: "CPU_TEMP_NORMAL",
		2: "CPU_TEMP_HIGH",
		3: "CPU_TEMP_CRITICAL",
	}
	Temp_CPUThermalState_value = map[string]int32{
		"CPU_TEMP_NONE":     0,
		"CPU_TEMP_NORMAL":   1,
		"CPU_TEMP_HIGH":     2,
		"CPU_TEMP_CRITICAL": 3,
	}
)

func (x Temp_CPUThermalState) Enum() *Temp_CPUThermalState {
	p := new(Temp_CPUThermalState)
	*p = x
	return p
}

func (x Temp_CPUThermalState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Temp_CPUThermalState) Descriptor() protoreflect.EnumDescriptor {
	return file_rfthermal_proto_enumTypes[0].Descriptor()
}

func (Temp_CPUThermalState) Type() protoreflect.EnumType {
	return &file_rfthermal_proto_enumTypes[0]
}

func (x Temp_CPUThermalState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Temp_CPUThermalState.Descriptor instead.
func (Temp_CPUThermalState) EnumDescriptor() ([]byte, []int) {
	return file_rfthermal_proto_rawDescGZIP(), []int{0, 0}
}

type Temp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	State Temp_CPUThermalState `protobuf:"varint,1,opt,name=state,proto3,enum=RFThermal.Temp_CPUThermalState" json:"state,omitempty"`
}

func (x *Temp) Reset() {
	*x = Temp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rfthermal_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Temp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Temp) ProtoMessage() {}

func (x *Temp) ProtoReflect() protoreflect.Message {
	mi := &file_rfthermal_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Temp.ProtoReflect.Descriptor instead.
func (*Temp) Descriptor() ([]byte, []int) {
	return file_rfthermal_proto_rawDescGZIP(), []int{0}
}

func (x *Temp) GetState() Temp_CPUThermalState {
	if x != nil {
		return x.State
	}
	return Temp_CPU_TEMP_NONE
}

var File_rfthermal_proto protoreflect.FileDescriptor

var file_rfthermal_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x72, 0x66, 0x74, 0x68, 0x65, 0x72, 0x6d, 0x61, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x09, 0x52, 0x46, 0x54, 0x68, 0x65, 0x72, 0x6d, 0x61, 0x6c, 0x22, 0xa2, 0x01, 0x0a,
	0x04, 0x54, 0x65, 0x6d, 0x70, 0x12, 0x35, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0e, 0x32, 0x1f, 0x2e, 0x52, 0x46, 0x54, 0x68, 0x65, 0x72, 0x6d, 0x61, 0x6c,
	0x2e, 0x54, 0x65, 0x6d, 0x70, 0x2e, 0x43, 0x50, 0x55, 0x54, 0x68, 0x65, 0x72, 0x6d, 0x61, 0x6c,
	0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x22, 0x63, 0x0a, 0x0f,
	0x43, 0x50, 0x55, 0x54, 0x68, 0x65, 0x72, 0x6d, 0x61, 0x6c, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12,
	0x11, 0x0a, 0x0d, 0x43, 0x50, 0x55, 0x5f, 0x54, 0x45, 0x4d, 0x50, 0x5f, 0x4e, 0x4f, 0x4e, 0x45,
	0x10, 0x00, 0x12, 0x13, 0x0a, 0x0f, 0x43, 0x50, 0x55, 0x5f, 0x54, 0x45, 0x4d, 0x50, 0x5f, 0x4e,
	0x4f, 0x52, 0x4d, 0x41, 0x4c, 0x10, 0x01, 0x12, 0x11, 0x0a, 0x0d, 0x43, 0x50, 0x55, 0x5f, 0x54,
	0x45, 0x4d, 0x50, 0x5f, 0x48, 0x49, 0x47, 0x48, 0x10, 0x02, 0x12, 0x15, 0x0a, 0x11, 0x43, 0x50,
	0x55, 0x5f, 0x54, 0x45, 0x4d, 0x50, 0x5f, 0x43, 0x52, 0x49, 0x54, 0x49, 0x43, 0x41, 0x4c, 0x10,
	0x03, 0x42, 0x0d, 0x5a, 0x0b, 0x2e, 0x3b, 0x72, 0x66, 0x74, 0x68, 0x65, 0x72, 0x6d, 0x61, 0x6c,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_rfthermal_proto_rawDescOnce sync.Once
	file_rfthermal_proto_rawDescData = file_rfthermal_proto_rawDesc
)

func file_rfthermal_proto_rawDescGZIP() []byte {
	file_rfthermal_proto_rawDescOnce.Do(func() {
		file_rfthermal_proto_rawDescData = protoimpl.X.CompressGZIP(file_rfthermal_proto_rawDescData)
	})
	return file_rfthermal_proto_rawDescData
}

var file_rfthermal_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_rfthermal_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_rfthermal_proto_goTypes = []interface{}{
	(Temp_CPUThermalState)(0), // 0: RFThermal.Temp.CPUThermalState
	(*Temp)(nil),              // 1: RFThermal.Temp
}
var file_rfthermal_proto_depIdxs = []int32{
	0, // 0: RFThermal.Temp.state:type_name -> RFThermal.Temp.CPUThermalState
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_rfthermal_proto_init() }
func file_rfthermal_proto_init() {
	if File_rfthermal_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_rfthermal_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Temp); i {
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
			RawDescriptor: file_rfthermal_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_rfthermal_proto_goTypes,
		DependencyIndexes: file_rfthermal_proto_depIdxs,
		EnumInfos:         file_rfthermal_proto_enumTypes,
		MessageInfos:      file_rfthermal_proto_msgTypes,
	}.Build()
	File_rfthermal_proto = out.File
	file_rfthermal_proto_rawDesc = nil
	file_rfthermal_proto_goTypes = nil
	file_rfthermal_proto_depIdxs = nil
}
