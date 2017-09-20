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

package traversal

import (
	"testing"
	"strings"
	"reflect"
)


func newScanner(query string) *GremlinTraversalScanner {
	return NewGremlinTraversalScanner(strings.NewReader(query), nil)
}

func scanForBracketsInvalid(query string, t *testing.T) {
	_, err := newScanner(query).ScanBraces()
	if err == nil {
		t.Fatalf("Expected error, got some result for input '%s'", query)
	}
}

func scanForBrackets(query string, expected []string, t *testing.T) {
	actual, err := newScanner(query).ScanBraces()
	if err != nil {
		t.Fatalf("error while scanning brackets in '%s': %s", query, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Expected %s (len=%v), actual: %s (len=%v)", expected, len(expected), actual, len(actual))
	}
}

func TestScanBracketsEmpty(t *testing.T) {
	scanForBrackets("()", []string{}, t)
}

func TestScanBracketsSimple(t *testing.T) {
	scanForBrackets("(x)", []string{"x"}, t)
	scanForBrackets("(((x)))", []string{"((x))"}, t) // strip 1 level from many
	scanForBrackets("(  x  )", []string{"x"}, t) // trim
	scanForBrackets("(x,y,z)", []string{"x", "y", "z"}, t) // comma
	scanForBrackets("(x,y,(z))", []string{"x", "y", "(z)"}, t)
	scanForBrackets("(x,(y, z))", []string{"x", "(y, z)"}, t)
}

func TestScanBracketsInString(t *testing.T) {
	scanForBrackets("( has('x(y)z') )", []string{"has('x(y)z')"}, t)
	scanForBrackets("( has('bracket in property value', '(') )", []string{"has('bracket in property value', '(')"}, t)
	scanForBrackets("( has('bracket in property value', '\\'(') )", []string{"has('bracket in property value', '\\'(')"}, t)
}

func TestScanBracketsBroken(t *testing.T) {
	scanForBracketsInvalid("x()", t)
	scanForBracketsInvalid(",()", t)
	scanForBracketsInvalid(")(", t)
	scanForBracketsInvalid("(", t)
	scanForBracketsInvalid(")", t)
}

// check only 1st scanned and 2nd untouched
func TestScanBracketsSequence(t *testing.T) {
	scanner := newScanner("(x)(y)xxx")

	for _, expected := range []string{"x", "y"} {
		actual, err := scanner.ScanBraces()
		if err != nil {
			t.Fatalf("error while scanning brackets: %s", err)
		}
		if !reflect.DeepEqual(actual, []string{expected}) {
			t.Fatalf("Expected %s, actual %s", expected, actual)
		}
	}
}
