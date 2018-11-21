// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: websocket/structmessage.proto

package websocket

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

// StructMessage is a Protobuf based message on top of Message.
// It implements Message interface and can be sent with via a Speaker.
type StructMessageProtobuf struct {
	Namespace string `protobuf:"bytes,1,opt,name=Namespace,proto3" json:"Namespace,omitempty"`
	Type      string `protobuf:"bytes,2,opt,name=Type,proto3" json:"Type,omitempty"`
	UUID      string `protobuf:"bytes,3,opt,name=UUID,proto3" json:"UUID,omitempty"`
	Status    int64  `protobuf:"varint,4,opt,name=Status,proto3" json:"Status,omitempty"`
	Obj       []byte `protobuf:"bytes,5,opt,name=Obj,proto3" json:"Obj,omitempty"`
}

func (m *StructMessageProtobuf) Reset()         { *m = StructMessageProtobuf{} }
func (m *StructMessageProtobuf) String() string { return proto.CompactTextString(m) }
func (*StructMessageProtobuf) ProtoMessage()    {}
func (*StructMessageProtobuf) Descriptor() ([]byte, []int) {
	return fileDescriptor_structmessage_a11b611d9605b3ab, []int{0}
}
func (m *StructMessageProtobuf) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *StructMessageProtobuf) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_StructMessageProtobuf.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (dst *StructMessageProtobuf) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StructMessageProtobuf.Merge(dst, src)
}
func (m *StructMessageProtobuf) XXX_Size() int {
	return m.Size()
}
func (m *StructMessageProtobuf) XXX_DiscardUnknown() {
	xxx_messageInfo_StructMessageProtobuf.DiscardUnknown(m)
}

var xxx_messageInfo_StructMessageProtobuf proto.InternalMessageInfo

func (m *StructMessageProtobuf) GetNamespace() string {
	if m != nil {
		return m.Namespace
	}
	return ""
}

func (m *StructMessageProtobuf) GetType() string {
	if m != nil {
		return m.Type
	}
	return ""
}

func (m *StructMessageProtobuf) GetUUID() string {
	if m != nil {
		return m.UUID
	}
	return ""
}

func (m *StructMessageProtobuf) GetStatus() int64 {
	if m != nil {
		return m.Status
	}
	return 0
}

func (m *StructMessageProtobuf) GetObj() []byte {
	if m != nil {
		return m.Obj
	}
	return nil
}

func init() {
	proto.RegisterType((*StructMessageProtobuf)(nil), "websocket.StructMessageProtobuf")
}
func (m *StructMessageProtobuf) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *StructMessageProtobuf) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Namespace) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintStructmessage(dAtA, i, uint64(len(m.Namespace)))
		i += copy(dAtA[i:], m.Namespace)
	}
	if len(m.Type) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintStructmessage(dAtA, i, uint64(len(m.Type)))
		i += copy(dAtA[i:], m.Type)
	}
	if len(m.UUID) > 0 {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintStructmessage(dAtA, i, uint64(len(m.UUID)))
		i += copy(dAtA[i:], m.UUID)
	}
	if m.Status != 0 {
		dAtA[i] = 0x20
		i++
		i = encodeVarintStructmessage(dAtA, i, uint64(m.Status))
	}
	if len(m.Obj) > 0 {
		dAtA[i] = 0x2a
		i++
		i = encodeVarintStructmessage(dAtA, i, uint64(len(m.Obj)))
		i += copy(dAtA[i:], m.Obj)
	}
	return i, nil
}

func encodeVarintStructmessage(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *StructMessageProtobuf) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Namespace)
	if l > 0 {
		n += 1 + l + sovStructmessage(uint64(l))
	}
	l = len(m.Type)
	if l > 0 {
		n += 1 + l + sovStructmessage(uint64(l))
	}
	l = len(m.UUID)
	if l > 0 {
		n += 1 + l + sovStructmessage(uint64(l))
	}
	if m.Status != 0 {
		n += 1 + sovStructmessage(uint64(m.Status))
	}
	l = len(m.Obj)
	if l > 0 {
		n += 1 + l + sovStructmessage(uint64(l))
	}
	return n
}

func sovStructmessage(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozStructmessage(x uint64) (n int) {
	return sovStructmessage(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *StructMessageProtobuf) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowStructmessage
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: StructMessageProtobuf: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: StructMessageProtobuf: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Namespace", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStructmessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthStructmessage
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Namespace = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStructmessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthStructmessage
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Type = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field UUID", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStructmessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthStructmessage
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.UUID = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Status", wireType)
			}
			m.Status = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStructmessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Status |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Obj", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStructmessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthStructmessage
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Obj = append(m.Obj[:0], dAtA[iNdEx:postIndex]...)
			if m.Obj == nil {
				m.Obj = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipStructmessage(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthStructmessage
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipStructmessage(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowStructmessage
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
					return 0, ErrIntOverflowStructmessage
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowStructmessage
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
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthStructmessage
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowStructmessage
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipStructmessage(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthStructmessage = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowStructmessage   = fmt.Errorf("proto: integer overflow")
)

func init() {
	proto.RegisterFile("websocket/structmessage.proto", fileDescriptor_structmessage_a11b611d9605b3ab)
}

var fileDescriptor_structmessage_a11b611d9605b3ab = []byte{
	// 190 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0x2d, 0x4f, 0x4d, 0x2a,
	0xce, 0x4f, 0xce, 0x4e, 0x2d, 0xd1, 0x2f, 0x2e, 0x29, 0x2a, 0x4d, 0x2e, 0xc9, 0x4d, 0x2d, 0x2e,
	0x4e, 0x4c, 0x4f, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x84, 0x4b, 0x2b, 0xb5, 0x33,
	0x72, 0x89, 0x06, 0x83, 0x95, 0xf8, 0x42, 0x94, 0x04, 0x80, 0x54, 0x24, 0x95, 0xa6, 0x09, 0xc9,
	0x70, 0x71, 0xfa, 0x25, 0xe6, 0xa6, 0x16, 0x17, 0x24, 0x26, 0xa7, 0x4a, 0x30, 0x2a, 0x30, 0x6a,
	0x70, 0x06, 0x21, 0x04, 0x84, 0x84, 0xb8, 0x58, 0x42, 0x2a, 0x0b, 0x52, 0x25, 0x98, 0xc0, 0x12,
	0x60, 0x36, 0x48, 0x2c, 0x34, 0xd4, 0xd3, 0x45, 0x82, 0x19, 0x22, 0x06, 0x62, 0x0b, 0x89, 0x71,
	0xb1, 0x05, 0x97, 0x24, 0x96, 0x94, 0x16, 0x4b, 0xb0, 0x28, 0x30, 0x6a, 0x30, 0x07, 0x41, 0x79,
	0x42, 0x02, 0x5c, 0xcc, 0xfe, 0x49, 0x59, 0x12, 0xac, 0x0a, 0x8c, 0x1a, 0x3c, 0x41, 0x20, 0xa6,
	0x93, 0xc4, 0x89, 0x47, 0x72, 0x8c, 0x17, 0x1e, 0xc9, 0x31, 0x3e, 0x78, 0x24, 0xc7, 0x38, 0xe1,
	0xb1, 0x1c, 0xc3, 0x85, 0xc7, 0x72, 0x0c, 0x37, 0x1e, 0xcb, 0x31, 0x24, 0xb1, 0x81, 0x5d, 0x6d,
	0x0c, 0x08, 0x00, 0x00, 0xff, 0xff, 0xb6, 0xa5, 0x19, 0x3d, 0xd6, 0x00, 0x00, 0x00,
}
