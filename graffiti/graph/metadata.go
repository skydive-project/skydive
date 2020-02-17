//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

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
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/mohae/deepcopy"
	"github.com/spf13/cast"

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
		case Metadata:
			filters, err := v.Filter()
			if err != nil {
				return nil, err
			}
			termFilters = append(termFilters, filters)
		case map[string]interface{}:
			sm := Metadata(v)
			filters, err := sm.Filter()
			if err != nil {
				return nil, err
			}
			termFilters = append(termFilters, filters)
		default:
			i, err := cast.ToInt64E(v)
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
	value, err := GetMapField(m, key)
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
	value, err := GetMapField(m, key)
	if err != nil {
		return 0, err
	}

	return cast.ToInt64E(value)
}

// GetFieldString returns the value of a string field
func (m Metadata) GetFieldString(key string) (string, error) {
	value, err := GetMapField(m, key)
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
	return GetMapField(m, key)
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
			if v, err := cast.ToInt64E(intf); err == nil && predicate(v) {
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
		if v, err := cast.ToInt64E(field); err == nil {
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

func getFieldKeys(obj map[string]interface{}, path string) []string {
	var fields []string

	if path != "" {
		path += "."
	}

	for k, v := range obj {
		fields = append(fields, path+k)

		switch v.(type) {
		case common.Getter:
			keys := v.(common.Getter).GetFieldKeys()
			for _, subkey := range keys {
				fields = append(fields, k+"."+subkey)
			}
		case map[string]interface{}:
			subfields := getFieldKeys(v.(map[string]interface{}), path+k)
			fields = append(fields, subfields...)
		}
	}

	return fields
}

// GetMapFieldKeys returns all the keys using dot notation
func GetMapFieldKeys(obj map[string]interface{}) []string {
	return getFieldKeys(obj, "")
}

// GetFieldKeys returns all the keys of the medata
func (m Metadata) GetFieldKeys() []string {
	return GetMapFieldKeys(m)
}

// SetField set metadata value based on dot key ("a.b.c.d" = "ok")
func (m *Metadata) SetField(k string, v interface{}) bool {
	return SetMapField(*m, k, v)
}

// DelField removes a dot based key from metadata
func (m Metadata) DelField(k string) bool {
	return DelField(m, k)
}

// SetFieldAndNormalize set metadata value after normalization (and deepcopy)
func (m *Metadata) SetFieldAndNormalize(k string, v interface{}) bool {
	return SetMapField(*m, k, NormalizeValue(v))
}

// SetMapField set a value in a tree based on dot key ("a.b.c.d" = "ok")
func SetMapField(obj map[string]interface{}, k string, v interface{}) bool {
	components := strings.Split(k, ".")
	for n, component := range components {
		if n == len(components)-1 {
			obj[component] = v
		} else {
			m, ok := obj[component]
			if !ok {
				m := make(map[string]interface{})
				obj[component] = m
				obj = m
			} else if obj, ok = m.(map[string]interface{}); !ok {
				return false
			}
		}
	}
	return true
}

// NormalizeValue returns a version of the passed value
// that can be safely marshalled to JSON
func NormalizeValue(obj interface{}) interface{} {
	// TODO(sbaubeau) should return Metadata
	obj = deepcopy.Copy(obj)
	if structs.IsStruct(obj) {
		obj = structs.Map(obj)
	}
	switch v := obj.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(v))
		for key, value := range v {
			SetMapField(m, key, NormalizeValue(value))
		}
		return m
	case map[interface{}]interface{}:
		m := make(map[string]interface{}, len(v))
		for key, value := range v {
			SetMapField(m, key.(string), NormalizeValue(value))
		}
		return m
	case map[string]string:
		m := make(map[string]interface{}, len(v))
		for key, value := range v {
			SetMapField(m, key, value)
		}
		return m
	case []interface{}:
		for i, val := range v {
			v[i] = NormalizeValue(val)
		}
	case string:
		return v
	case nil:
		return ""
	}
	return obj
}

// DelField deletes a value in a tree based on dot key
func DelField(obj map[string]interface{}, k string) bool {
	components := strings.Split(k, ".")
	o, ok := obj[components[0]]
	if !ok {
		return false
	}
	if len(components) == 1 {
		oldLength := len(obj)
		delete(obj, k)
		return oldLength != len(obj)
	}

	object, ok := o.(map[string]interface{})
	if !ok {
		return false
	}
	removed := DelField(object, strings.SplitN(k, ".", 2)[1])
	if removed && len(object) == 0 {
		delete(obj, components[0])
	}

	return removed
}

// GetMapField retrieves a value from a tree from the dot key like "a.b.c.d"
func GetMapField(obj map[string]interface{}, k string) (interface{}, error) {
	components := strings.Split(k, ".")
	for n, component := range components {
		i, ok := obj[component]
		if !ok {
			return nil, common.ErrFieldNotFound
		}

		if n == len(components)-1 {
			return i, nil
		}

		subkey := strings.Join(components[n+1:], ".")

		switch i.(type) {
		case common.Getter:
			return i.(common.Getter).GetField(subkey)
		case []interface{}:
			var results []interface{}
			for _, v := range i.([]interface{}) {
				switch v := v.(type) {
				case common.Getter:
					if obj, err := v.(common.Getter).GetField(subkey); err == nil {
						results = append(results, obj)
					}
				case map[string]interface{}:
					if obj, err := GetMapField(v, subkey); err == nil {
						results = append(results, obj)
					}
				}
			}
			return results, nil
		case map[string]interface{}:
			obj = i.(map[string]interface{})
		default:
			return nil, fmt.Errorf("%s is not a supported type(%+v)", component, reflect.TypeOf(obj))
		}
	}

	return obj, nil
}
