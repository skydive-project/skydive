// +build linux

/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package hardware

import (
	"reflect"
	"testing"
)

func TestIsolatedCPU(t *testing.T) {
	ic := "3,4,7-9,10"

	expected := []int64{3, 4, 7, 8, 9, 10}

	list, err := parseIsolatedCPUs(ic)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(list, expected) {
		t.Fatalf("Parsing of isolated cpu failed, expected: %v, got: %v", expected, list)
	}

	_, err = parseIsolatedCPUs("3,4,9-7,10")
	if err == nil {
		t.Fatal("Parsing of isolated cpu should return an error")
	}

	_, err = parseIsolatedCPUs("3,4,7-,10")
	if err == nil {
		t.Fatal("Parsing of isolated cpu should return an error")
	}
}
