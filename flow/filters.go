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
	"regexp"

	"github.com/skydive-project/skydive/topology/graph"
)

func (f *Filter) Eval(flow *Flow) bool {
	if f.BoolFilter != nil {
		return f.BoolFilter.Eval(flow)
	}
	if f.TermStringFilter != nil {
		return f.TermStringFilter.Eval(flow)
	}
	if f.TermInt64Filter != nil {
		return f.TermInt64Filter.Eval(flow)
	}
	if f.GtInt64Filter != nil {
		return f.GtInt64Filter.Eval(flow)
	}
	if f.LtInt64Filter != nil {
		return f.LtInt64Filter.Eval(flow)
	}
	if f.GteInt64Filter != nil {
		return f.GteInt64Filter.Eval(flow)
	}
	if f.LteInt64Filter != nil {
		return f.LteInt64Filter.Eval(flow)
	}
	if f.RegexFilter != nil {
		return f.RegexFilter.Eval(flow)
	}

	return true
}

func (b *BoolFilter) Eval(flow *Flow) bool {
	for _, filter := range b.Filters {
		result := filter.Eval(flow)
		if b.Op == BoolFilterOp_NOT && !result {
			return true
		}
		if b.Op == BoolFilterOp_AND && !result {
			return false
		} else if b.Op == BoolFilterOp_OR && result {
			return true
		}
	}
	return b.Op == BoolFilterOp_AND || len(b.Filters) == 0
}

func (r *GtInt64Filter) Eval(f *Flow) bool {
	field, err := f.GetFieldInt64(r.Key)
	if err != nil {
		return false
	}

	if field > r.Value {
		return true
	}
	return false
}

func (r *LtInt64Filter) Eval(f *Flow) bool {
	field, err := f.GetFieldInt64(r.Key)
	if err != nil {
		return false
	}

	if field < r.Value {
		return true
	}
	return false
}

func (r *GteInt64Filter) Eval(f *Flow) bool {
	field, err := f.GetFieldInt64(r.Key)
	if err != nil {
		return false
	}

	if field >= r.Value {
		return true
	}
	return false
}

func (r *LteInt64Filter) Eval(f *Flow) bool {
	field, err := f.GetFieldInt64(r.Key)
	if err != nil {
		return false
	}

	if field <= r.Value {
		return true
	}
	return false
}

func (t *TermStringFilter) Eval(f *Flow) bool {
	field, err := f.GetFieldString(t.Key)
	if err != nil {
		return false
	}

	return field == t.Value
}

func (t *TermInt64Filter) Eval(f *Flow) bool {
	field, err := f.GetFieldInt64(t.Key)
	if err != nil {
		return false
	}

	return field == t.Value
}

func (r *RegexFilter) Eval(f *Flow) bool {
	field, err := f.GetFieldString(r.Key)
	if err != nil {
		return false
	}
	// TODO: don't compile regex here
	re := regexp.MustCompile(r.Value)
	return re.MatchString(field)
}

func NewBoolFilter(op BoolFilterOp, filters ...*Filter) *Filter {
	boolFilter := &BoolFilter{
		Op:      op,
		Filters: []*Filter{},
	}

	for _, filter := range filters {
		if filter != nil {
			boolFilter.Filters = append(boolFilter.Filters, filter)
		}
	}

	return &Filter{BoolFilter: boolFilter}
}

func NewAndFilter(filters ...*Filter) *Filter {
	return NewBoolFilter(BoolFilterOp_AND, filters...)
}

func NewOrFilter(filters ...*Filter) *Filter {
	return NewBoolFilter(BoolFilterOp_OR, filters...)
}

func NewFilterForIds(uuids []string, attrs ...string) *Filter {
	terms := make([]*Filter, len(uuids)*len(attrs))
	for i, uuid := range uuids {
		for j, attr := range attrs {
			terms[i*len(attrs)+j] = &Filter{
				TermStringFilter: &TermStringFilter{Key: attr, Value: uuid},
			}
		}
	}

	return &Filter{
		BoolFilter: &BoolFilter{
			Op:      BoolFilterOp_OR,
			Filters: terms,
		},
	}
}

func NewFilterForNodeTIDs(uuids []string) *Filter {
	return NewFilterForIds(uuids, "NodeTID", "ANodeTID", "BNodeTID")
}

func NewFilterForNodes(nodes []*graph.Node) *Filter {
	var ids []string
	for _, node := range nodes {
		if t, ok := node.Metadata()["TID"]; ok {
			ids = append(ids, t.(string))
		}
	}
	return NewFilterForNodeTIDs(ids)
}

func NewFilterForFlowSet(flowset *FlowSet) *Filter {
	ids := make([]string, len(flowset.Flows))
	for i, flow := range flowset.Flows {
		ids[i] = string(flow.UUID)
	}
	return NewFilterForIds(ids, "UUID")
}

// NewFilterActiveIn returns a filter that returns elements that were active
// in the given time range.
func NewFilterActiveIn(fr Range, prefix string) *Filter {
	andFilter := &BoolFilter{
		Op: BoolFilterOp_AND,
		Filters: []*Filter{
			{
				LtInt64Filter: &LtInt64Filter{
					Key:   prefix + "Start",
					Value: fr.To,
				},
			},
			{
				GteInt64Filter: &GteInt64Filter{
					Key:   prefix + "Last",
					Value: fr.From,
				},
			},
		},
	}
	return &Filter{BoolFilter: andFilter}
}

// NewFilterIncludedIn returns a filter that returns elements that include in
// the time range.
func NewFilterIncludedIn(fr Range, prefix string) *Filter {
	andFilter := &BoolFilter{
		Op: BoolFilterOp_AND,
		Filters: []*Filter{
			{
				GteInt64Filter: &GteInt64Filter{
					Key:   prefix + "Start",
					Value: fr.From,
				},
			},
			{
				LteInt64Filter: &LteInt64Filter{
					Key:   prefix + "Last",
					Value: fr.To,
				},
			},
		},
	}
	return &Filter{BoolFilter: andFilter}
}

// NewFilterActiveAt returns a filter including all the elements that are
// active/alive at that time, ex: flow started and not expired at that time.
func NewFilterActiveAt(t int64, prefix string) *Filter {
	andFilter := &BoolFilter{
		Op: BoolFilterOp_AND,
		Filters: []*Filter{
			{
				LteInt64Filter: &LteInt64Filter{
					Key:   prefix + "Start",
					Value: t,
				},
			},
			{
				GteInt64Filter: &GteInt64Filter{
					Key:   prefix + "Last",
					Value: t,
				},
			},
		},
	}
	return &Filter{BoolFilter: andFilter}
}
