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
	"fmt"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

type storeStdout struct {
	pipeline *Pipeline
}

// SetPipeline setup
func (s *storeStdout) SetPipeline(pipeline *Pipeline) {
	s.pipeline = pipeline
}

// StoreFlows store flows in memory, before being written to the object store
func (s *storeStdout) StoreFlows(flows []*flow.Flow) error {
	transformed := s.pipeline.Transformer.Transform(flows)

	encoded, err := s.pipeline.Encoder.Encode(transformed)
	if err != nil {
		logging.GetLogger().Error("Failed to encode object: ", err)
		return err
	}

	compressed, err := s.pipeline.Compressor.Compress(encoded)
	if err != nil {
		logging.GetLogger().Error("Failed to compress object: ", err)
		return err
	}

	fmt.Printf("%s\n", compressed)

	return nil
}

// NewStoreStdout returns a new storage interface for storing flows to object store
func NewStoreStdout() (Storer, error) {
	return &storeStdout{}, nil
}
