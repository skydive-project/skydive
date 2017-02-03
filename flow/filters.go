/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package flow

import (
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/topology/graph"
)

func NewFilterForNodeTIDs(uuids []string) *filters.Filter {
	return filters.NewFilterForIds(uuids, "NodeTID", "ANodeTID", "BNodeTID")
}

func NewFilterForNodes(nodes []*graph.Node) *filters.Filter {
	var ids []string
	for _, node := range nodes {
		if t, ok := node.Metadata()["TID"]; ok {
			ids = append(ids, t.(string))
		}
	}
	return NewFilterForNodeTIDs(ids)
}

func NewFilterForFlowSet(flowset *FlowSet) *filters.Filter {
	ids := make([]string, len(flowset.Flows))
	for i, flow := range flowset.Flows {
		ids[i] = string(flow.UUID)
	}
	return filters.NewFilterForIds(ids, "UUID")
}
