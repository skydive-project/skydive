package ipfix

import "io"

// Read reads and returns an IPFIX message, the parsed message header and
// an error or nil. The given byte slice is used and returned (sliced to the
// message length) if it is large enough to contain the message; otherwise a
// new slice is allocated. The returned message slice contains the message
// header.
func Read(r io.Reader, bs []byte) ([]byte, MessageHeader, error) {
	if len(bs) < msgHeaderLength {
		bs = make([]byte, 65536)
	}
	_, err := io.ReadFull(r, bs[:msgHeaderLength])
	if err != nil {
		return nil, MessageHeader{}, err
	}

	var hdr MessageHeader
	hdr.unmarshal(newSlice(bs))

	if hdr.Version != 10 {
		return nil, hdr, ErrVersion
	}
	if len(bs) < int(hdr.Length) {
		newBs := make([]byte, 65536)
		copy(newBs, bs[:msgHeaderLength])
		bs = newBs
	}

	if hdr.Length < msgHeaderLength {
		// Message can't be shorter than its header
		return nil, hdr, io.ErrUnexpectedEOF
	}

	bs = bs[:int(hdr.Length)]
	_, err = io.ReadFull(r, bs[msgHeaderLength:])
	if err != nil {
		return nil, hdr, err
	}

	return bs, hdr, nil
}
