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
	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/logging"
)

type storeDirect struct {
	pipeline *Pipeline
}

// SetPipeline setup
func (s *storeDirect) SetPipeline(pipeline *Pipeline) {
	s.pipeline = pipeline
}

// StoreFlows store flows in memory, before being written to the object store
func (s *storeDirect) StoreFlows(flows map[Tag][]interface{}) error {
	for t, val := range flows {
		encoded, err := s.pipeline.Encoder.Encode(val)
		if err != nil {
			logging.GetLogger().Error("Failed to encode object: ", err)
			return err
		}

		compressed, err := s.pipeline.Compressor.Compress(encoded)
		if err != nil {
			logging.GetLogger().Error("Failed to compress object: ", err)
			return err
		}

		err = s.pipeline.Writer.Write("my_dirname", string(t), compressed.String(),
			"application/json", "gzip", map[string]*string{})
		if err != nil {
			logging.GetLogger().Error("Failed to store object: ", err)
			return err
		}
	}

	return nil
}

// NewStoreDirect returns a new storage interface for storing flows to object store
func NewStoreDirect(cfg *viper.Viper) (interface{}, error) {
	return &storeDirect{}, nil
}
