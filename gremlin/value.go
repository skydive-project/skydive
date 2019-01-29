/*
 * Copyright (C) 2018 IBM, Inc.
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

package gremlin

import (
	"fmt"
	"strconv"
)

// ValueString a value used within query constructs
type ValueString string

// NewValueStringFromArgument via inferance creates a correct ValueString
func NewValueStringFromArgument(v interface{}) ValueString {
	switch t := v.(type) {
	case ValueString:
		return t
	case string:
		return Quote(t)
	case fmt.Stringer:
		return Quote(t.String())
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return ValueString(fmt.Sprintf("%d", t))
	case bool:
		return ValueString(strconv.FormatBool(t))
	default:
		panic(fmt.Sprintf("argument %v: type %T not supported", v, t))
	}
}

// String converts value to string
func (v ValueString) String() string {
	return string(v)
}

// DESC const definition
const DESC = ValueString("DESC")

// Quote used to quote string values as needed by query
func Quote(format string, a ...interface{}) ValueString {
	s := fmt.Sprintf(format, a...)
	return ValueString(fmt.Sprintf(`"%s"`, s))
}

// Regex used for constructing a regexp expression string
func Regex(format string, a ...interface{}) ValueString {
	s := fmt.Sprintf(format, a...)
	return ValueString(fmt.Sprintf("Regex(%s)", Quote(s)))
}

func newValueString(name string, list ...interface{}) ValueString {
	s := fmt.Sprintf("%s(", name)
	first := true
	for _, v := range list {
		if !first {
			s = s + ", "
		}
		first = false
		s = s + NewValueStringFromArgument(v).String()
	}
	return ValueString(s + ")")
}

// Between append a Between() operation to query
func Between(list ...interface{}) ValueString {
	return newValueString("Between", list...)
}

// Gt append a Gt() operation to query
func Gt(v interface{}) ValueString {
	return newValueString("Gt", v)
}

// Gte append a Gte() operation to query
func Gte(v interface{}) ValueString {
	return newValueString("Gte", v)
}

// Ipv4Range append a Ipv4Range() operation to query
func Ipv4Range(list ...interface{}) ValueString {
	return newValueString("Ipv4Range", list...)
}

// Inside append a Inside() operation to query
func Inside(list ...interface{}) ValueString {
	return newValueString("Inside", list...)
}

// Lt append a Lt() operation to query
func Lt(v interface{}) ValueString {
	return newValueString("Lt", v)
}

// Lte append a Lte() operation to query
func Lte(v interface{}) ValueString {
	return newValueString("Lte", v)
}

// Metadata append a Metadata() operation to query
func Metadata(list ...interface{}) ValueString {
	return newValueString("Metadata", list...)
}

// Ne append a Ne() operation to query
func Ne(v interface{}) ValueString {
	return newValueString("Ne", v)
}

// Within append a Within() operation to query
func Within(list ...interface{}) ValueString {
	return newValueString("Within", list...)
}
