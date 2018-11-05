/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package topology

import (
	"errors"

	"github.com/skydive-project/skydive/statics"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/xeipuuv/gojsonschema"
)

// ErrInvalidSchema is return when a JSON schema is invalid
var ErrInvalidSchema = errors.New("Invalid schema")

// SchemaValidator validates graph nodes and edges using a JSON schema
type SchemaValidator struct {
	nodeSchema gojsonschema.JSONLoader
	edgeSchema gojsonschema.JSONLoader
}

func (v *SchemaValidator) validate(obj interface{}, schema gojsonschema.JSONLoader) error {
	loader := gojsonschema.NewGoLoader(obj)
	result, err := gojsonschema.Validate(schema, loader)
	if err != nil {
		return err
	} else if !result.Valid() {
		return ErrInvalidSchema
	}
	return nil
}

// ValidateNode valides a graph node
func (v *SchemaValidator) ValidateNode(node *graph.Node) error {
	return v.validate(node, v.nodeSchema)
}

// ValidateEdge valides a graph edge
func (v *SchemaValidator) ValidateEdge(edge *graph.Edge) error {
	return v.validate(edge, v.edgeSchema)
}

// NewSchemaValidator returns a new JSON schema validator for
// graph nodes and edges. based on JSON schema bundled with go-bindata
func NewSchemaValidator() (*SchemaValidator, error) {
	nodeSchema, err := statics.Asset("statics/schemas/node.schema")
	if err != nil {
		return nil, err
	}

	edgeSchema, err := statics.Asset("statics/schemas/edge.schema")
	if err != nil {
		return nil, err
	}

	return &SchemaValidator{
		nodeSchema: gojsonschema.NewBytesLoader(nodeSchema),
		edgeSchema: gojsonschema.NewBytesLoader(edgeSchema),
	}, nil
}
