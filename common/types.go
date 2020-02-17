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

package common

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/structs"
	"github.com/mohae/deepcopy"
)

var (
	// ErrCantCompareInterface error can't compare interface
	ErrCantCompareInterface = errors.New("Can't compare interface")
	// ErrFieldNotFound error field not found
	ErrFieldNotFound = errors.New("Field not found")
	// ErrFieldWrongType error field has wrong type
	ErrFieldWrongType = errors.New("Field has wrong type")
	// ErrNotFound error no result was found
	ErrNotFound = errors.New("No result found")
	// ErrTimeout network timeout
	ErrTimeout = errors.New("Timeout")
	// ErrNotImplemented unimplemented feature
	ErrNotImplemented = errors.New("Not implemented")
)

// SortOrder describes ascending or descending order
type SortOrder string

const (
	// SortAscending sorting order
	SortAscending SortOrder = "ASC"
	// SortDescending sorting order
	SortDescending SortOrder = "DESC"
)

// BoolPredicate is a function that applies a test against a boolean
type BoolPredicate func(b bool) bool

// Int64Predicate is a function that applies a test against an integer
type Int64Predicate func(i int64) bool

// StringPredicate is a function that applies a test against a string
type StringPredicate func(s string) bool

// Getter describes filter getter fields
type Getter interface {
	GetField(field string) (interface{}, error)
	GetFieldKeys() []string
	GetFieldBool(field string) (bool, error)
	GetFieldInt64(field string) (int64, error)
	GetFieldString(field string) (string, error)
	MatchBool(field string, predicate BoolPredicate) bool
	MatchInt64(field string, predicate Int64Predicate) bool
	MatchString(field string, predicate StringPredicate) bool
}

// ToInt64 Convert all number like type to int64
func ToInt64(i interface{}) (int64, error) {
	switch v := i.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, nil
		}
		if f, err := v.Float64(); err == nil {
			return int64(f), nil
		}
	case string:
		return strconv.ParseInt(v, 10, 64)
	case int:
		return int64(v), nil
	case uint:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	}
	return 0, fmt.Errorf("failed to convert to an integer: %v", i)
}

// NormalizeValue returns a version of the passed value
// that can be safely marshalled to JSON
func NormalizeValue(obj interface{}) interface{} {
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

// UnixMillis returns the current time in miliseconds
func UnixMillis(t time.Time) int64 {
	return t.UTC().UnixNano() / 1000000
}

// TimeSlice defines a time boudary values
type TimeSlice struct {
	Start int64 `json:"Start"`
	Last  int64 `json:"Last"`
}

// NewTimeSlice creates a new TimeSlice based on Start and Last
func NewTimeSlice(s, l int64) *TimeSlice {
	return &TimeSlice{Start: s, Last: l}
}

// Metric defines a common metric interface
type Metric interface {
	// part of the Getter interface
	GetFieldInt64(field string) (int64, error)
	GetFieldKeys() []string

	Add(m Metric) Metric
	Sub(m Metric) Metric
	Split(cut int64) (Metric, Metric)
	GetStart() int64
	SetStart(start int64)
	GetLast() int64
	SetLast(last int64)
	IsZero() bool
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
			return nil, ErrFieldNotFound
		}

		if n == len(components)-1 {
			return i, nil
		}

		subkey := strings.Join(components[n+1:], ".")

		switch i.(type) {
		case Getter:
			return i.(Getter).GetField(subkey)
		case []interface{}:
			var results []interface{}
			for _, v := range i.([]interface{}) {
				switch v := v.(type) {
				case Getter:
					if obj, err := v.(Getter).GetField(subkey); err == nil {
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

func getFieldKeys(obj map[string]interface{}, path string) []string {
	var fields []string

	if path != "" {
		path += "."
	}

	for k, v := range obj {
		fields = append(fields, path+k)

		switch v.(type) {
		case Getter:
			keys := v.(Getter).GetFieldKeys()
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

func splitToRanges(min, max int) []int {
	stops := map[int]bool{max: true}

	ninesCount := 1
	stop := fillByNines(min, ninesCount)
	for min <= stop && stop < max {
		stops[stop] = true

		ninesCount++
		stop = fillByNines(min, ninesCount)
	}

	zerosCount := 1
	stop = fillByZeros(max+1, zerosCount) - 1
	for min < stop && stop <= max {
		stops[stop] = true

		zerosCount++
		stop = fillByZeros(max+1, zerosCount) - 1
	}

	var sr []int
	for i := range stops {
		sr = append(sr, i)
	}

	sort.Ints(sr)

	return sr
}

func rangeToPattern(start, stop int) string {
	pattern := ""
	anyDigitCount := 0

	startStr, stopStr := strconv.Itoa(start), strconv.Itoa(stop)
	for i := 0; i != len(startStr); i++ {
		startDigit, stopDigit := string(startStr[i]), string(stopStr[i])

		if startDigit == stopDigit {
			pattern += startDigit
		} else if startDigit != "0" || stopDigit != "9" {
			pattern += fmt.Sprintf("[%s-%s]", startDigit, stopDigit)
		} else {
			anyDigitCount++
		}
	}

	if anyDigitCount > 0 {
		pattern += "[0-9]"
	}

	if anyDigitCount > 1 {
		pattern += fmt.Sprintf("{%d}", anyDigitCount)
	}

	return pattern
}

func splitToPatterns(min, max int) []string {
	var subpatterns []string

	start := min
	for _, stop := range splitToRanges(min, max) {
		subpatterns = append(subpatterns, rangeToPattern(start, stop))
		start = stop + 1
	}

	return subpatterns
}

func fillByNines(i, count int) int {
	str := strconv.Itoa(i)

	var prefix string
	if count < len(str) {
		prefix = str[:len(str)-count]
	}
	i, _ = strconv.Atoi(prefix + strings.Repeat("9", count))
	return i
}

func fillByZeros(i, count int) int {
	return i - i%int(math.Pow10(count))
}

// RangeToRegex returns a regular expression matching number in the given range
// Golang version of https://github.com/dimka665/range-regex
func RangeToRegex(min, max int) string {
	subpatterns := splitToPatterns(min, max)
	return strings.Join(subpatterns, "|")
}

// IPV4CIDRToRegex returns a regex matching IPs belonging to a given cidr
func IPV4CIDRToRegex(cidr string) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	netIP := ipnet.IP.To4()
	firstIP := netIP.Mask(ipnet.Mask)
	lastIP := net.IPv4(0, 0, 0, 0).To4()

	var regex string
	var groupMask bool
	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^ipnet.Mask[i]

		fip := int(firstIP[i])
		lip := int(lastIP[i])

		if regex != "" {
			regex += `\.`
		}

		if fip == lip {
			if !groupMask {
				regex += "("
				groupMask = true
			}

			regex += strconv.Itoa(fip)
		} else {
			if groupMask {
				regex += ")"
				groupMask = false
			}

			regex += "(" + RangeToRegex(fip, lip) + ")"
		}
	}

	if groupMask {
		regex += ")"
	}
	return "^" + regex + `(\/[0-9]?[0-9])?$`, nil
}

// IPStrToUint32 converts IP string to 32bits
func IPStrToUint32(ipAddr string) (uint32, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return 0, errors.New("wrong ipAddr format")
	}
	ip = ip.To4()
	if ip == nil {
		return 0, errors.New("wrong ipAddr format")
	}
	return binary.BigEndian.Uint32(ip), nil
}
