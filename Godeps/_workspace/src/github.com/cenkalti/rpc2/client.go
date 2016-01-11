// Package rpc2 provides bi-directional RPC client and server similar to net/rpc.
package rpc2

import (
	"errors"
	"io"
	"log"
	"reflect"
	"sync"
)

// Client represents an RPC Client.
// There may be multiple outstanding Calls associated
// with a single Client, and a Client may be used by
// multiple goroutines simultaneously.
type Client struct {
	mutex      sync.Mutex // protects pending, seq, request
	sending    sync.Mutex
	request    Request // temp area used in send()
	seq        uint64
	pending    map[uint64]*Call
	closing    bool
	shutdown   bool
	server     bool
	codec      Codec
	handlers   map[string]*handler
	disconnect chan struct{}
	State      *State // additional information to associate with client
}

// NewClient returns a new Client to handle requests to the
// set of services at the other end of the connection.
// It adds a buffer to the write side of the connection so
// the header and payload are sent as a unit.
func NewClient(conn io.ReadWriteCloser) *Client {
	return NewClientWithCodec(newGobCodec(conn))
}

// NewClientWithCodec is like NewClient but uses the specified
// codec to encode requests and decode responses.
func NewClientWithCodec(codec Codec) *Client {
	return &Client{
		codec:      codec,
		pending:    make(map[uint64]*Call),
		handlers:   make(map[string]*handler),
		disconnect: make(chan struct{}),
		seq:        1, // 0 means notification.
	}
}

// Run the client's read loop.
// You must run this method before calling any methods on the server.
func (c *Client) Run() {
	c.readLoop()
}

// DisconnectNotify returns a channel that is closed
// when the client connection has gone away.
func (c *Client) DisconnectNotify() chan struct{} {
	return c.disconnect
}

// Handle registers the handler function for the given method. If a handler already exists for method, Handle panics.
func (c *Client) Handle(method string, handlerFunc interface{}) {
	addHandler(c.handlers, method, handlerFunc)
}

// readLoop reads messages from codec.
// It reads a reqeust or a response to the previous request.
// If the message is request, calls the handler function.
// If the message is response, sends the reply to the associated call.
func (c *Client) readLoop() {
	var err error
	var req Request
	var resp Response
	for err == nil {
		req = Request{}
		resp = Response{}
		if err = c.codec.ReadHeader(&req, &resp); err != nil {
			break
		}

		if req.Method != "" {
			// request comes to server
			if err = c.readRequest(&req); err != nil {
				log.Println("rpc2: error reading request:", err.Error())
			}
		} else {
			// response comes to client
			if err = c.readResponse(&resp); err != nil {
				log.Println("rpc2: error reading response:", err.Error())
			}
		}
	}
	// Terminate pending calls.
	c.sending.Lock()
	c.mutex.Lock()
	c.shutdown = true
	closing := c.closing
	if err == io.EOF {
		if closing {
			err = ErrShutdown
		} else {
			err = io.ErrUnexpectedEOF
		}
	}
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
	c.mutex.Unlock()
	c.sending.Unlock()
	if err != io.EOF && !closing && !c.server {
		log.Println("rpc2: client protocol error:", err)
	}
	close(c.disconnect)
}

func (c *Client) readRequest(req *Request) error {
	method, ok := c.handlers[req.Method]
	if !ok {
		resp := &Response{
			Seq:   req.Seq,
			Error: "rpc2: can't find method " + req.Method,
		}
		return c.codec.WriteResponse(resp, resp)
	}

	// Decode the argument value.
	var argv reflect.Value
	argIsValue := false // if true, need to indirect before calling.
	if method.argType.Kind() == reflect.Ptr {
		argv = reflect.New(method.argType.Elem())
	} else {
		argv = reflect.New(method.argType)
		argIsValue = true
	}
	// argv guaranteed to be a pointer now.
	if err := c.codec.ReadRequestBody(argv.Interface()); err != nil {
		return err
	}
	if argIsValue {
		argv = argv.Elem()
	}

	// Invoke the method, providing a new value for the reply.
	replyv := reflect.New(method.replyType.Elem())

	// Call handler function.
	go func(req Request) {
		returnValues := method.fn.Call([]reflect.Value{reflect.ValueOf(c), argv, replyv})

		// Do not send response if request is a notification.
		if req.Seq == 0 {
			return
		}

		// The return value for the method is an error.
		errInter := returnValues[0].Interface()
		errmsg := ""
		if errInter != nil {
			errmsg = errInter.(error).Error()
		}
		resp := &Response{
			Seq:   req.Seq,
			Error: errmsg,
		}
		c.codec.WriteResponse(resp, replyv.Interface())
	}(*req)

	return nil
}

