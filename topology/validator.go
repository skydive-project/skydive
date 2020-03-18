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

package topology

import (
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/schema"
	"github.com/skydive-project/skydive/statics"
)

// SchemaValidator is the global validator for Skydive resources
var SchemaValidator *schema.Validator

func init() {
	nodeSchema, err := statics.Asset("statics/schemas/node.schema")
	if err != nil {
		panic(err)
	}

	edgeSchema, err := statics.Asset("statics/schemas/edge.schema")
	if err != nil {
		panic(err)
	}

	SchemaValidator = schema.NewValidator()
	SchemaValidator.LoadSchema("node", nodeSchema)
	SchemaValidator.LoadSchema("edge", edgeSchema)

	types.SchemaValidator = SchemaValidator
}
