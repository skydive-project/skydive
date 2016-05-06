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

import "fmt"

func toInt64(i interface{}) (int64, error) {
	switch i.(type) {
	case int:
		return int64(i.(int)), nil
	case uint:
		return int64(i.(uint)), nil
	case int32:
		return int64(i.(int32)), nil
	case uint32:
		return int64(i.(uint32)), nil
	case int64:
		return i.(int64), nil
	case uint64:
		return int64(i.(uint64)), nil
	case float32:
		return int64(i.(float32)), nil
	case float64:
		return int64(i.(float64)), nil
	}
	return 0, fmt.Errorf("not an integer: %v", i)
}

func integerEqual(a interface{}, b interface{}) bool {
	n1, err := toInt64(a)
	if err != nil {
		return false
	}

	n2, err := toInt64(b)
	if err != nil {
		return false
	}
	return n1 == n2
}

func toFloat64(f interface{}) (float64, error) {
	switch f.(type) {
	case int, uint, int32, uint32, int64, uint64:
		i, err := toInt64(f)
		if err != nil {
			return 0, err
		}
		return float64(i), nil
	case float32:
		return float64(f.(float32)), nil
	case float64:
		return f.(float64), nil
	}
	return 0, fmt.Errorf("not a float: %v", f)
}

func floatEqual(a interface{}, b interface{}) bool {
	f1, err := toFloat64(a)
	if err != nil {
		return false
	}

	f2, err := toFloat64(b)
	if err != nil {
		return false
	}

	return f1 == f2
}

func CrossTypeEqual(a interface{}, b interface{}) bool {
	switch a.(type) {
	case float32, float64:
		return floatEqual(a, b)
	}

	switch b.(type) {
	case float32, float64:
		return floatEqual(a, b)
	}

	switch a.(type) {
	case int, uint, int32, uint32, int64, uint64:
		return integerEqual(a, b)
	default:
		return a == b
	}
}
