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

package getter

import "errors"

// ErrFieldNotFound error field not found
var ErrFieldNotFound = errors.New("Field not found")

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
