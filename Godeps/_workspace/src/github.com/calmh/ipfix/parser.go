package ipfix

import (
	"errors"
	"io"
	"sync"
)

// The version field in IPFIX messages should always have the value 10. If it
// does not, you get this error. It's probably a sign of a bug in the parser or
// the exporter and that we have lost synchronization with the data stream.
// Reestablishing the session is the only way forward at this point.
var ErrVersion = errors.New("incorrect version field in message header - out of sync?")

// ErrRead is returned when a packet is not long enough for the field it is
// supposed to contain. This is a sign of an earlier read error or a corrupted
// packet.
var ErrRead = errors.New("short read - malformed packet?")

// ErrProtocol is returned when impossible values that constitute a protocol
// error are encountered.
var ErrProtocol = errors.New("protocol error")

// A Message is the top level construct representing an IPFIX message. A well
// formed message contains one or more sets of data or template information.
type Message struct {
	Header          MessageHeader
	DataRecords     []DataRecord
	TemplateRecords []TemplateRecord
}

// The MessageHeader provides metadata for the entire Message. The sequence
// number and domain ID can be used to gain knowledge of messages lost on an
// unreliable transport such as UDP.
type MessageHeader struct {
	Version        uint16 // Always 0x0a
	Length         uint16
	ExportTime     uint32 // Epoch seconds
	SequenceNumber uint32
	DomainID       uint32
}

func (h *MessageHeader) unmarshal(s *slice) {
	h.Version = s.Uint16()
	h.Length = s.Uint16()
	h.ExportTime = s.Uint32()
	h.SequenceNumber = s.Uint32()
	h.DomainID = s.Uint32()
}

type setHeader struct {
	SetID  uint16
	Length uint16
}

func (h *setHeader) unmarshal(s *slice) {
	h.SetID = s.Uint16()
	h.Length = s.Uint16()
}

type templateHeader struct {
	TemplateID uint16
	FieldCount uint16
}

func (h *templateHeader) unmarshal(s *slice) {
	h.TemplateID = s.Uint16()
	h.FieldCount = s.Uint16()
}

// The DataRecord represents a single exported flow. The Fields each describe
// different aspects of the flow (source and destination address, counters,
// service, etc.).
type DataRecord struct {
	TemplateID uint16
	Fields     [][]byte
}

// The TemplateRecord describes a data template, as used by DataRecords.
type TemplateRecord struct {
	TemplateID      uint16
	FieldSpecifiers []TemplateFieldSpecifier
}

// The TemplateFieldSpecifier describes the ID and size of the corresponding
// Fields in a DataRecord.
type TemplateFieldSpecifier struct {
	EnterpriseID uint32
	FieldID      uint16
	Length       uint16
}

// The Session is the context for IPFIX messages.
type Session struct {
	buffers *sync.Pool

	mut       sync.RWMutex
	templates [][]TemplateFieldSpecifier
	minRecord []uint16
}

// NewSession initializes a new Session based on the provided io.Reader.
func NewSession() *Session {
	var s Session
	s.templates = make([][]TemplateFieldSpecifier, 65536)
	s.minRecord = make([]uint16, 65536)
	s.buffers = &sync.Pool{
		New: func() interface{} {
			return make([]byte, 65536)
		},
	}
	return &s
}

const (
	msgHeaderLength = 2 + 2 + 4 + 4 + 4
	setHeaderLength = 2 + 2
)

// ParseReader extracts and returns one message from the IPFIX stream. As long
// as err is nil, further messages can be read from the stream. Errors are not
// recoverable -- once an error has been returned, ParseReader should not be
// called again on the same session.
func (s *Session) ParseReader(r io.Reader) (Message, error) {
	bs := s.buffers.Get().([]byte)
	bs, hdr, err := Read(r, bs)
	if err != nil {
		return Message{}, err
	}

	sl := newSlice(bs[msgHeaderLength:])
	var msg Message
	msg.Header = hdr
	msg.TemplateRecords, msg.DataRecords, err = s.readBuffer(sl)
	s.buffers.Put(bs)
	return msg, err
}

// ParseBuffer extracts one message from the given buffer and returns it. Err
// is nil if the buffer could be parsed correctly. ParseBuffer is goroutine safe.
func (s *Session) ParseBuffer(bs []byte) (Message, error) {
	var msg Message
	var err error

	sl := newSlice(bs)
	msg.Header.unmarshal(sl)
	msg.TemplateRecords, msg.DataRecords, err = s.readBuffer(sl)
	return msg, err
}

func (s *Session) readBuffer(sl *slice) ([]TemplateRecord, []DataRecord, error) {
	var ts, trecs []TemplateRecord
	var ds, drecs []DataRecord
	var err error

	for sl.Len() > 0 {
		// Read a set header
		var setHdr setHeader
		setHdr.unmarshal(sl)

		if debug {
			dl.Printf("setHdr: %+v", setHdr)
		}

		if setHdr.Length < setHeaderLength {
			// Set cannot be shorter than its header
			if debug {
				dl.Println("setHdr too short")
			}
			return nil, nil, io.ErrUnexpectedEOF
		}

		// Grab the bytes representing the set
		setLen := int(setHdr.Length) - setHeaderLength
		setSl := newSlice(sl.Cut(setLen))
		if err := sl.Error(); err != nil {
			if debug {
				dl.Println("slice error")
			}
			return nil, nil, err
		}

		// Parse them
		ts, ds, err = s.readSet(setHdr, setSl)
		if err != nil {
			if debug {
				dl.Println("readSet:", err)
			}
			return nil, nil, err
		}

		trecs = append(trecs, ts...)
		drecs = append(drecs, ds...)
	}

	return trecs, drecs, nil
}

