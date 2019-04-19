/*
 * Copyright (C) 2019 Red Hat, Inc.
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
	"strings"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
)

// Metadata describes the graph node metadata type. It implements ElementMatcher
// based only on Metadata.
// easyjson:json
type Metadata map[string]interface{}

// Match returns true if the the given element matches the metadata.
func (m Metadata) Match(g common.Getter) bool {
	f, err := m.Filter()
	if err != nil {
		return false
	}
	return f.Eval(g)
}

// Filter returns a filter corresponding to the metadata
func (m Metadata) Filter() (*filters.Filter, error) {
	var termFilters []*filters.Filter
	for k, v := range m {
		switch v := v.(type) {
		case int64:
			termFilters = append(termFilters, filters.NewTermInt64Filter(k, v))
		case string:
			termFilters = append(termFilters, filters.NewTermStringFilter(k, v))
		case bool:
			termFilters = append(termFilters, filters.NewTermBoolFilter(k, v))
		case map[string]interface{}:
			sm := Metadata(v)
			filters, err := sm.Filter()
			if err != nil {
				return nil, err
			}
			termFilters = append(termFilters, filters)
		default:
			i, err := common.ToInt64(v)
			if err != nil {
				return nil, err
			}
			termFilters = append(termFilters, filters.NewTermInt64Filter(k, i))
		}
	}
	return filters.NewAndFilter(termFilters...), nil
}

// GetFieldBool returns the value of a bool field
func (m Metadata) GetFieldBool(key string) (bool, error) {
	value, err := common.GetMapField(m, key)
	if err != nil {
		return false, err
	}

	if b, ok := value.(bool); ok {
		return b, nil
	}

	return false, common.ErrFieldNotFound
}

// GetFieldInt64 returns the value of a integer field
func (m Metadata) GetFieldInt64(key string) (int64, error) {
	value, err := common.GetMapField(m, key)
	if err != nil {
		return 0, err
	}

	return common.ToInt64(value)
}

// GetFieldString returns the value of a string field
func (m Metadata) GetFieldString(key string) (string, error) {
	value, err := common.GetMapField(m, key)
	if err != nil {
		return "", err
	}

	if s, ok := value.(string); ok {
		return s, nil
	}

	return "", common.ErrFieldNotFound
}

// GetField returns the field with the given name
func (m Metadata) GetField(key string) (interface{}, error) {
	return common.GetMapField(m, key)
}

// MatchBool implements Getter interface
func (m Metadata) MatchBool(key string, predicate common.BoolPredicate) bool {
	if index := strings.Index(key, "."); index != -1 {
		first := key[:index]
		if v, found := m[first]; found {
			if getter, found := v.(common.Getter); found {
				return getter.MatchBool(key[index+1:], predicate)
			}
		}
	}

	field, err := m.GetField(key)
	if err != nil {
		return false
	}

	switch field := field.(type) {
	case []interface{}:
		for _, intf := range field {
			if b, ok := intf.(bool); ok && predicate(b) {
				return true
			}
		}
	case []bool:
		for _, v := range field {
			if predicate(v) {
				return true
			}
		}
	case bool:
		return predicate(field)

	}

	return false
}

// MatchInt64 implements Getter interface
func (m Metadata) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if index := strings.Index(key, "."); index != -1 {
		first := key[:index]
		if v, found := m[first]; found {
			if getter, found := v.(common.Getter); found {
				return getter.MatchInt64(key[index+1:], predicate)
			}
		}
	}

	field, err := m.GetField(key)
	if err != nil {
		return false
	}

	switch field := field.(type) {
	case []interface{}:
		for _, intf := range field {
			if v, err := common.ToInt64(intf); err == nil && predicate(v) {
				return true
			}
		}
	case []int64:
		for _, v := range field {
			if predicate(v) {
				return true
			}
		}
	case int64:
		return predicate(field)
	default:
		if v, err := common.ToInt64(field); err == nil {
			return predicate(v)
		}
	}

	return false
}

// MatchString implements Getter interface
func (m Metadata) MatchString(key string, predicate common.StringPredicate) bool {
	if index := strings.Index(key, "."); index != -1 {
		first := key[:index]
		if v, found := m[first]; found {
			if getter, found := v.(common.Getter); found {
				return getter.MatchString(key[index+1:], predicate)
			}
		}
	}

	field, err := m.GetField(key)
	if err != nil {
		return false
	}

	switch field := field.(type) {
	case []interface{}:
		for _, intf := range field {
			if s, ok := intf.(string); ok && predicate(s) {
				return true
			}
		}
	case []string:
		for _, s := range field {
			if predicate(s) {
				return true
			}
		}
	case string:
		return predicate(field)
	}

	return false
}

// GetFieldKeys returns all the keys of the medata
func (m Metadata) GetFieldKeys() []string {
	return common.GetMapFieldKeys(m)
}

// SetField set metadata value based on dot key ("a.b.c.d" = "ok")
func (m *Metadata) SetField(k string, v interface{}) bool {
	return common.SetMapField(*m, k, v)
}

// SetFieldAndNormalize set metadata value after normalization (and deepcopy)
func (m *Metadata) SetFieldAndNormalize(k string, v interface{}) bool {
	return common.SetMapField(*m, k, common.NormalizeValue(v))
}
