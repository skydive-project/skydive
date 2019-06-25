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
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

type storeLog struct {
}

// SetPipeline setup
func (s *storeLog) SetPipeline(pipeline *Pipeline) {
}

// StoreFlows store flows in memory, before being written to the object store
func (s *storeLog) StoreFlows(flows []*flow.Flow) error {
	for _, fl := range flows {
		logging.GetLogger().Infof("store flow: %v", fl)
	}
	return nil
}

// NewStoreLog returns a new storage interface for storing flows to object store
func NewStoreLog() (Storer, error) {
	return &storeLog{}, nil
}
