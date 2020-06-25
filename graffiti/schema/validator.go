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
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// ErrInvalidSchema is return when a JSON schema is invalid
type ErrInvalidSchema struct {
	Errors []gojsonschema.ResultError
}

func (e ErrInvalidSchema) Error() string {
	stringErrors := []string{}

	for _, e := range e.Errors {
		stringErrors = append(stringErrors, fmt.Sprint(e))
	}

	return fmt.Sprintf("invalid schema: %v", strings.Join(stringErrors, "; "))
}

// Validator is the interface to implement to validate REST resources
type Validator interface {
	Validate(string, interface{}) error
}

// JSONValidator validates graph nodes and edges using a JSON schema
type JSONValidator struct {
	schemas map[string]gojsonschema.JSONLoader
}

// Validate an object against the JSON schema associated to its kind
func (v *JSONValidator) Validate(kind string, obj interface{}) error {
	schema := v.schemas[kind]
	if schema == nil {
		return nil
	}

	loader := gojsonschema.NewGoLoader(obj)
	result, err := gojsonschema.Validate(schema, loader)
	if err != nil {
		return err
	} else if !result.Valid() {
		return &ErrInvalidSchema{Errors: result.Errors()}
	}
	return nil
}

// LoadSchema loads a JSON schema for a kind of object
func (v *JSONValidator) LoadSchema(kind string, schema []byte) {
	v.schemas[kind] = gojsonschema.NewBytesLoader(schema)
}

// NewJSONValidator returns a new JSON schema validator
func NewJSONValidator() *JSONValidator {
	return &JSONValidator{
		schemas: make(map[string]gojsonschema.JSONLoader),
	}
}
