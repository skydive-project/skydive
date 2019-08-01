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

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/flow"
)

// Storer interface of a store object
type Storer interface {
	StoreFlows(flows []*flow.Flow) error
	SetPipeline(p *Pipeline)
}

// NewStoreFromConfig creates store from config
func NewStoreFromConfig(cfg *viper.Viper) (Storer, error) {
	storeType := cfg.GetString(CfgRoot + "store.type")
	switch storeType {
	case "stdout":
		return NewStoreStdout()
	case "s3":
		return NewStoreS3FromConfig(cfg)
	default:
		return nil, fmt.Errorf("Store type %s not supported", storeType)
	}
}
