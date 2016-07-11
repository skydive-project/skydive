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
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/skydive-project/skydive/common"
)

type TermFilterOp int

const (
	OR TermFilterOp = iota
	AND
)

type GetAttr interface {
	GetAttr(name string) interface{}
}

type Filter interface {
	String() string
	Eval(value interface{}) bool
}

type TermFilter struct {
	Key   string
	Value interface{}
}

type RangeFilter struct {
	Key string
	Gt  interface{} `json:"Gt,omitempty"`
	Lt  interface{} `json:"Lt,omitempty"`
	Gte interface{} `json:"Gte,omitempty"`
	Lte interface{} `json:"Lte,omitempty"`
}

type BoolFilter struct {
	Op      TermFilterOp
	Filters []Filter
}

func (b BoolFilter) String() string {
	keyword := ""
	switch b.Op {
	case OR:
		keyword = "OR"
	case AND:
		keyword = "AND"
	}
	var conditions []string
	for _, item := range b.Filters {
		conditions = append(conditions, "("+item.String()+")")
	}
	return strings.Join(conditions, " "+keyword+" ")
}

func (b BoolFilter) Eval(value interface{}) bool {
	for _, term := range b.Filters {
		result := term.Eval(value)
		if b.Op == AND && !result {
			return false
		} else if b.Op == OR && result {
			return true
		}
	}
	return b.Op == AND || len(b.Filters) == 0
}

func (r RangeFilter) String() string {
	var predicates []string
	if r.Gt != nil {
		predicates = append(predicates, fmt.Sprintf("%v > %v", r.Key, r.Gt))
	}
	if r.Lt != nil {
		predicates = append(predicates, fmt.Sprintf("%v < %v", r.Key, r.Lt))
	}
	if r.Gte != nil {
		predicates = append(predicates, fmt.Sprintf("%v >= %v", r.Key, r.Gte))
	}
	if r.Lte != nil {
		predicates = append(predicates, fmt.Sprintf("%v <= %v", r.Key, r.Lte))
	}
	return strings.Join(predicates, " AND ")
}

func (r RangeFilter) Eval(value interface{}) bool {
	field := GetFields(value, strings.Split(r.Key, "."))
	if field == nil {
		return false
	}

	if r.Gt != nil {
		if result, err := common.CrossTypeCompare(field, r.Gt); err != nil || result != 1 {
			return false
		}
	}
	if r.Lt != nil {
		if result, err := common.CrossTypeCompare(field, r.Lt); err != nil || result != -1 {
			return false
		}
	}
	if r.Gte != nil {
		if result, err := common.CrossTypeCompare(field, r.Gte); err != nil || result == -1 {
			return false
		}
	}
	if r.Lte != nil {
		if result, err := common.CrossTypeCompare(field, r.Lte); err != nil || result == 1 {
			return false
		}
	}

	return true
}

func (t TermFilter) String() string {
	marshal, err := json.Marshal(t.Value)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s = %s", t.Key, marshal)
}

func (t TermFilter) Eval(value interface{}) bool {
	field := GetFields(value, strings.Split(t.Key, "."))
	if field == nil {
		return false
	}

	return common.CrossTypeEqual(field, t.Value)
}

func decodeQuery(data interface{}) (result interface{}, err error) {
	switch t := data.(type) {
	case map[string]interface{}:
		filtersValue, hasFilters := t["Filters"]
		opValue, hasOp := t["Op"]
		if hasFilters && hasOp {
			op := int(opValue.(float64))
			filters, err := decodeQuery(filtersValue)
			if err != nil {
				return nil, err
			}

			return BoolFilter{
				Op:      TermFilterOp(op),
				Filters: filters.([]Filter),
			}, nil
		}

		_, hasKey := t["Key"]
		_, hasValue := t["Value"]
		if hasKey && hasValue {
			termFilter := TermFilter{}
			if err := mapstructure.Decode(t, &termFilter); err != nil {
				return nil, err
			}
			return termFilter, nil
		}

		if hasKey {
			rangeFilter := RangeFilter{}
			if err := mapstructure.Decode(t, &rangeFilter); err != nil {
				return nil, err
			}
			return rangeFilter, nil
		}

	case []interface{}:
		items := make([]Filter, len(t))
		for i, item := range t {
			obj, err := decodeQuery(item)
			if err != nil {
				return nil, err
			}
			items[i] = obj.(Filter)
		}
		return items, nil
	}

	return nil, fmt.Errorf("Could not decode %+v", data)
}

func DecodeFilter(reader io.Reader) (Filter, error) {
	var mapInterface map[string]interface{}
	jsonDecoder := json.NewDecoder(reader)
	err := jsonDecoder.Decode(&mapInterface)
	if err != nil {
		return nil, err
	}
	obj, err := decodeQuery(mapInterface["Obj"])
	if err != nil {
		return nil, err
	}
	filter, ok := obj.(Filter)
	if !ok {
		return nil, fmt.Errorf("Query filter has wrong type : %+v", reflect.TypeOf(obj))
	}
	return filter, nil
}
