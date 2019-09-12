/*
 * Copyright (C) 2019 IBM, Inc.
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

package mod

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/contrib/exporters/core"
)

func areEqualJSON(buf1, buf2 []byte) (bool, error) {
	var o1 interface{}
	var o2 interface{}

	var err error
	err = json.Unmarshal(buf1, &o1)
	if err != nil {
		return false, err
	}
	err = json.Unmarshal(buf2, &o2)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(o1, o2), nil
}

func getEncoder(t *testing.T) core.Encoder {
	cfg := viper.New()
	encoder, err := NewEncode(cfg)
	if err != nil {
		t.Fatalf("NewEncode returned unexpected error: %v", err)
	}
	return encoder.(core.Encoder)
}

func Test_Encode_empty_flows_array(t *testing.T) {
	in := make([]interface{}, 0)
	result, err := getEncoder(t).Encode(in)
	if err != nil {
		t.Fatalf("Encode returned unexpected error: %v", err)
	}
	expected := []byte(
		`{
			"data": []
		}`)
	equal, err := areEqualJSON(expected, result)
	if err != nil {
		t.Fatalf("Error parsing JSON: %v", err)
	}
	if !equal {
		t.Fatalf("Objects not identical")
	}
}

func Test_Encode_flows_array_with_objects(t *testing.T) {
	in := []interface{}{
		&SecurityAdvisorFlow{
			Status: "STARTED",
			Start:  1234,
		},
		&SecurityAdvisorFlow{
			Status: "ENDED",
			Last:   5678,
		},
	}
	result, err := getEncoder(t).Encode(in)
	if err != nil {
		t.Fatalf("Encode returned unexpected error: %v", err)
	}
	expected := []byte(
		`{
			"data": [
				{
					"Status": "STARTED",
					"Start": 1234,
					"Last": 0,
					"UpdateCount": 0
				},
				{
					"Status": "ENDED",
					"Start": 0,
					"Last": 5678,
					"UpdateCount": 0
				}
			]
		}`)
	equal, err := areEqualJSON(expected, result)
	if err != nil {
		t.Fatalf("Error parsing JSON: %v", err)
	}
	if !equal {
		t.Fatalf("Objects not identical")
	}
}
