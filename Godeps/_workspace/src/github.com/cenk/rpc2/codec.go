package rpc2

import (
	"bufio"
	"encoding/gob"
	"io"
	"sync"
)

// A Codec implements reading and writing of RPC requests and responses.
// The client calls ReadHeader to read a message header.
// The implementation must populate either Request or Response argument.
// Depending on which argument is populated, ReadRequestBody or
// ReadResponseBody is called right after ReadHeader.
// ReadRequestBody and ReadResponseBody may be called with a nil
// argument to force the body to be read and then discarded.
type Codec interface {
	// ReadHeader must read a message and populate either the request
	// or the response by inspecting the incoming message.
	ReadHeader(*Request, *Response) error

	// ReadRequestBody into args argument of handler function.
	ReadRequestBody(interface{}) error

	// ReadResponseBody into reply argument of handler function.
	ReadResponseBody(interface{}) error

	// WriteRequest must be safe for concurrent use by multiple goroutines.
	WriteRequest(*Request, interface{}) error

	// WriteResponse must be safe for concurrent use by multiple goroutines.
	WriteResponse(*Response, interface{}) error

	// Close is called when client/server finished with the connection.
	Close() error
}

// Request is a header written before every RPC call.
type Request struct {
	Seq    uint64 // sequence number chosen by client
	Method string
}

// Response is a header written before every RPC return.
type Response struct {
	Seq   uint64 // echoes that of the request
	Error string // error, if any.
}

type gobCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
	mutex  sync.Mutex
}

type message struct {
	Seq    uint64
	Method string
	Error  string
}

func newGobCodec(conn io.ReadWriteCloser) *gobCodec {
	buf := bufio.NewWriter(conn)
	return &gobCodec{
		rwc:    conn,
		dec:    gob.NewDecoder(conn),
		enc:    gob.NewEncoder(buf),
		encBuf: buf,
	}
}

func (c *gobCodec) ReadHeader(req *Request, resp *Response) error {
	var msg message
	if err := c.dec.Decode(&msg); err != nil {
		return err
	}

	if msg.Method != "" {
		req.Seq = msg.Seq
		req.Method = msg.Method
	} else {
		resp.Seq = msg.Seq
		resp.Error = msg.Error
	}
	return nil
}

func (c *gobCodec) ReadRequestBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *gobCodec) ReadResponseBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *gobCodec) WriteRequest(r *Request, body interface{}) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err = c.enc.Encode(r); err != nil {
		return
	}
	if err = c.enc.Encode(body); err != nil {
		return
	}
	return c.encBuf.Flush()
}

func (c *gobCodec) WriteResponse(r *Response, body interface{}) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err = c.enc.Encode(r); err != nil {
		return
	}
	if err = c.enc.Encode(body); err != nil {
		return
	}
	return c.encBuf.Flush()
}

func (c *gobCodec) Close() error {
	return c.rwc.Close()
}
