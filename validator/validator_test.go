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

package validator

import "testing"

type bpfTest struct {
	BPFFilter string `valid:"isBPFFilter"`
}

func TestBPFFilter(t *testing.T) {
	b := bpfTest{BPFFilter: "port 22"}
	if err := Validate("", b); err != nil {
		t.Errorf("Should not return an error: %s", err.Error())
	}

	b = bpfTest{BPFFilter: "err_port 22"}
	if err := Validate("", b); err == nil {
		t.Error("Should return an error")
	}
}

type gremlinTest struct {
	GremlinQuery string `valid:"isGremlinExpr"`
}

func TestGremlin(t *testing.T) {
	g := gremlinTest{GremlinQuery: "G.V().Has('Name', 'test')"}
	if err := Validate("", g); err != nil {
		t.Errorf("Should not return an error: %s", err.Error())
	}

	g = gremlinTest{GremlinQuery: "G.V(.Has('Name', 'test')"}
	if err := Validate("", g); err == nil {
		t.Error("Should return an error")
	}
}

type ipTest struct {
	IP string `valid:"isIP"`
}

func TestIP(t *testing.T) {
	i := ipTest{IP: "192.255.0.1"}
	if err := Validate("", i); err != nil {
		t.Errorf("Should not return an error: %s", err.Error())
	}

	i = ipTest{IP: "192.257.0.1"}
	if err := Validate("", i); err == nil {
		t.Error("Should return an error")
	}
}
