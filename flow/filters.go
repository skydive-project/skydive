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
	"fmt"
	"strings"
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

	return true
}

func (f *Filter) Expression() string {
	if f.BoolFilter != nil {
		return f.BoolFilter.Expression()
	}
	if f.TermStringFilter != nil {
		return f.TermStringFilter.Expression()
	}
	if f.TermInt64Filter != nil {
		return f.TermInt64Filter.Expression()
	}
	if f.GtInt64Filter != nil {
		return f.GtInt64Filter.Expression()
	}
	if f.LtInt64Filter != nil {
		return f.LtInt64Filter.Expression()
	}
	if f.GteInt64Filter != nil {
		return f.GteInt64Filter.Expression()
	}
	if f.LteInt64Filter != nil {
		return f.LteInt64Filter.Expression()
	}

	return ""
}

func (b *BoolFilter) Expression() string {
	keyword := ""
	switch b.Op {
	case BoolFilterOp_NOT:
		// FIX not yet implemented for the orientdb backend
		// http://orientdb.com/docs/2.0/orientdb.wiki/SQL-Where.html
		return "NOT " + b.Filters[0].Expression()
	case BoolFilterOp_OR:
		keyword = "OR"
	case BoolFilterOp_AND:
		keyword = "AND"
	}
	var conditions []string
	for _, item := range b.Filters {
		conditions = append(conditions, "("+item.Expression()+")")
	}
	return strings.Join(conditions, " "+keyword+" ")
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

func (r *GtInt64Filter) Expression() string {
	return fmt.Sprintf("%v > %v", r.Key, r.Value)
}

func (r *LtInt64Filter) Expression() string {
	return fmt.Sprintf("%v < %v", r.Key, r.Value)
}

func (r *GteInt64Filter) Expression() string {
	return fmt.Sprintf("%v >= %v", r.Key, r.Value)
}

func (r *LteInt64Filter) Expression() string {
	return fmt.Sprintf("%v <= %v", r.Key, r.Value)
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

func (t *TermStringFilter) Expression() string {
	return fmt.Sprintf(`%s = "%s"`, t.Key, t.Value)
}

func (t *TermStringFilter) Eval(f *Flow) bool {
	field, err := f.GetFieldString(t.Key)
	if err != nil {
		return false
	}

	return field == t.Value
}

func (t *TermInt64Filter) Expression() string {
	return fmt.Sprintf(`%s = %d`, t.Key, t.Value)
}

func (t *TermInt64Filter) Eval(f *Flow) bool {
	field, err := f.GetFieldInt64(t.Key)
	if err != nil {
		return false
	}

	return field == t.Value
}

func NewFilterForNodes(uuids []string) *Filter {
	terms := make([]*Filter, len(uuids)*3)
	for i, uuid := range uuids {
		terms[i*3] = &Filter{
			TermStringFilter: &TermStringFilter{Key: "NodeUUID", Value: uuid},
		}
		terms[i*3+1] = &Filter{
			TermStringFilter: &TermStringFilter{Key: "ANodeUUID", Value: uuid},
		}
		terms[i*3+2] = &Filter{
			TermStringFilter: &TermStringFilter{Key: "BNodeUUID", Value: uuid},
		}
	}

	return &Filter{
		BoolFilter: &BoolFilter{
			Op:      BoolFilterOp_OR,
			Filters: terms,
		},
	}
}
