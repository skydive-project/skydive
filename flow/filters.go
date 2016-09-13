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

	"github.com/skydive-project/skydive/common"
)

func (f *Filter) Eval(value interface{}) bool {
	if f.BoolFilter != nil {
		return f.BoolFilter.Eval(value)
	}
	if f.TermStringFilter != nil {
		return f.TermStringFilter.Eval(value)
	}
	if f.TermInt64Filter != nil {
		return f.TermInt64Filter.Eval(value)
	}
	if f.GtInt64Filter != nil {
		return f.GtInt64Filter.Eval(value)
	}
	if f.LtInt64Filter != nil {
		return f.LtInt64Filter.Eval(value)
	}
	if f.GteInt64Filter != nil {
		return f.GteInt64Filter.Eval(value)
	}
	if f.LteInt64Filter != nil {
		return f.LteInt64Filter.Eval(value)
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

func (b *BoolFilter) Eval(value interface{}) bool {
	for _, filter := range b.Filters {
		result := filter.Eval(value)
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

func (r *GtInt64Filter) Eval(value interface{}) bool {
	field := GetFields(value, strings.Split(r.Key, "."))
	if field == nil {
		return false
	}

	if result, err := common.CrossTypeCompare(field, r.Value); err != nil || result != 1 {
		return false
	}

	return true
}

func (r *LtInt64Filter) Eval(value interface{}) bool {
	field := GetFields(value, strings.Split(r.Key, "."))
	if field == nil {
		return false
	}

	if result, err := common.CrossTypeCompare(field, r.Value); err != nil || result != -1 {
		return false
	}

	return true
}

func (r *GteInt64Filter) Eval(value interface{}) bool {
	field := GetFields(value, strings.Split(r.Key, "."))
	if field == nil {
		return false
	}

	if result, err := common.CrossTypeCompare(field, r.Value); err != nil || result == -1 {
		return false
	}

	return true
}

func (r *LteInt64Filter) Eval(value interface{}) bool {
	field := GetFields(value, strings.Split(r.Key, "."))
	if field == nil {
		return false
	}

	if result, err := common.CrossTypeCompare(field, r.Value); err != nil || result == 1 {
		return false
	}

	return true
}

func (t *TermStringFilter) Expression() string {
	return fmt.Sprintf(`%s = "%s"`, t.Key, t.Value)
}

func (t *TermStringFilter) Eval(value interface{}) bool {
	field := GetFields(value, strings.Split(t.Key, "."))
	if field == nil {
		return false
	}

	return common.CrossTypeEqual(field, t.Value)
}

func (t *TermInt64Filter) Expression() string {
	return fmt.Sprintf(`%s = %d`, t.Key, t.Value)
}

func (t *TermInt64Filter) Eval(value interface{}) bool {
	field := GetFields(value, strings.Split(t.Key, "."))
	if field == nil {
		return false
	}

	return common.CrossTypeEqual(field, t.Value)
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
