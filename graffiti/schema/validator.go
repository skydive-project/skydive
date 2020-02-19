/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package schema

import (
	"errors"

	"github.com/xeipuuv/gojsonschema"
)

// ErrInvalidSchema is return when a JSON schema is invalid
var ErrInvalidSchema = errors.New("Invalid schema")

// Validator validates graph nodes and edges using a JSON schema
type Validator struct {
	schemas map[string]gojsonschema.JSONLoader
}

// Validate an object against the JSON schema associated to its kind
func (v *Validator) Validate(kind string, obj interface{}) error {
	schema := v.schemas[kind]
	if schema == nil {
		return nil
	}

	loader := gojsonschema.NewGoLoader(obj)
	result, err := gojsonschema.Validate(schema, loader)
	if err != nil {
		return err
	} else if !result.Valid() {
		return ErrInvalidSchema
	}
	return nil
}

// LoadSchema loads a JSON schema for a kind of object
func (v *Validator) LoadSchema(kind string, schema []byte) {
	v.schemas[kind] = gojsonschema.NewBytesLoader(schema)
}

// NewValidator returns a new JSON schema validator
func NewValidator() *Validator {
	return &Validator{
		schemas: make(map[string]gojsonschema.JSONLoader),
	}
}
