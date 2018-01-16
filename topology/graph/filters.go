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

package graph

import (
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
)

// NewFilterForEdge creates a filter based on parent or child
func NewFilterForEdge(parent Identifier, child Identifier) *filters.Filter {
	return filters.NewOrFilter(
		filters.NewTermStringFilter("Parent", string(parent)),
		filters.NewTermStringFilter("Child", string(child)),
	)
}

// NewFilterForTimeSlice creates a filter based on a time slice between CreatedAt and DeletedAt
// time.Now() is used as reference if t == nil
func NewFilterForTimeSlice(t *common.TimeSlice) *filters.Filter {
	if t == nil {
		return NewFilterForTime(time.Now())
	}

	return filters.NewAndFilter(
		filters.NewLteInt64Filter("CreatedAt", t.Last),
		filters.NewOrFilter(
			filters.NewNullFilter("DeletedAt"),
			filters.NewGteInt64Filter("DeletedAt", t.Start),
		),
	)
}

// NewFilterForTime creates a filter including time slice t
func NewFilterForTime(t time.Time) *filters.Filter {
	u := common.UnixMillis(t)
	return NewFilterForTimeSlice(common.NewTimeSlice(u, u))
}
