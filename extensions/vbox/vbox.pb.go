// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: vbox.proto

package vbox

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	golang_proto "github.com/golang/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = golang_proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type VirtualMachine struct {
	ApiServer            string   `protobuf:"bytes,1,opt,name=api_server,json=apiServer,proto3" json:"api_server,omitempty"`
	VmName               string   `protobuf:"bytes,2,opt,name=vm_name,json=vmName,proto3" json:"vm_name,omitempty"`
	Uuid                 string   `protobuf:"bytes,3,opt,name=uuid,proto3" json:"uuid,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *VirtualMachine) Reset()         { *m = VirtualMachine{} }
func (m *VirtualMachine) String() string { return proto.CompactTextString(m) }
func (*VirtualMachine) ProtoMessage()    {}
func (*VirtualMachine) Descriptor() ([]byte, []int) {
	return fileDescriptor_16145ecf1ae9b3d9, []int{0}
}
func (m *VirtualMachine) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *VirtualMachine) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_VirtualMachine.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *VirtualMachine) XXX_Merge(src proto.Message) {
	xxx_messageInfo_VirtualMachine.Merge(m, src)
}
func (m *VirtualMachine) XXX_Size() int {
	return m.Size()
}
func (m *VirtualMachine) XXX_DiscardUnknown() {
	xxx_messageInfo_VirtualMachine.DiscardUnknown(m)
}

var xxx_messageInfo_VirtualMachine proto.InternalMessageInfo

func (m *VirtualMachine) GetApiServer() string {
	if m != nil {
		return m.ApiServer
	}
	return ""
}

func (m *VirtualMachine) GetVmName() string {
	if m != nil {
		return m.VmName
	}
	return ""
}

func (m *VirtualMachine) GetUuid() string {
	if m != nil {
		return m.Uuid
	}
	return ""
}

func (*VirtualMachine) XXX_MessageName() string {
	return "VBox.VirtualMachine"
}
func init() {
	proto.RegisterType((*VirtualMachine)(nil), "VBox.VirtualMachine")
	golang_proto.RegisterType((*VirtualMachine)(nil), "VBox.VirtualMachine")
}

func init() { proto.RegisterFile("vbox.proto", fileDescriptor_16145ecf1ae9b3d9) }
func init() { golang_proto.RegisterFile("vbox.proto", fileDescriptor_16145ecf1ae9b3d9) }

var fileDescriptor_16145ecf1ae9b3d9 = []byte{
	// 188 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x2a, 0x4b, 0xca, 0xaf,
	0xd0, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x09, 0x73, 0xca, 0xaf, 0x90, 0xd2, 0x4d, 0xcf,
	0x2c, 0xc9, 0x28, 0x4d, 0xd2, 0x4b, 0xce, 0xcf, 0xd5, 0x4f, 0xcf, 0x4f, 0xcf, 0xd7, 0x07, 0x4b,
	0x26, 0x95, 0xa6, 0x81, 0x79, 0x60, 0x0e, 0x98, 0x05, 0xd1, 0xa4, 0x14, 0xc3, 0xc5, 0x17, 0x96,
	0x59, 0x54, 0x52, 0x9a, 0x98, 0xe3, 0x9b, 0x98, 0x9c, 0x91, 0x99, 0x97, 0x2a, 0x24, 0xcb, 0xc5,
	0x95, 0x58, 0x90, 0x19, 0x5f, 0x9c, 0x5a, 0x54, 0x96, 0x5a, 0x24, 0xc1, 0xa8, 0xc0, 0xa8, 0xc1,
	0x19, 0xc4, 0x99, 0x58, 0x90, 0x19, 0x0c, 0x16, 0x10, 0x12, 0xe7, 0x62, 0x2f, 0xcb, 0x8d, 0xcf,
	0x4b, 0xcc, 0x4d, 0x95, 0x60, 0x02, 0xcb, 0xb1, 0x95, 0xe5, 0xfa, 0x25, 0xe6, 0xa6, 0x0a, 0x09,
	0x71, 0xb1, 0x94, 0x96, 0x66, 0xa6, 0x48, 0x30, 0x83, 0x45, 0xc1, 0x6c, 0x27, 0x99, 0x13, 0x8f,
	0xe4, 0x18, 0x2f, 0x3c, 0x92, 0x63, 0x7c, 0xf0, 0x48, 0x8e, 0xf1, 0xc0, 0x63, 0x39, 0xc6, 0x13,
	0x8f, 0xe5, 0x18, 0xa3, 0xd8, 0xf4, 0xac, 0x41, 0xce, 0x4e, 0x62, 0x03, 0x3b, 0xc1, 0x18, 0x10,
	0x00, 0x00, 0xff, 0xff, 0x59, 0xec, 0xe9, 0x6c, 0xc5, 0x00, 0x00, 0x00,
}

func (m *VirtualMachine) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *VirtualMachine) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *VirtualMachine) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Uuid) > 0 {
		i -= len(m.Uuid)
		copy(dAtA[i:], m.Uuid)
		i = encodeVarintVbox(dAtA, i, uint64(len(m.Uuid)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.VmName) > 0 {
		i -= len(m.VmName)
		copy(dAtA[i:], m.VmName)
		i = encodeVarintVbox(dAtA, i, uint64(len(m.VmName)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.ApiServer) > 0 {
		i -= len(m.ApiServer)
		copy(dAtA[i:], m.ApiServer)
		i = encodeVarintVbox(dAtA, i, uint64(len(m.ApiServer)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintVbox(dAtA []byte, offset int, v uint64) int {
	offset -= sovVbox(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *VirtualMachine) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.ApiServer)
	if l > 0 {
		n += 1 + l + sovVbox(uint64(l))
	}
	l = len(m.VmName)
	if l > 0 {
		n += 1 + l + sovVbox(uint64(l))
	}
	l = len(m.Uuid)
	if l > 0 {
		n += 1 + l + sovVbox(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovVbox(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozVbox(x uint64) (n int) {
	return sovVbox(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *VirtualMachine) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowVbox
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: VirtualMachine: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: VirtualMachine: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ApiServer", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVbox
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVbox
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVbox
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ApiServer = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field VmName", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVbox
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVbox
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVbox
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.VmName = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Uuid", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVbox
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVbox
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVbox
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Uuid = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipVbox(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthVbox
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipVbox(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowVbox
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowVbox
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowVbox
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthVbox
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupVbox
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthVbox
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthVbox        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowVbox          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupVbox = fmt.Errorf("proto: unexpected end of group")
)