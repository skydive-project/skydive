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

package topology

import (
	"reflect"
	"testing"
)

func TestSlice(t *testing.T) {
	m := &InterfaceMetric{
		RxBytes:   100,
		RxPackets: 100,
		TxBytes:   100,
		TxPackets: 100,
		Last:      100,
	}

	s1, s2 := m.Split(25)

	expected := &InterfaceMetric{
		RxBytes:   25,
		RxPackets: 25,
		TxBytes:   25,
		TxPackets: 25,
		Start:     0,
		Last:      25,
	}

	if !reflect.DeepEqual(expected, s1) {
		t.Errorf("Slice 1 error, expected %+v, got %+v", expected, s1)
	}

	expected = &InterfaceMetric{
		RxBytes:   75,
		RxPackets: 75,
		TxBytes:   75,
		TxPackets: 75,
		Start:     25,
		Last:      100,
	}

	if !reflect.DeepEqual(expected, s2) {
		t.Errorf("Slice 2 error, expected %+v, got %+v", expected, s2)
	}
}
