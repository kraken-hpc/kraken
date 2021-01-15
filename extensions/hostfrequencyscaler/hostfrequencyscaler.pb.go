// HostFrequencyScaler.proto: describes host specific CPU frquency scaling policy objects
//
// Author: Ghazanfar Ali <ghazanfar.ali@ttu.edu>, Kevin Pelzel <kevinpelzel22@gmail.com>;J. Lowell Wofford <lowell@lanl.gov>
//
// This software is open source software available under the BSD-3 license.
// Copyright (c) 2019, Triad National Security, LLC
// See LICENSE file for details.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.6.1
// source: hostfrequencyscaler.proto

package hostfrequencyscaler

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

type Scaler_ScalerState int32

const (
	Scaler_NONE        Scaler_ScalerState = 0
	Scaler_POWER_SAVE  Scaler_ScalerState = 1
	Scaler_PERFORMANCE Scaler_ScalerState = 2
	Scaler_SCHEDUTIL   Scaler_ScalerState = 3
)

// Enum value maps for Scaler_ScalerState.
var (
	Scaler_ScalerState_name = map[int32]string{
		0: "NONE",
		1: "POWER_SAVE",
		2: "PERFORMANCE",
		3: "SCHEDUTIL",
	}
	Scaler_ScalerState_value = map[string]int32{
		"NONE":        0,
		"POWER_SAVE":  1,
		"PERFORMANCE": 2,
		"SCHEDUTIL":   3,
	}
)

func (x Scaler_ScalerState) Enum() *Scaler_ScalerState {
	p := new(Scaler_ScalerState)
	*p = x
	return p
}

func (x Scaler_ScalerState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Scaler_ScalerState) Descriptor() protoreflect.EnumDescriptor {
	return file_hostfrequencyscaler_proto_enumTypes[0].Descriptor()
}

func (Scaler_ScalerState) Type() protoreflect.EnumType {
	return &file_hostfrequencyscaler_proto_enumTypes[0]
}

func (x Scaler_ScalerState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Scaler_ScalerState.Descriptor instead.
func (Scaler_ScalerState) EnumDescriptor() ([]byte, []int) {
	return file_hostfrequencyscaler_proto_rawDescGZIP(), []int{0, 0}
}

type Scaler struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	State Scaler_ScalerState `protobuf:"varint,1,opt,name=state,proto3,enum=HostFrequencyScaler.Scaler_ScalerState" json:"state,omitempty"`
}

func (x *Scaler) Reset() {
	*x = Scaler{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hostfrequencyscaler_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Scaler) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Scaler) ProtoMessage() {}

func (x *Scaler) ProtoReflect() protoreflect.Message {
	mi := &file_hostfrequencyscaler_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Scaler.ProtoReflect.Descriptor instead.
func (*Scaler) Descriptor() ([]byte, []int) {
	return file_hostfrequencyscaler_proto_rawDescGZIP(), []int{0}
}

func (x *Scaler) GetState() Scaler_ScalerState {
	if x != nil {
		return x.State
	}
	return Scaler_NONE
}

var File_hostfrequencyscaler_proto protoreflect.FileDescriptor

var file_hostfrequencyscaler_proto_rawDesc = []byte{
	0x0a, 0x19, 0x68, 0x6f, 0x73, 0x74, 0x66, 0x72, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x79, 0x73,
	0x63, 0x61, 0x6c, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x13, 0x48, 0x6f, 0x73,
	0x74, 0x46, 0x72, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x79, 0x53, 0x63, 0x61, 0x6c, 0x65, 0x72,
	0x22, 0x90, 0x01, 0x0a, 0x06, 0x53, 0x63, 0x61, 0x6c, 0x65, 0x72, 0x12, 0x3d, 0x0a, 0x05, 0x73,
	0x74, 0x61, 0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x27, 0x2e, 0x48, 0x6f, 0x73,
	0x74, 0x46, 0x72, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x79, 0x53, 0x63, 0x61, 0x6c, 0x65, 0x72,
	0x2e, 0x53, 0x63, 0x61, 0x6c, 0x65, 0x72, 0x2e, 0x53, 0x63, 0x61, 0x6c, 0x65, 0x72, 0x53, 0x74,
	0x61, 0x74, 0x65, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x22, 0x47, 0x0a, 0x0b, 0x53, 0x63,
	0x61, 0x6c, 0x65, 0x72, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x08, 0x0a, 0x04, 0x4e, 0x4f, 0x4e,
	0x45, 0x10, 0x00, 0x12, 0x0e, 0x0a, 0x0a, 0x50, 0x4f, 0x57, 0x45, 0x52, 0x5f, 0x53, 0x41, 0x56,
	0x45, 0x10, 0x01, 0x12, 0x0f, 0x0a, 0x0b, 0x50, 0x45, 0x52, 0x46, 0x4f, 0x52, 0x4d, 0x41, 0x4e,
	0x43, 0x45, 0x10, 0x02, 0x12, 0x0d, 0x0a, 0x09, 0x53, 0x43, 0x48, 0x45, 0x44, 0x55, 0x54, 0x49,
	0x4c, 0x10, 0x03, 0x42, 0x17, 0x5a, 0x15, 0x2e, 0x3b, 0x68, 0x6f, 0x73, 0x74, 0x66, 0x72, 0x65,
	0x71, 0x75, 0x65, 0x6e, 0x63, 0x79, 0x73, 0x63, 0x61, 0x6c, 0x65, 0x72, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_hostfrequencyscaler_proto_rawDescOnce sync.Once
	file_hostfrequencyscaler_proto_rawDescData = file_hostfrequencyscaler_proto_rawDesc
)

func file_hostfrequencyscaler_proto_rawDescGZIP() []byte {
	file_hostfrequencyscaler_proto_rawDescOnce.Do(func() {
		file_hostfrequencyscaler_proto_rawDescData = protoimpl.X.CompressGZIP(file_hostfrequencyscaler_proto_rawDescData)
	})
	return file_hostfrequencyscaler_proto_rawDescData
}

var file_hostfrequencyscaler_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_hostfrequencyscaler_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_hostfrequencyscaler_proto_goTypes = []interface{}{
	(Scaler_ScalerState)(0), // 0: HostFrequencyScaler.Scaler.ScalerState
	(*Scaler)(nil),          // 1: HostFrequencyScaler.Scaler
}
var file_hostfrequencyscaler_proto_depIdxs = []int32{
	0, // 0: HostFrequencyScaler.Scaler.state:type_name -> HostFrequencyScaler.Scaler.ScalerState
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_hostfrequencyscaler_proto_init() }
func file_hostfrequencyscaler_proto_init() {
	if File_hostfrequencyscaler_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_hostfrequencyscaler_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Scaler); i {
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
			RawDescriptor: file_hostfrequencyscaler_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_hostfrequencyscaler_proto_goTypes,
		DependencyIndexes: file_hostfrequencyscaler_proto_depIdxs,
		EnumInfos:         file_hostfrequencyscaler_proto_enumTypes,
		MessageInfos:      file_hostfrequencyscaler_proto_msgTypes,
	}.Build()
	File_hostfrequencyscaler_proto = out.File
	file_hostfrequencyscaler_proto_rawDesc = nil
	file_hostfrequencyscaler_proto_goTypes = nil
	file_hostfrequencyscaler_proto_depIdxs = nil
}
