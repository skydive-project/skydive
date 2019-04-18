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

package subscriber

import (
	"fmt"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/flow"
)

// flowTransformer allows generic transformations of a flow
type flowTransformer interface {
	// Transform transforms a flow before being stored
	Transform(f *flow.Flow) interface{}
}

// newFlowTransformer creates a new flow transformer based on a name string
func newFlowTransformer(flowTransformerName string, gremlinClient *client.GremlinQueryHelper) (flowTransformer, error) {
	switch flowTransformerName {
	case "security-advisor":
		return newSecurityAdvisorFlowTransformer(gremlinClient), nil
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("Flow transformer '%s' is not supported", flowTransformerName)
	}
}
