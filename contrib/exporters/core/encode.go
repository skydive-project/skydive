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

	"github.com/gocarina/gocsv"
	"github.com/spf13/viper"
)

// EncodeJSON encoder encodes flows as a JSON array
type EncodeJSON struct {
	pretty bool
}

// Encode implements Encoder interface
func (e *EncodeJSON) Encode(in interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)

	if e.pretty {
		encoder.SetIndent("", "\t")
	}

	err := encoder.Encode(in)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

// NewEncodeJSON creates an encode object
func NewEncodeJSON(cfg *viper.Viper) (interface{}, error) {
	return &EncodeJSON{
		pretty: cfg.GetBool(CfgRoot + "encode.json.pretty"),
	}, nil
}

// EncodeCSV encoder encodes flows as CSV rows
type EncodeCSV struct {
}

// Encode implements Encoder interface
func (e *EncodeCSV) Encode(in interface{}) ([]byte, error) {
	return gocsv.MarshalBytes(in)
}

// NewEncodeCSV creates an encode object
func NewEncodeCSV(cfg *viper.Viper) (interface{}, error) {
	return &EncodeCSV{}, nil
}
