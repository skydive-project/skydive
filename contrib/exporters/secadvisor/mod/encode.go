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

package mod

import (
	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/contrib/exporters/core"
)

type encode struct {
	*core.EncodeJSON
}

type topLevelObject struct {
	Data []interface{} `json:"data"`
}

// Encode the incoming flows array as a JSON object with key "data" whose value
// holds the array
func (e *encode) Encode(in interface{}) ([]byte, error) {
	return e.EncodeJSON.Encode(topLevelObject{Data: in.([]interface{})})
}

// NewEncode creates an encode object for Secadvisor format
func NewEncode(cfg *viper.Viper) (interface{}, error) {
	jsonEncoder, err := core.NewEncodeJSON(cfg)
	if err != nil {
		return nil, err
	}
	return &encode{jsonEncoder.(*core.EncodeJSON)}, nil
}
