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

// Iterator describes a int64 iterator
type Iterator struct {
	at, from, to int64
}

// Done returns true when the Iterator is at the end or uninitialized properly
func (it *Iterator) Done() bool {
	return it.to != -1 && it.at >= it.to
}

// Next returns true if we can continue to iterate
func (it *Iterator) Next() bool {
	it.at++
	return it.at-1 >= it.from
}

// NewIterator creates a new iterator based on (at, from, to) parameters
func NewIterator(values ...int64) (it *Iterator) {
	it = &Iterator{to: -1}
	if len(values) > 0 {
		it.at = values[0]
	}
	if len(values) > 1 {
		it.from = values[1]
	}
	if len(values) > 2 {
		it.to = values[2]
	}
	return
}
