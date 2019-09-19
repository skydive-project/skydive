/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"fmt"
	"reflect"
	"regexp"
	"testing"
)

func TestIPV4Range24(t *testing.T) {
	expr, err := IPV4CIDRToRegex("192.168.0.0/24")
	if err != nil {
		t.Error(err)
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		t.Error(err)
	}

	for i := 0; i != 254; i++ {
		ip := fmt.Sprintf("192.168.0.%d/24", i)

		if !re.MatchString(ip) {
			t.Errorf("%s not matching the rexp %s", ip, expr)
		}
	}

	ip := "192.168.1.0/24"
	if re.MatchString(ip) {
		t.Errorf("%s matches the rexp %s", ip, expr)
	}

	ip = "192.168.1.34/24"
	if re.MatchString(ip) {
		t.Errorf("%s matches the rexp %s", ip, expr)
	}
}

func TestIPV4Range16(t *testing.T) {
	expr, err := IPV4CIDRToRegex("192.168.0.0/16")
	if err != nil {
		t.Error(err)
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		t.Error(err)
	}

	for i := 0; i != 254; i++ {
		for j := 0; j != 254; j++ {
			ip := fmt.Sprintf("192.168.0.%d/24", i)

			if !re.MatchString(ip) {
				t.Errorf("%s not matching the rexp %s", ip, expr)
			}
		}
	}

	ip := "192.169.3.34/24"
	if re.MatchString(ip) {
		t.Errorf("%s matches the rexp %s", ip, expr)
	}
}

func TestNormalizeStructToMap(t *testing.T) {
	type (
		B struct {
			C1 string
			C2 string
			C3 string
		}
		A struct {
			B B
		}
	)

	before := A{
		B: B{
			C1: "ccc",
		},
	}

	expected := map[string]interface{}{
		"B": map[string]interface{}{
			"C1": "ccc",
			"C2": "",
			"C3": "",
		},
	}

	actual := NormalizeValue(before)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %+v actual %+v", expected, actual)
	}
}

func TestNormalizeMapKeys(t *testing.T) {
	before := map[string]interface{}{
		"a.b": "A.B",
		"d":   "D",
	}

	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "A.B",
		},
		"d": "D",
	}

	actual := NormalizeValue(before)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %+v actual %+v", expected, actual)
	}
}

func TestSetField(t *testing.T) {
	actual := map[string]interface{}{
		"a": map[string]interface{}{
			"b": true,
		},
	}

	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": false,
			"d": map[string]interface{}{
				"c": true,
			},
		},
	}

	if SetMapField(actual, "a.b.c", true) {
		t.Errorf("Expected SetField to not overwrite any key")
	}

	if !SetMapField(actual, "a.b", false) {
		t.Errorf("Expected SetField to overwrite a.b")
	}

	if !SetMapField(actual, "a.d.c", true) {
		t.Errorf("Expected SetField to create a.d.c key")
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %+v actual %+v", expected, actual)
	}
}

func TestDelField(t *testing.T) {
	actual := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": true,
			},
		},
		"d": map[string]interface{}{
			"e": true,
			"f": false,
		},
	}

	expected := map[string]interface{}{
		"d": map[string]interface{}{
			"f": false,
		},
	}

	if !DelField(actual, "a.b.c") {
		t.Errorf("Expected DelField to remove a.b.c")
	}

	if !DelField(actual, "d.e") {
		t.Errorf("Expected DelField to remove d.e")
	}

	if DelField(actual, "d.g") {
		t.Errorf("Expected DelField to return false")
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %+v actual %+v", expected, actual)
	}
}

func TestGetMapField(t *testing.T) {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "c_value",
			},
		},
		"d": map[string]interface{}{
			"e": "e_value",
			"f": []interface{}{
				map[string]interface{}{
					"name": "f_0_name",
					"type": "f_0_type",
				},
				map[string]interface{}{
					"name": "f_1_name",
					"type": "f_1_type",
				},
				map[string]interface{}{
					"name": "f_2_name",
					"type": "f_2_type",
					"extra": map[string]interface{}{
						"g": "g_value",
					},
				},
			},
		},
	}

	type testInstance struct {
		key      string
		expected interface{}
	}
	tests := []testInstance{
		testInstance{"a.b", map[string]interface{}{"c": "c_value"}},
		testInstance{"a.b.c", "c_value"},
		testInstance{"d.e", "e_value"},
		testInstance{"d.f.name", []interface{}{"f_0_name", "f_1_name", "f_2_name"}},
		testInstance{"d.f.type", []interface{}{"f_0_type", "f_1_type", "f_2_type"}},
		testInstance{"d.f.extra.g", []interface{}{"g_value"}},
	}
	for _, ti := range tests {
		actual, err := GetMapField(data, ti.key)
		if err != nil {
			t.Errorf("Expected GetMapField to find the key: %s", ti.key)
		}
		if !reflect.DeepEqual(ti.expected, actual) {
			t.Errorf("Key: %s, Expected: %+v, actual: %+v", ti.key, ti.expected, actual)
		}
	}

	_, err := GetMapField(data, "a.non_existing")
	if err == nil {
		t.Errorf("Expected GetMapField to fail for non-existing key")
	}
}

type structB struct {
	I int16
	S string
}

type structA struct {
	Sub *structB
}

func TestLookupPath(t *testing.T) {
	s := &structA{
		Sub: &structB{
			I: int16(22),
			S: "rr",
		},
	}

	value, ok := LookupPath(*s, "Sub", reflect.Struct)
	if !ok {
		t.Error("Should find the struct")
	}

	if value.Interface().(structB).I != 22 {
		t.Error("Value expected not found")
	}

	value, ok = LookupPath(*s, "Sub.I", reflect.Int)
	if !ok {
		t.Error("Should find the struct")
	}
	if value.Int() != 22 {
		t.Error("Value expected not found")
	}

	value, ok = LookupPath(*s, "Sub.Z", reflect.Int)
	if ok {
		t.Error("Shouldn't find the struct")
	}

	value, ok = LookupPath(*s, "Sub.S", reflect.Int)
	if ok {
		t.Error("Shouldn't find the struct")
	}
}
