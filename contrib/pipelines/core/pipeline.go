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
	"encoding/json"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	ws "github.com/skydive-project/skydive/websocket"
)

// CfgRoot configuration root path
const CfgRoot = "pipeline."

// Pipeline manager
type Pipeline struct {
	Transformer Transformer
	Classifier  Classifier
	Filterer    Filterer
	Encoder     Encoder
	Compressor  Compressor
	Storer      Storer
}

// NewPipeline defines the pipeline elements
func NewPipeline(transformer Transformer, classifier Classifier, filterer Filterer, encoder Encoder, compressor Compressor, storer Storer) *Pipeline {
	p := &Pipeline{
		Transformer: transformer,
		Classifier:  classifier,
		Filterer:    filterer,
		Encoder:     encoder,
		Compressor:  compressor,
		Storer:      storer,
	}
	storer.SetPipeline(p)
	return p
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

		p.Storer.StoreFlows(flows)
	default:
		logging.GetLogger().Error("Unknown message type: ", msg.Type)
	}
}