func (c *Client) readResponse(resp *Response) error {
	seq := resp.Seq
	c.mutex.Lock()
	call := c.pending[seq]
	delete(c.pending, seq)
	c.mutex.Unlock()

	var err error
	switch {
	case call == nil:
		// We've got no pending call. That usually means that
		// WriteRequest partially failed, and call was already
		// removed; response is a server telling us about an
		// error reading request body. We should still attempt
		// to read error body, but there's no one to give it to.
		err = c.codec.ReadResponseBody(nil)
		if err != nil {
			err = errors.New("reading error body: " + err.Error())
		}
	case resp.Error != "":
		// We've got an error response. Give this to the request;
		// any subsequent requests will get the ReadResponseBody
		// error if there is one.
		call.Error = ServerError(resp.Error)
		err = c.codec.ReadResponseBody(nil)
		if err != nil {
			err = errors.New("reading error body: " + err.Error())
		}
		call.done()
	default:
		err = c.codec.ReadResponseBody(call.Reply)
		if err != nil {
			call.Error = errors.New("reading body " + err.Error())
		}
		call.done()
	}

	return nil
}

// Close waits for active calls to finish and closes the codec.
func (c *Client) Close() error {
	c.mutex.Lock()
	if c.shutdown || c.closing {
		c.mutex.Unlock()
		return ErrShutdown
	}
	c.closing = true
	c.mutex.Unlock()
	return c.codec.Close()
}

// Go invokes the function asynchronously.  It returns the Call structure representing
// the invocation.  The done channel will signal when the call is complete by returning
// the same Call object.  If done is nil, Go will allocate a new channel.
// If non-nil, done must be buffered or Go will deliberately crash.
func (c *Client) Go(method string, args interface{}, reply interface{}, done chan *Call) *Call {
	call := new(Call)
	call.Method = method
	call.Args = args
	call.Reply = reply
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel.  If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("rpc2: done channel is unbuffered")
		}
	}
	call.Done = done
	c.send(call)
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (c *Client) Call(method string, args interface{}, reply interface{}) error {
	call := <-c.Go(method, args, reply, make(chan *Call, 1)).Done
	return call.Error
}

func (call *Call) done() {
	select {
	case call.Done <- call:
		// ok
	default:
		// We don't want to block here.  It is the caller's responsibility to make
		// sure the channel has enough buffer space. See comment in Go().
		log.Println("rpc2: discarding Call reply due to insufficient Done chan capacity")
	}
}

// ServerError represents an error that has been returned from
// the remote side of the RPC connection.
type ServerError string

func (e ServerError) Error() string {
	return string(e)
}

var ErrShutdown = errors.New("connection is shut down")

// Call represents an active RPC.
type Call struct {
	Method string      // The name of the service and method to call.
	Args   interface{} // The argument to the function (*struct).
	Reply  interface{} // The reply from the function (*struct).
	Error  error       // After completion, the error status.
	Done   chan *Call  // Strobes when call is complete.
}

func (c *Client) send(call *Call) {
	c.sending.Lock()
	defer c.sending.Unlock()

	// Register this call.
	c.mutex.Lock()
	if c.shutdown || c.closing {
		call.Error = ErrShutdown
		c.mutex.Unlock()
		call.done()
		return
	}
	seq := c.seq
	c.seq++
	c.pending[seq] = call
	c.mutex.Unlock()

	// Encode and send the request.
	c.request.Seq = seq
	c.request.Method = call.Method
	err := c.codec.WriteRequest(&c.request, call.Args)
	if err != nil {
		c.mutex.Lock()
		call = c.pending[seq]
		delete(c.pending, seq)
		c.mutex.Unlock()
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (c *Client) Notify(method string, args interface{}) error {
	c.sending.Lock()
	defer c.sending.Unlock()

	if c.shutdown || c.closing {
		return ErrShutdown
	}

	c.request.Seq = 0
	c.request.Method = method
	return c.codec.WriteRequest(&c.request, args)
}
