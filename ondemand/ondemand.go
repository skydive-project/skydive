/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package ondemand

import (
	json "encoding/json"

	"github.com/skydive-project/skydive/graffiti/graph"
)

// Namespace "OnDemand"
const Namespace = "OnDemand"

// Query describes an ondemand query
// easyjson:json
type Query struct {
	NodeID   graph.Identifier
	Resource interface{}
}

// RawQuery describes a raw ondemand query
// easyjson:json
type RawQuery struct {
	NodeID   graph.Identifier
	Resource json.RawMessage
}

// Task describes the processing unit created by an ondemand
type Task = interface{}
