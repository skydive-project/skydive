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
)

// Compressor exposes the interface for compressesing encoded flows
type Compressor interface {
	Compress(b []byte) (bytes.Buffer, error)
}

type compressGzip struct {
}

// Compress inplements Compressor interface
func (e *compressGzip) Compress(in []byte) (bytes.Buffer, error) {
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	w.Write(in)
	w.Close()
	return out, nil
}

// NewCompressGzip create an encode object
func NewCompressGzip() (Compressor, error) {
	return &compressGzip{}, nil
}
