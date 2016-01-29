package gremlin

import (
	"github.com/satori/go.uuid"
)

type Request struct {
	RequestId string       `json:"requestId"`
	Op        string       `json:"op"`
	Processor string       `json:"processor"`
	Args      *RequestArgs `json:"args"`
}

type RequestArgs struct {
	Gremlin    string `json:"gremlin,omitempty"`
	Session    string `json:"session,omitempty"`
	Bindings   Bind   `json:"bindings,omitempty"`
	Language   string `json:"language,omitempty"`
	Rebindings Bind   `json:"rebindings,omitempty"`
	Sasl       []byte `json:"sasl,omitempty"`
	BatchSize  int    `json:"batchSize,omitempty"`
}

type Bind map[string]interface{}

func Query(query string) *Request {
	args := &RequestArgs{
		Gremlin:  query,
		Language: "gremlin-groovy",
	}
	req := &Request{
		RequestId: uuid.NewV4().String(),
		Op:        "eval",
		Processor: "",
		Args:      args,
	}
	return req
}

func (req *Request) Bindings(bindings Bind) *Request {
	req.Args.Bindings = bindings
	return req
}
