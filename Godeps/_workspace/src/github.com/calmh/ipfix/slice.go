package ipfix

import "encoding/binary"

type slice struct {
	bs  []byte
	err error
}

func newSlice(bs []byte) *slice {
	return &slice{
		bs: bs,
	}
}

// Cut returns the first length bytes from the slice and winds the slice
// forward by the same amount.
func (s *slice) Cut(length int) []byte {
	if len(s.bs) < length {
		length = len(s.bs)
		s.err = ErrRead
	}
	val := s.bs[:length]
	s.bs = s.bs[length:]
	return val
}

func (s *slice) Uint8() uint8 {
	if len(s.bs) < 1 {
		s.err = ErrRead
		return 0
	}
	v := s.bs[0]
	s.bs = s.bs[1:]
	return v
}

func (s *slice) Uint16() uint16 {
	if len(s.bs) < 2 {
		s.err = ErrRead
		return 0
	}
	v := binary.BigEndian.Uint16(s.bs)
	s.bs = s.bs[2:]
	return v
}

func (s *slice) Uint32() uint32 {
	if len(s.bs) < 4 {
		s.err = ErrRead
		return 0
	}
	v := binary.BigEndian.Uint32(s.bs)
	s.bs = s.bs[4:]
	return v
}

func (s *slice) Len() int {
	return len(s.bs)
}

func (s *slice) Error() error {
	return s.err
}

func (s *slice) bytes() []byte {
	return s.bs
}
