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
	"compress/gzip"

	"github.com/spf13/viper"
)

type compressNone struct {
}

// Compress inplements Compressor interface
func (e *compressNone) Compress(in []byte) (*bytes.Buffer, error) {
	return bytes.NewBuffer(in), nil
}

// NewCompressNone create an encode object
func NewCompressNone(cfg *viper.Viper) (interface{}, error) {
	return &compressNone{}, nil
}

type compressGzip struct {
}

// Compress inplements Compressor interface
func (e *compressGzip) Compress(in []byte) (*bytes.Buffer, error) {
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	if _, err := w.Write(in); err != nil {
		return nil, err
	}
	w.Close()
	return &out, nil
}

// NewCompressGzip create an encode object
func NewCompressGzip(cfg *viper.Viper) (interface{}, error) {
	return &compressGzip{}, nil
}
