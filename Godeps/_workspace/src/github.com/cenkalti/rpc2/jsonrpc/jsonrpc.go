// Package jsonrpc implements a JSON-RPC ClientCodec and ServerCodec for the rpc2 package.
//
// Beside struct types, JSONCodec allows using positional arguments.
// Use []interface{} as the type of argument when sending and receiving methods.
//
// Positional arguments example:
// 	server.Handle("add", func(client *rpc2.Client, args []interface{}, result *float64) error {
// 		*result = args[0].(float64) + args[1].(float64)
// 		return nil
// 	})
//
//	var result float64
// 	client.Call("add", []interface{}{1, 2}, &result)
//
package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/cenkalti/rpc2"
)

type jsonCodec struct {
	dec *json.Decoder // for reading JSON values
	enc *json.Encoder // for writing JSON values
	c   io.Closer

	// temporary work space
	msg            message
	serverRequest  serverRequest
	clientResponse clientResponse

	// JSON-RPC clients can use arbitrary json values as request IDs.
	// Package rpc expects uint64 request IDs.
	// We assign uint64 sequence numbers to incoming requests
	// but save the original request ID in the pending map.
	// When rpc responds, we use the sequence number in
	// the response to find the original request ID.
	mutex   sync.Mutex // protects seq, pending
	pending map[uint64]*json.RawMessage
	seq     uint64
}

// NewJSONCodec returns a new rpc2.Codec using JSON-RPC on conn.
func NewJSONCodec(conn io.ReadWriteCloser) rpc2.Codec {
	return &jsonCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]*json.RawMessage),
	}
}

// serverRequest and clientResponse combined
type message struct {
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params"`
	Id     *json.RawMessage `json:"id"`
	Result *json.RawMessage `json:"result"`
	Error  interface{}      `json:"error"`
}

// Unmarshal to
type serverRequest struct {
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params"`
	Id     *json.RawMessage `json:"id"`
}
type clientResponse struct {
	Id     uint64           `json:"id"`
	Result *json.RawMessage `json:"result"`
	Error  interface{}      `json:"error"`
}

// to Marshal
type serverResponse struct {
	Id     *json.RawMessage `json:"id"`
	Result interface{}      `json:"result"`
	Error  interface{}      `json:"error"`
}
type clientRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
	Id     *uint64       `json:"id"`
}

func (c *jsonCodec) ReadHeader(req *rpc2.Request, resp *rpc2.Response) error {
	c.msg = message{}
	if err := c.dec.Decode(&c.msg); err != nil {
		return err
	}

	if c.msg.Method != "" {
		// request comes to server
		c.serverRequest.Id = c.msg.Id
		c.serverRequest.Method = c.msg.Method
		c.serverRequest.Params = c.msg.Params

		req.Method = c.serverRequest.Method

		// JSON request id can be any JSON value;
		// RPC package expects uint64.  Translate to
		// internal uint64 and save JSON on the side.
		if c.serverRequest.Id == nil {
			// Notification
		} else {
			c.mutex.Lock()
			c.seq++
			c.pending[c.seq] = c.serverRequest.Id
			c.serverRequest.Id = nil
			req.Seq = c.seq
			c.mutex.Unlock()
		}
	} else {
		// response comes to client
		err := json.Unmarshal(*c.msg.Id, &c.clientResponse.Id)
		if err != nil {
			return err
		}
		c.clientResponse.Result = c.msg.Result
		c.clientResponse.Error = c.msg.Error

		resp.Error = ""
		resp.Seq = c.clientResponse.Id
		if c.clientResponse.Error != nil || c.clientResponse.Result == nil {
			x, ok := c.clientResponse.Error.(string)
			if !ok {
				return fmt.Errorf("invalid error %v", c.clientResponse.Error)
			}
			if x == "" {
				x = "unspecified error"
			}
			resp.Error = x
		}
	}
	return nil
}

var errMissingParams = errors.New("jsonrpc: request body missing params")

func (c *jsonCodec) ReadRequestBody(x interface{}) error {
	if x == nil {
		return nil
	}
	if c.serverRequest.Params == nil {
		return errMissingParams
	}
	var params *[]interface{}
	switch x := x.(type) {
	case *[]interface{}:
		params = x
	default:
		params = &[]interface{}{x}
	}
	return json.Unmarshal(*c.serverRequest.Params, params)
}

func (c *jsonCodec) ReadResponseBody(x interface{}) error {
	if x == nil {
		return nil
	}
	return json.Unmarshal(*c.clientResponse.Result, x)
}

func (c *jsonCodec) WriteRequest(r *rpc2.Request, param interface{}) error {
	req := &clientRequest{Method: r.Method}
	switch param := param.(type) {
	case []interface{}:
		req.Params = param
	default:
		req.Params = []interface{}{param}
	}
	if r.Seq == 0 {
		// Notification
		req.Id = nil
	} else {
		seq := r.Seq
		req.Id = &seq
	}
	return c.enc.Encode(req)
}

var null = json.RawMessage([]byte("null"))

func (c *jsonCodec) WriteResponse(r *rpc2.Response, x interface{}) error {
	c.mutex.Lock()
	b, ok := c.pending[r.Seq]
	if !ok {
		c.mutex.Unlock()
		return errors.New("invalid sequence number in response")
	}
	delete(c.pending, r.Seq)
	c.mutex.Unlock()

	if b == nil {
		// Invalid request so no id.  Use JSON null.
		b = &null
	}
	resp := serverResponse{Id: b}
	if r.Error == "" {
		resp.Result = x
	} else {
		resp.Error = r.Error
	}
	return c.enc.Encode(resp)
}

func (c *jsonCodec) Close() error {
	return c.c.Close()
}
