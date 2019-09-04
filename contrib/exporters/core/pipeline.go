/*
 * Copyright (C) 2019 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	ws "github.com/skydive-project/skydive/websocket"
)

// CfgRoot configuration root path
const CfgRoot = "pipeline."

// Transformer allows generic transformations of a flow
type Transformer interface {
	// Transform transforms a flow before being stored
	Transform(f *flow.Flow) interface{}
}

// Classifier exposes the interface for tag based classification
type Classifier interface {
	GetFlowTag(fl *flow.Flow) Tag
}

// Filterer exposes the interface for tag based filtering
type Filterer interface {
	IsExcluded(tag Tag) bool
}

// Encoder exposes the interface for encoding flows
type Encoder interface {
	Encode(in interface{}) ([]byte, error)
}

// Compressor exposes the interface for compressesing encoded flows
type Compressor interface {
	Compress(b []byte) (*bytes.Buffer, error)
}

// Storer interface of a store object
type Storer interface {
	StoreFlows(flows map[Tag][]interface{}) error
	SetPipeline(p *Pipeline)
}

// Writer allows uploading objects to an object storage service
type Writer interface {
	Write(bucket, objectKey, data, contentType, contentEncoding string, metadata map[string]*string) error
}

// Handler used for creating a phase handler from configuration
type Handler = func(cfg *viper.Viper) (interface{}, error)

// HandlersMap a map of handlers
type HandlersMap map[string]Handler

// Global set of handlers
var (
	TransformerHandlers HandlersMap
	ClassifierHandlers  HandlersMap
	FiltererHandlers    HandlersMap
	EncoderHandlers     HandlersMap
	CompressorHandlers  HandlersMap
	StorerHandlers      HandlersMap
	WriterHandlers      HandlersMap
)

// Register associates a handler with its' label
func (m HandlersMap) Register(name string, handler Handler, isDefault bool) {
	m[name] = handler
	if isDefault {
		m[""] = handler
	}
}

// Init creates resource from config
func (m HandlersMap) Init(cfg *viper.Viper, phase string) (interface{}, error) {
	ty := cfg.GetString(CfgRoot + fmt.Sprintf("%s.type", phase))
	for t, fn := range m {
		if ty == t {
			return fn(cfg)
		}
	}
	return nil, fmt.Errorf("%s type %s not supported", phase, ty)
}

func init() {
	TransformerHandlers = make(HandlersMap)
	TransformerHandlers.Register("none", NewTransformNone, true)

	ClassifierHandlers = make(HandlersMap)
	ClassifierHandlers.Register("subnet", NewClassifySubnet, true)

	FiltererHandlers = make(HandlersMap)
	FiltererHandlers.Register("subnet", NewFilterSubnet, true)

	EncoderHandlers = make(HandlersMap)
	EncoderHandlers.Register("json", NewEncodeJSON, true)
	EncoderHandlers.Register("csv", NewEncodeCSV, false)

	CompressorHandlers = make(HandlersMap)
	CompressorHandlers.Register("none", NewCompressNone, true)
	CompressorHandlers.Register("gzip", NewCompressGzip, false)

	StorerHandlers = make(HandlersMap)
	StorerHandlers.Register("buffered", NewStoreBuffered, true)
	StorerHandlers.Register("direct", NewStoreDirect, false)

	WriterHandlers = make(HandlersMap)
	WriterHandlers.Register("s3", NewWriteS3, true)
	WriterHandlers.Register("stdout", NewWriteStdout, false)
}

// Pipeline manager
type Pipeline struct {
	sync.Mutex

	Transformer Transformer
	Classifier  Classifier
	Filterer    Filterer
	Encoder     Encoder
	Compressor  Compressor
	Storer      Storer
	Writer      Writer
}

// NewPipeline defines the pipeline elements
func NewPipeline(cfg *viper.Viper) (*Pipeline, error) {
	transformer, err := TransformerHandlers.Init(cfg, "transform")
	if err != nil {
		return nil, err
	}

	classifier, err := ClassifierHandlers.Init(cfg, "classify")
	if err != nil {
		return nil, err
	}

	filterer, err := FiltererHandlers.Init(cfg, "filter")
	if err != nil {
		return nil, err
	}

	encoder, err := EncoderHandlers.Init(cfg, "encode")
	if err != nil {
		return nil, err
	}

	compressor, err := CompressorHandlers.Init(cfg, "compress")
	if err != nil {
		return nil, err
	}

	storer, err := StorerHandlers.Init(cfg, "store")
	if err != nil {
		return nil, err
	}

	writer, err := WriterHandlers.Init(cfg, "write")
	if err != nil {
		return nil, err
	}

	p := &Pipeline{
		Transformer: transformer.(Transformer),
		Classifier:  classifier.(Classifier),
		Filterer:    filterer.(Filterer),
		Encoder:     encoder.(Encoder),
		Compressor:  compressor.(Compressor),
		Storer:      storer.(Storer),
		Writer:      writer.(Writer),
	}
	storer.(Storer).SetPipeline(p)

	return p, nil
}

func (p *Pipeline) filter(in []*flow.Flow) (out []*flow.Flow) {
	for _, fl := range in {
		flowTag := p.Classifier.GetFlowTag(fl)

		if p.Filterer.IsExcluded(flowTag) {
			continue
		}

		out = append(out, fl)
	}
	return
}

func (p *Pipeline) split(in []*flow.Flow) map[Tag][]*flow.Flow {
	out := make(map[Tag][]*flow.Flow)
	for _, fl := range in {
		flowTag := p.Classifier.GetFlowTag(fl)
		out[flowTag] = append(out[flowTag], fl)
	}
	return out
}

func (p *Pipeline) transform(in map[Tag][]*flow.Flow) map[Tag][]interface{} {
	out := make(map[Tag][]interface{})
	for tag := range in {
		for _, f := range in[tag] {
			i := p.Transformer.Transform(f)
			if i != nil {
				out[tag] = append(out[tag], i)
			}
		}
	}
	return out
}

func (p *Pipeline) store(in map[Tag][]interface{}) {
	p.Storer.StoreFlows(in)
}

func (p *Pipeline) process(flows []*flow.Flow) {
	p.Lock()
	defer p.Unlock()

	filtered := p.filter(flows)
	split := p.split(filtered)
	transformed := p.transform(split)
	p.store(transformed)
}

// OnStructMessage is triggered when WS server sends us a message.
func (p *Pipeline) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	switch msg.Type {
	case "store":
		var flows []*flow.Flow
		if err := json.Unmarshal(msg.Obj, &flows); err != nil {
			logging.GetLogger().Error("Failed to unmarshal flows: ", err)
			return
		}

		p.process(flows)
	default:
		logging.GetLogger().Error("Unknown message type: ", msg.Type)
	}
}
