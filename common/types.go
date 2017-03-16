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
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	CantCompareInterface = errors.New("Can't compare interface")
	ErrFieldNotFound     = errors.New("Field not found")
)

type SortOrder string

const (
	SortAscending  SortOrder = "ASC"
	SortDescending SortOrder = "DESC"
)

func ToInt64(i interface{}) (int64, error) {
	switch v := i.(type) {
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
	return 0, fmt.Errorf("not an integer: %v", i)
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

func ToFloat64(f interface{}) (float64, error) {
	switch v := f.(type) {
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
		return 0, CantCompareInterface
	}
}

func CrossTypeEqual(a interface{}, b interface{}) bool {
	result, err := CrossTypeCompare(a, b)
	if err == CantCompareInterface {
		return a == b
	} else if err != nil {
		return false
	}
	return result == 0
}

func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

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

func IPToString(ip net.IP) string {
	if ip.To4() == nil {
		return "[" + ip.String() + "]"
	}
	return ip.String()
}

func JsonDecode(r io.Reader, i interface{}) error {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	return decoder.Decode(i)
}

func UnixMillis(t time.Time) int64 {
	return t.UTC().UnixNano() / 1000000
}

type TimeSlice struct {
	Start int64 `json:"Start"`
	Last  int64 `json:"Last"`
}

func NewTimeSlice(s, l int64) *TimeSlice {
	return &TimeSlice{Start: s, Last: l}
}

type Metric interface {
	GetField(field string) (int64, error)
	Add(m Metric) Metric
}

type TimedMetric struct {
	TimeSlice
	Metric Metric
}

func (tm *TimedMetric) GetField(field string) (int64, error) {
	return tm.Metric.GetField(field)
}

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
