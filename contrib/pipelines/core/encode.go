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
)

// Encoder exposes the interface for encoding flows
type Encoder interface {
	Encode(in []interface{}) ([]byte, error)
}

type encodeJSON struct {
}

// Encode explements Encounter interface
func (e *encodeJSON) Encode(in []interface{}) ([]byte, error) {
	return json.Marshal(in)
}

// NewEncodeJSON create an encode object
func NewEncodeJSON() (Encoder, error) {
	return &encodeJSON{}, nil
}
