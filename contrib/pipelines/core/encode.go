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

	"github.com/gocarina/gocsv"
	"github.com/spf13/viper"
)

// Encoder exposes the interface for encoding flows
type Encoder interface {
	Encode(in interface{}) ([]byte, error)
}

type encodeJSON struct {
	pretty bool
}

// Encode explements Encounter interface
func (e *encodeJSON) Encode(in interface{}) ([]byte, error) {
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

// NewEncodeJSON create an encode object
func NewEncodeJSON(cfg *viper.Viper) (Encoder, error) {
	return &encodeJSON{
		pretty: cfg.GetBool(CfgRoot + "encode.json.pretty"),
	}, nil
}

type encodeCSV struct {
}

// Encode explements Encounter interface
func (e *encodeCSV) Encode(in interface{}) ([]byte, error) {
	return gocsv.MarshalBytes(in)
}

// NewEncodeCSV create an encode object
func NewEncodeCSV() (Encoder, error) {
	return &encodeCSV{}, nil
}

// NewEncodeFromConfig creates store from config
func NewEncodeFromConfig(cfg *viper.Viper) (Encoder, error) {
	ty := cfg.GetString(CfgRoot + "encode.type")
	switch ty {
	case "csv":
		return NewEncodeCSV()
	case "json":
		return NewEncodeJSON(cfg)
	default:
		return nil, fmt.Errorf("Encode type %s not supported", ty)
	}
}
