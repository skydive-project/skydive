/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package client

// Operation describes a JSONPatch operation
type Operation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// JSONPatch describes a JSON patch
type JSONPatch []Operation

// NewPatchOperation creates a new JSON patch operation
func NewPatchOperation(op, path string, value ...interface{}) Operation {
	patchOperation := Operation{
		Op:   op,
		Path: path,
	}
	if len(value) > 0 {
		patchOperation.Value = value[0]
	}
	return patchOperation
}