func (s *Session) readSet(setHdr setHeader, sl *slice) ([]TemplateRecord, []DataRecord, error) {
	var trecs []TemplateRecord
	var drecs []DataRecord

	s.mut.RLock()
	minLength := int(s.minRecord[setHdr.SetID])
	s.mut.RUnlock()

	for sl.Len() > 0 && sl.Error() == nil {
		if sl.Len() < minLength {
			if debug {
				dl.Println("ignoring padding")
			}
			// Padding
			return trecs, drecs, sl.Error()
		}

		// Set ID
		//
		// Identifies the Set.  A value of 2 is reserved for Template Sets. A
		// value of 3 is reserved for Options Template Sets.  Values from 4 to
		// 255 are reserved for future use.  Values 256 and above are used for
		// Data Sets.  The Set ID values of 0 and 1 are not used, for
		// historical reasons [RFC3954].

		switch {
		case setHdr.SetID < 2:
			// Unused, shouldn't happen
			if debug {
				dl.Println("bad SetID", setHdr.SetID)
			}
			return nil, nil, ErrProtocol

		case setHdr.SetID == 2:
			// Template Set
			if debug {
				dl.Println("parsing template set")
			}
			tr := s.readTemplateRecord(sl)
			trecs = append(trecs, tr)

			s.registerTemplateRecord(tr)
			if debug {
				dl.Printf("registered template: %+v", tr)
			}

		case setHdr.SetID == 3:
			// Options Template Set, not handled
			if debug {
				dl.Println("skipping option template set")
			}
			sl.Cut(sl.Len())

		case setHdr.SetID > 3 && setHdr.SetID < 256:
			// Reserved, shouldn't happen
			if debug {
				dl.Println("bad SetID", setHdr.SetID)
			}
			return nil, nil, ErrProtocol

		default:
			// Data set
			if debug {
				dl.Println("parsing data set")
			}

			s.mut.RLock()
			tpl := s.templates[setHdr.SetID]
			s.mut.RUnlock()

			if tpl != nil {
				// Data set
				ds, err := s.readDataRecord(sl, tpl)
				if err != nil {
					return nil, nil, err
				}
				ds.TemplateID = setHdr.SetID
				drecs = append(drecs, ds)
			} else {
				// Data set with unknown template
				// We can't trust set length, because we might be out of sync.
				// Consume rest of message.
				return trecs, drecs, sl.Error()
			}
		}
	}

	return trecs, drecs, sl.Error()
}

func (s *Session) readDataRecord(sl *slice, tpl []TemplateFieldSpecifier) (DataRecord, error) {
	var dr DataRecord
	dr.Fields = make([][]byte, len(tpl))

	var err error
	total := 0
	for i := range tpl {
		var val []byte
		if tpl[i].Length == 65535 {
			val, err = s.readVariableLength(sl)
			if err != nil {
				return DataRecord{}, err
			}
		} else {
			l := int(tpl[i].Length)
			val = sl.Cut(l)
		}
		dr.Fields[i] = val
		total += len(val)
	}

	// The loop above keeps slices of the original buffer. But that buffer
	// will be recycled so we need to copy them to separate storage. It's more
	// efficient to do it this way, with a single allocation at the end than
	// doing individual allocations along the way.

	cp := make([]byte, total)
	next := 0
	for i := range dr.Fields {
		ln := copy(cp[next:], dr.Fields[i])
		dr.Fields[i] = cp[next : next+ln]
		next += ln
	}

	return dr, sl.Error()
}

func (s *Session) readTemplateRecord(sl *slice) TemplateRecord {
	var th templateHeader
	th.unmarshal(sl)
	if debug {
		dl.Printf("templateHeader: %+v", th)
	}

	var tr TemplateRecord
	tr.TemplateID = th.TemplateID
	tr.FieldSpecifiers = make([]TemplateFieldSpecifier, th.FieldCount)
	for i := 0; i < int(th.FieldCount); i++ {
		f := TemplateFieldSpecifier{}
		f.FieldID = sl.Uint16()
		f.Length = sl.Uint16()
		if f.FieldID >= 0x8000 {
			f.FieldID -= 0x8000
			f.EnterpriseID = sl.Uint32()
		}
		tr.FieldSpecifiers[i] = f
	}

	return tr
}

func (s *Session) readVariableLength(sl *slice) (val []byte, err error) {
	var l int

	l0 := sl.Uint8()
	if l0 < 255 {
		l = int(l0)
	} else {
		l = int(sl.Uint16())
	}

	return sl.Cut(l), sl.Error()
}

func (s *Session) registerTemplateRecord(tr TemplateRecord) {
	// Update the template cache
	tid := tr.TemplateID

	// Calculate the minimum possible record length
	var minLength uint16
	for i := range tr.FieldSpecifiers {
		if tr.FieldSpecifiers[i].Length == 65535 {
			minLength++
		} else {
			minLength += tr.FieldSpecifiers[i].Length
		}
	}

	// Update templates and minimum record cache
	s.mut.Lock()
	if len(tr.FieldSpecifiers) == 0 {
		// Set was withdrawn
		s.templates[tid] = nil
	} else {
		s.templates[tid] = tr.FieldSpecifiers
	}
	s.minRecord[tid] = minLength
	s.mut.Unlock()
}
