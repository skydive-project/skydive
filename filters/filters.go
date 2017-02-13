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

package filters

import "regexp"

type Getter interface {
	GetFieldInt64(field string) (int64, error)
	GetFieldString(field string) (string, error)
}

func (f *Filter) Eval(g Getter) bool {
	if f.BoolFilter != nil {
		return f.BoolFilter.Eval(g)
	}
	if f.TermStringFilter != nil {
		return f.TermStringFilter.Eval(g)
	}
	if f.TermInt64Filter != nil {
		return f.TermInt64Filter.Eval(g)
	}
	if f.GtInt64Filter != nil {
		return f.GtInt64Filter.Eval(g)
	}
	if f.LtInt64Filter != nil {
		return f.LtInt64Filter.Eval(g)
	}
	if f.GteInt64Filter != nil {
		return f.GteInt64Filter.Eval(g)
	}
	if f.LteInt64Filter != nil {
		return f.LteInt64Filter.Eval(g)
	}
	if f.RegexFilter != nil {
		return f.RegexFilter.Eval(g)
	}
	if f.NullFilter != nil {
		return f.NullFilter.Eval(g)
	}

	return true
}

func (b *BoolFilter) Eval(g Getter) bool {
	for _, filter := range b.Filters {
		result := filter.Eval(g)
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

func (r *GtInt64Filter) Eval(g Getter) bool {
	field, err := g.GetFieldInt64(r.Key)
	if err != nil {
		return false
	}

	if field > r.Value {
		return true
	}
	return false
}

func (r *LtInt64Filter) Eval(g Getter) bool {
	field, err := g.GetFieldInt64(r.Key)
	if err != nil {
		return false
	}

	if field < r.Value {
		return true
	}
	return false
}

func (r *GteInt64Filter) Eval(g Getter) bool {
	field, err := g.GetFieldInt64(r.Key)
	if err != nil {
		return false
	}

	if field >= r.Value {
		return true
	}
	return false
}

func (r *LteInt64Filter) Eval(g Getter) bool {
	field, err := g.GetFieldInt64(r.Key)
	if err != nil {
		return false
	}

	if field <= r.Value {
		return true
	}
	return false
}

func (t *TermStringFilter) Eval(g Getter) bool {
	field, err := g.GetFieldString(t.Key)
	if err != nil {
		return false
	}

	return field == t.Value
}

func (t *TermInt64Filter) Eval(g Getter) bool {
	field, err := g.GetFieldInt64(t.Key)
	if err != nil {
		return false
	}

	return field == t.Value
}

func (r *RegexFilter) Eval(g Getter) bool {
	field, err := g.GetFieldString(r.Key)
	if err != nil {
		return false
	}
	// TODO: don't compile regex here
	re := regexp.MustCompile(r.Value)
	return re.MatchString(field)
}

func (r *NullFilter) Eval(g Getter) bool {
	if _, err := g.GetFieldString(r.Key); err == nil {
		return false
	}
	if _, err := g.GetFieldInt64(r.Key); err == nil {
		return false
	}
	return true
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

func NewNotFilter(filter *Filter) *Filter {
	return NewBoolFilter(BoolFilterOp_NOT, filter)
}

func NewGtInt64Filter(key string, value int64) *Filter {
	return &Filter{GtInt64Filter: &GtInt64Filter{Key: key, Value: value}}
}

func NewGteInt64Filter(key string, value int64) *Filter {
	return &Filter{GteInt64Filter: &GteInt64Filter{Key: key, Value: value}}
}

func NewLtInt64Filter(key string, value int64) *Filter {
	return &Filter{LtInt64Filter: &LtInt64Filter{Key: key, Value: value}}
}

func NewLteInt64Filter(key string, value int64) *Filter {
	return &Filter{LteInt64Filter: &LteInt64Filter{Key: key, Value: value}}
}

func NewTermInt64Filter(key string, value int64) *Filter {
	return &Filter{TermInt64Filter: &TermInt64Filter{Key: key, Value: value}}
}

func NewTermStringFilter(key string, value string) *Filter {
	return &Filter{TermStringFilter: &TermStringFilter{Key: key, Value: value}}
}

func NewNullFilter(key string) *Filter {
	return &Filter{NullFilter: &NullFilter{Key: key}}
}

func NewFilterForIds(uuids []string, attrs ...string) *Filter {
	terms := make([]*Filter, len(uuids)*len(attrs))
	for i, uuid := range uuids {
		for j, attr := range attrs {
			terms[i*len(attrs)+j] = NewTermStringFilter(attr, uuid)
		}
	}
	return NewOrFilter(terms...)
}

// NewFilterActiveIn returns a filter that returns elements that were active
// in the given time range.
func NewFilterActiveIn(fr Range, prefix string) *Filter {
	return NewAndFilter(
		NewLteInt64Filter(prefix+"Start", fr.To),
		NewGteInt64Filter(prefix+"Last", fr.From),
	)
}

// NewFilterIncludedIn returns a filter that returns elements that include in
// the time range.
func NewFilterIncludedIn(fr Range, prefix string) *Filter {
	return NewAndFilter(
		NewGteInt64Filter(prefix+"Start", fr.From),
		NewLteInt64Filter(prefix+"Last", fr.To),
	)
}
