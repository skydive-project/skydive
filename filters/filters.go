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

package filters

import (
	"regexp"
	"time"

	"github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/common"
)

var regexpCache *cache.Cache

func evalStringField(g common.Getter, key string, predicate func(string) bool) bool {
	if s, err := common.GetFieldString(g, key); err == nil {
		return predicate(s)
	}

	if strings, err := common.GetFieldStringList(g, key); err == nil {
		for _, s := range strings {
			if predicate(s) {
				return true
			}
		}
	}

	return false
}

func evalInt64Field(g common.Getter, key string, predicate func(int64) bool) bool {
	if s, err := common.GetFieldInt64(g, key); err == nil {
		return predicate(s)
	}

	if integers, err := common.GetFieldInt64List(g, key); err == nil {
		for _, i := range integers {
			if predicate(i) {
				return true
			}
		}
	}

	return false
}

// Eval evaluates a filter
func (f *Filter) Eval(g common.Getter) bool {
	if f.BoolFilter != nil {
		return f.BoolFilter.Eval(g)
	}
	if f.TermStringFilter != nil {
		return f.TermStringFilter.Eval(g)
	}
	if f.TermInt64Filter != nil {
		return f.TermInt64Filter.Eval(g)
	}
	if f.TermBoolFilter != nil {
		return f.TermBoolFilter.Eval(g)
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
	if f.IPV4RangeFilter != nil {
		return f.IPV4RangeFilter.Eval(g)
	}

	return true
}

// Eval evaluates a boolean (not, and, or) filter
func (b *BoolFilter) Eval(g common.Getter) bool {
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

// Eval evaluates an int64 > filter
func (r *GtInt64Filter) Eval(g common.Getter) bool {
	return evalInt64Field(g, r.Key, func(i int64) bool {
		return i > r.Value
	})
}

// Eval evaluates an int64 < filter
func (r *LtInt64Filter) Eval(g common.Getter) bool {
	return evalInt64Field(g, r.Key, func(i int64) bool {
		return i < r.Value
	})
}

// Eval evaluates an int64 >= filter
func (r *GteInt64Filter) Eval(g common.Getter) bool {
	return evalInt64Field(g, r.Key, func(i int64) bool {
		return i >= r.Value
	})
}

// Eval evaluates an int64 <= filter
func (r *LteInt64Filter) Eval(g common.Getter) bool {
	return evalInt64Field(g, r.Key, func(i int64) bool {
		return i <= r.Value
	})
}

// Eval evaluates an string type filter
func (t *TermStringFilter) Eval(g common.Getter) bool {
	return evalStringField(g, t.Key, func(s string) bool {
		return s == t.Value
	})
}

// Eval evaluates an int64 type filter
func (t *TermInt64Filter) Eval(g common.Getter) bool {
	return evalInt64Field(g, t.Key, func(i int64) bool {
		return i == t.Value
	})
}

// Eval evaluates a bool type filter
func (t *TermBoolFilter) Eval(g common.Getter) bool {
	field, err := common.GetField(g, t.Key)
	if err != nil {
		return false
	}
	switch field := field.(type) {
	case []interface{}:
		for _, intf := range field {
			if b, ok := intf.(bool); ok && b == t.Value {
				return true
			}
		}
	case []bool:
		for _, v := range field {
			if v == t.Value {
				return true
			}
		}
	case bool:
		if field == t.Value {
			return true
		}
	}
	return false
}

// Eval evaluates an regex filter
func (r *RegexFilter) Eval(g common.Getter) bool {
	re, found := regexpCache.Get(r.Value)
	if !found {
		re = regexp.MustCompile(r.Value)
		regexpCache.Set(r.Value, re, cache.DefaultExpiration)
	}

	return evalStringField(g, r.Key, func(s string) bool {
		return re.(*regexp.Regexp).MatchString(s)
	})
}

// NewRegexFilter returns a new regular expression based filter
func NewRegexFilter(key string, pattern string) (*RegexFilter, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexpCache.Set(pattern, re, cache.DefaultExpiration)

	return &RegexFilter{Key: key, Value: pattern}, nil
}

// Eval evaluates an null filter (not string and not int64 types)
func (n *NullFilter) Eval(g common.Getter) bool {
	if _, err := common.GetField(g, n.Key); err == nil {
		return false
	}
	return true
}

// Eval evaluates an ipv4 range filter
func (r *IPV4RangeFilter) Eval(g common.Getter) bool {
	re, found := regexpCache.Get(r.Value)
	if !found {
		// ignore error at this point should have been check in the contructor
		regex, _ := common.IPV4CIDRToRegex(r.Value)
		re = regexp.MustCompile(regex)
		regexpCache.Set(r.Value, re, cache.DefaultExpiration)
	}

	return evalStringField(g, r.Key, func(s string) bool {
		return re.(*regexp.Regexp).MatchString(s)
	})
}

// NewIPV4RangeFilter creates a regex based filter corresponding to the ip range
func NewIPV4RangeFilter(key, cidr string) (*IPV4RangeFilter, error) {
	regex, err := common.IPV4CIDRToRegex(cidr)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	regexpCache.Set(cidr, re, cache.DefaultExpiration)

	return &IPV4RangeFilter{Key: key, Value: cidr}, nil
}

// NewBoolFilter creates a new boolean filter
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

// NewAndFilter creates a new boolean And filter
func NewAndFilter(filters ...*Filter) *Filter {
	return NewBoolFilter(BoolFilterOp_AND, filters...)
}

// NewOrFilter creates a new boolean Or filter
func NewOrFilter(filters ...*Filter) *Filter {
	return NewBoolFilter(BoolFilterOp_OR, filters...)
}

// NewNotFilter creates a new boolean Not filter
func NewNotFilter(filter *Filter) *Filter {
	return NewBoolFilter(BoolFilterOp_NOT, filter)
}

// NewGtInt64Filter creates a new > filter
func NewGtInt64Filter(key string, value int64) *Filter {
	return &Filter{GtInt64Filter: &GtInt64Filter{Key: key, Value: value}}
}

// NewGteInt64Filter creates a new >= filter
func NewGteInt64Filter(key string, value int64) *Filter {
	return &Filter{GteInt64Filter: &GteInt64Filter{Key: key, Value: value}}
}

// NewLtInt64Filter creates a new < filter
func NewLtInt64Filter(key string, value int64) *Filter {
	return &Filter{LtInt64Filter: &LtInt64Filter{Key: key, Value: value}}
}

// NewLteInt64Filter creates a new <= filter
func NewLteInt64Filter(key string, value int64) *Filter {
	return &Filter{LteInt64Filter: &LteInt64Filter{Key: key, Value: value}}
}

// NewTermInt64Filter creates a new string iny64 filter
func NewTermInt64Filter(key string, value int64) *Filter {
	return &Filter{TermInt64Filter: &TermInt64Filter{Key: key, Value: value}}
}

// NewTermStringFilter creates a new string filter
func NewTermStringFilter(key string, value string) *Filter {
	return &Filter{TermStringFilter: &TermStringFilter{Key: key, Value: value}}
}

// NewTermBoolFilter creates a new bool filter
func NewTermBoolFilter(key string, value bool) *Filter {
	return &Filter{TermBoolFilter: &TermBoolFilter{Key: key, Value: value}}
}

// NewNullFilter creates a new null filter
func NewNullFilter(key string) *Filter {
	return &Filter{NullFilter: &NullFilter{Key: key}}
}

// NewOrTermStringFilter creates a new "or" filter based on values and attributes
func NewOrTermStringFilter(values []string, attrs ...string) *Filter {
	terms := make([]*Filter, len(values)*len(attrs))
	for i, value := range values {
		for j, attr := range attrs {
			terms[i*len(attrs)+j] = NewTermStringFilter(attr, value)
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

// NewNotNullFilter returns a filter that returns elements with a field set.
func NewNotNullFilter(key string) *Filter {
	return NewNotFilter(NewNullFilter(key))
}

func init() {
	regexpCache = cache.New(5*time.Minute, 10*time.Minute)
}
