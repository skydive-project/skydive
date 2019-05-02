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

package graph

import (
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
)

// ElementMatcher defines an interface used to match an element
type ElementMatcher interface {
	Match(g common.Getter) bool
	Filter() (*filters.Filter, error)
}

// ElementFilter implements ElementMatcher interface based on filter
type ElementFilter struct {
	filter *filters.Filter
}

// Match returns true if the given element matches the filter.
func (mf *ElementFilter) Match(g common.Getter) bool {
	return mf.filter.Eval(g)
}

// Filter returns the filter
func (mf *ElementFilter) Filter() (*filters.Filter, error) {
	return mf.filter, nil
}

// NewElementFilter returns a new ElementFilter
func NewElementFilter(f *filters.Filter) *ElementFilter {
	return &ElementFilter{filter: f}
}

// NewFilterForEdge creates a filter based on parent or child
func NewFilterForEdge(parent Identifier, child Identifier) *filters.Filter {
	return filters.NewOrFilter(
		filters.NewTermStringFilter("Parent", string(parent)),
		filters.NewTermStringFilter("Child", string(child)),
	)
}

// filterForTimeSlice creates a filter based on a time slice between
// startName and endName. time.Now() is used as reference if t == nil
func filterForTimeSlice(t *common.TimeSlice, startName, endName string) *filters.Filter {
	if t == nil {
		u := common.UnixMillis(time.Now())
		t = common.NewTimeSlice(u, u)
	}

	return filters.NewAndFilter(
		filters.NewLteInt64Filter(startName, t.Last),
		filters.NewOrFilter(
			filters.NewNullFilter(endName),
			filters.NewGteInt64Filter(endName, t.Start),
		),
	)
}

func getTimeFilter(t *common.TimeSlice) *filters.Filter {
	if t == nil {
		return filters.NewNullFilter("ArchivedAt")
	}

	return filters.NewAndFilter(
		filterForTimeSlice(t, "CreatedAt", "DeletedAt"),
		filterForTimeSlice(t, "UpdatedAt", "ArchivedAt"),
	)
}
