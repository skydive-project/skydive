/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package storage

import (
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/topology/graph"
)

type Storage interface {
	Start()
	StoreFlows(flows []*flow.Flow) error
	SearchFlows(filter flow.Filter) ([]*flow.Flow, error)
	Stop()
}

func LookupFlowsByNodes(s Storage, context graph.GraphContext, hnmap flow.HostNodeIDMap, filter flow.Filter) ([]*flow.Flow, error) {
	andFilter := flow.BoolFilter{
		Op: flow.AND,
		Filters: []flow.Filter{
			flow.RangeFilter{
				Key: "Statistics.Start",
				Lte: context.Time.Unix(),
			},
			flow.RangeFilter{
				Key: "Statistics.Last",
				Gte: context.Time.Unix(),
			},
		},
	}

	if len(hnmap) > 0 {
		nodeFilter := flow.BoolFilter{Op: flow.OR}
		for _, ids := range hnmap {
			for _, id := range ids {
				nodeFilter.Filters = append(nodeFilter.Filters,
					flow.TermFilter{Key: "ProbeNodeUUID", Value: id},
					flow.TermFilter{Key: "IfSrcNodeUUID", Value: id},
					flow.TermFilter{Key: "IfDstNodeUUID", Value: id})
			}
		}
		andFilter.Filters = append(andFilter.Filters, nodeFilter)
	}

	if filter != nil {
		andFilter.Filters = append(andFilter.Filters, filter)
	}

	return s.SearchFlows(andFilter)
}
