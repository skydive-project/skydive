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

package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrCantCompareInterface error can't compare inteface
	ErrCantCompareInterface = errors.New("Can't compare interface")
	// ErrFieldNotFound error field not found
	ErrFieldNotFound = errors.New("Field not found")
	// ErrFieldWrongType error field has wrong type
	ErrFieldWrongType = errors.New("Field has wrong type")
	// ErrNotFound error no result was found
	ErrNotFound = errors.New("No result found")
	// ErrTimeout network timeout
	ErrTimeout = errors.New("Timeout")
)

// SortOrder describes ascending or descending order
type SortOrder string

const (
	// SortAscending sorting order
	SortAscending SortOrder = "ASC"
	// SortDescending sorting order
	SortDescending SortOrder = "DESC"
)

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

func integerCompare(a interface{}, b interface{}) (int, error) {
	n1, err := ToInt64(a)
	if err != nil {
		return 0, err
	}

	n2, err := ToInt64(b)
	if err != nil {
		return 0, err
	}

	if n1 == n2 {
		return 0, nil
	} else if n1 < n2 {
		return -1, nil
	} else {
		return 1, nil
	}
}

// ToFloat64 Convert all number like type to float64
func ToFloat64(f interface{}) (float64, error) {
	switch v := f.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return float64(i), nil
		}
		if f, err := v.Float64(); err == nil {
			return f, nil
		}
	case string:
		return strconv.ParseFloat(v, 64)
	case int, uint, int32, uint32, int64, uint64:
		i, err := ToInt64(f)
		if err != nil {
			return 0, err
		}
		return float64(i), nil
	case float32:
		return float64(v), nil
	case float64:
		return f.(float64), nil
	}
	return 0, fmt.Errorf("not a float: %v", f)
}

func floatCompare(a interface{}, b interface{}) (int, error) {
	f1, err := ToFloat64(a)
	if err != nil {
		return 0, err
	}

	f2, err := ToFloat64(b)
	if err != nil {
		return 0, err
	}

	if f1 == f2 {
		return 0, nil
	} else if f1 < f2 {
		return -1, nil
	} else {
		return 1, nil
	}
}

// CrossTypeCompare compare 2 differents number types like Float64 vs Float32
func CrossTypeCompare(a interface{}, b interface{}) (int, error) {
	switch a.(type) {
	case float32, float64:
		return floatCompare(a, b)
	}

	switch b.(type) {
	case float32, float64:
		return floatCompare(a, b)
	}

	switch a.(type) {
	case int, uint, int32, uint32, int64, uint64:
		return integerCompare(a, b)
	default:
		return 0, ErrCantCompareInterface
	}
}

// CrossTypeEqual compare 2 differents number types
func CrossTypeEqual(a interface{}, b interface{}) bool {
	result, err := CrossTypeCompare(a, b)
	if err == ErrCantCompareInterface {
		return a == b
	} else if err != nil {
		return false
	}
	return result == 0
}

// MinInt64 returns the lowest value
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// MaxInt64 returns the biggest value
func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// IPv6Supported returns true if the platform support IPv6
func IPv6Supported() bool {
	if _, err := os.Stat("/proc/net/if_inet6"); os.IsNotExist(err) {
		return false
	}

	data, err := ioutil.ReadFile("/proc/sys/net/ipv6/conf/all/disable_ipv6")
	if err != nil {
		return false
	}

	if strings.TrimSpace(string(data)) == "1" {
		return false
	}

	return true
}

// IPToString convert IPv4 or IPv6 to a string
func IPToString(ip net.IP) string {
	if ip.To4() == nil {
		return "[" + ip.String() + "]"
	}
	return ip.String()
}

// JSONDecode wrapper to UseNumber during JSON decoding
func JSONDecode(r io.Reader, i interface{}) error {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	return decoder.Decode(i)
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

// Metric defines accessors
type Metric interface {
	GetFieldInt64(field string) (int64, error)
	Add(m Metric) Metric
}

// TimedMetric defines Metric during a time slice
type TimedMetric struct {
	TimeSlice
	Metric Metric
}

// GetFieldInt64 returns the field value
func (tm *TimedMetric) GetFieldInt64(field string) (int64, error) {
	return tm.Metric.GetFieldInt64(field)
}

// MarshalJSON serialized a TimedMetric in JSON
func (tm *TimedMetric) MarshalJSON() ([]byte, error) {
	var s string
	if tm.Metric != nil {
		b, err := json.Marshal(tm.Metric)
		if err != nil {
			return nil, err
		}
		s = fmt.Sprintf(`{"Start":%d,"Last":%d,%s`, tm.Start, tm.Last, string(b[1:]))
	} else {
		s = "null"
	}
	return []byte(s), nil
}

// SetField set a value in a tree based on dot key ("a.b.c.d" = "ok")
func SetField(obj map[string]interface{}, k string, v interface{}) bool {
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

// DelField detete a value in a tree based on dot key
func DelField(obj map[string]interface{}, k string) interface{} {
	components := strings.Split(k, ".")
	o, ok := obj[components[0]]
	if !ok {
		return obj
	}
	if len(components) == 1 {
		delete(obj, k)
		return obj
	}

	object, ok := o.(map[string]interface{})
	if !ok {
		return obj
	}
	r := DelField(object, strings.SplitN(k, ".", 2)[1])
	if len(r.(map[string]interface{})) == 0 {
		delete(obj, components[0])
	}

	return obj
}

// GetField retrieve a value from a tree from the dot key like "a.b.c.d"
func GetField(obj map[string]interface{}, k string) (interface{}, error) {
	components := strings.Split(k, ".")
	for n, component := range components {
		i, ok := obj[component]
		if !ok {
			return nil, ErrFieldNotFound
		}

		if n == len(components)-1 {
			return i, nil
		}

		if list, ok := i.([]interface{}); ok {
			var results []interface{}
			for _, v := range list {
				switch v := v.(type) {
				case map[string]interface{}:
					if obj, err := GetField(v, strings.Join(components[n+1:], ".")); err == nil {
						results = append(results, obj)
					}
				}
			}
			return results, nil
		}

		if obj, ok = i.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("%s is not a map, but a %+v", component, reflect.TypeOf(obj))
		}
	}

	return obj, nil
}

func splitToRanges(min, max int) []int {
	stops := make(map[int]bool)

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
