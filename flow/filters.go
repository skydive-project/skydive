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

package flow

import (
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// NewFilterForNodeTIDs creates a new filter based on flow NodeTID
func NewFilterForNodeTIDs(uuids []string) *filters.Filter {
	return filters.NewOrTermStringFilter(uuids, "NodeTID")
}

// NewFilterForNodes creates a new filter based on graph nodes
func NewFilterForNodes(nodes []*graph.Node) *filters.Filter {
	var tids []string

	seen := make(map[string]bool)
	for _, node := range nodes {
		if tid, _ := node.GetFieldString("TID"); tid != "" {
			if _, ok := seen[tid]; !ok {
				tids = append(tids, tid)
				seen[tid] = true
			}
		}
	}
	return NewFilterForNodeTIDs(tids)
}

// NewFilterForFlowSet creates a new filter based on a set of flows
func NewFilterForFlowSet(flowset *FlowSet) *filters.Filter {
	ids := make([]string, len(flowset.Flows))
	for i, flow := range flowset.Flows {
		ids[i] = string(flow.UUID)
	}
	return filters.NewOrTermStringFilter(ids, "UUID")
}
