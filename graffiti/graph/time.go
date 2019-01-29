/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package graph

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/skydive-project/skydive/common"
)

// Time describes time type used in the graph
type Time time.Time

// Unix returns the time in millisecond
func (t Time) Unix() int64 {
	return common.UnixMillis(time.Time(t))
}

// MarshalJSON custom marshalling function
func (t *Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Unix())
}

// UnmarshalJSON custom unmarshalling function
func (t *Time) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	ms, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return err
	}
	*t = Time(time.Unix(0, ms*int64(time.Millisecond)))

	return nil
}

// IsZero returns is empty or not
func (t Time) IsZero() bool {
	return time.Time(t).IsZero()
}

// TimeUTC creates a Time with now UTC
func TimeUTC() Time {
	return Time(time.Now().UTC())
}

// Unix returns Time for given sec, nsec
func Unix(sec int64, nsec int64) Time {
	return Time(time.Unix(sec, nsec))
}
