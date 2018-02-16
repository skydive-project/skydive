/*
 * Copyright (C) 2017 Orange.
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

package ovsdb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestProtectCommas(t *testing.T) {
	var inp = "abc,def(g(h),ij,kl(mn,op)),qr,st(u(v,w)),xy"
	var exp = "abc,def(g(h);ij;kl(mn;op)),qr,st(u(v;w)),xy"
	var out = protectCommas(inp)
	if out != exp {
		t.Errorf("ProtectComma failure %s != %s", exp, out)
	}
	if protectCommas("") != "" {
		t.Error("ProtectComma fails on empty string")
	}
}

func TestParseEvent(t *testing.T) {
	var line = " event=ADDED table=21 cookie=32 dl_src=01:00:00:00:00:00/01:00:00:00:00:00\n"
	event, err := parseEvent(line, "br", "host-br-")
	if err != nil {
		t.Error("parseEvent should succeed")
	}
	if event.Bridge != "br" {
		t.Error("wrong bridge assigned")
	}
	if event.RawRule.Actions != "" {
		t.Error("No action here")
	}
	if event.RawRule.Filter != "dl_src=01:00:00:00:00:00/01:00:00:00:00:00" {
		t.Errorf("Bad filter: %s", event.RawRule.Filter)
	}
	if event.RawRule.Cookie != 32 {
		t.Errorf("Bad cookie")
	}
	if event.RawRule.Table != 21 {
		t.Errorf("Bad table: %d", event.RawRule.Table)
	}
}

func TestParseEventWithAction(t *testing.T) {
	var line = " event=ADDED table=21 cookie=32 actions=resubmit(,1)\n"
	event, err := parseEvent(line, "br", "host-br-")
	if err != nil {
		t.Error("parseEvent should succeed")
	}
	if event.RawRule.Actions != "resubmit(;1)" {
		t.Errorf("Bad action: %s", event.RawRule.Actions)
	}
	if event.RawRule.Filter != "" {
		t.Errorf("No filter here %s", event.RawRule.Filter)
	}
	if event.RawRule.Cookie != 32 {
		t.Errorf("Bad cookie")
	}
	if event.RawRule.Table != 21 {
		t.Errorf("Bad table: %d", event.RawRule.Table)
	}
}

func TestParseEventRemove(t *testing.T) {
	var line = " event=DELETED reason=delete table=66 cookie=0 ip,nw_dst=192.168.0.1\n"
	event, err := parseEvent(line, "br", "host-br-")
	if err != nil {
		t.Error("parseEvent should succeed")
	}
	if event.RawRule.Actions != "" {
		t.Errorf("No action here")
	}
	if event.RawRule.Filter != "ip,nw_dst=192.168.0.1" {
		t.Errorf("Bad filter %s", event.RawRule.Filter)
	}
	if event.RawRule.Cookie != 0 {
		t.Errorf("Bad cookie")
	}
	if event.RawRule.Table != 66 {
		t.Errorf("Bad table: %d", event.RawRule.Table)
	}
}

func TestFillUUID(t *testing.T) {
	var (
		rule1   = Rule{Cookie: 1, Table: 1, Filter: "a"}
		rule2   = Rule{Cookie: 1, Table: 1, Filter: "b"}
		rule3   = Rule{Cookie: 2, Table: 1, Filter: "a"}
		rule4   = Rule{Cookie: 1, Table: 2, Filter: "a"}
		prefix1 = "XX"
		prefix2 = "YY"
	)
	fillUUID(&rule1, prefix1)
	var uuid = rule1.UUID
	if uuid == "" {
		t.Error("UUID not filled in")
	}
	fillUUID(&rule1, prefix2)
	if uuid == rule1.UUID {
		t.Error("Same UUID with distinct prefix")
	}
	fillUUID(&rule2, prefix1)
	if uuid == rule2.UUID {
		t.Error("Same UUID with distinct filter")
	}
	fillUUID(&rule3, prefix1)
	if uuid == rule3.UUID {
		t.Error("Same UUID with distinct cookie")
	}
	fillUUID(&rule4, prefix1)
	if uuid == rule4.UUID {
		t.Error("Same UUID with distinct table")
	}
}

func TestRule1(t *testing.T) {
	var line1 = " cookie=0x10, duration=57227.249s, table=11, n_packets=0, n_bytes=0, idle_age=57227, dl_src=01:00:00:00:00:00/01:00:00:00:00:00 actions=drop"
	rule1, err1 := parseRule(line1)
	if err1 != nil {
		t.Error("unexpected error")
	}
	if rule1.Cookie != 16 || rule1.Table != 11 || rule1.Filter != "dl_src=01:00:00:00:00:00/01:00:00:00:00:00" || rule1.Actions != "drop" {
		out, err := json.Marshal(&rule1)
		if err != nil {
			t.Error("bad marshall")
		}
		t.Errorf("Bad rule 1 %s", out)
	}
}

func TestRule2(t *testing.T) {
	var line2 = " cookie=0x20, duration=57214.349s, table=22, n_packets=0, n_bytes=0, idle_age=57293, priority=10 actions=resubmit(,1)"
	rule2, err2 := parseRule(line2)
	if err2 != nil {
		t.Error("unexpected error")
	}
	if rule2.Cookie != 32 || rule2.Table != 22 || rule2.Filter != "priority=10" || rule2.Actions != "resubmit(;1)" {
		out, err := json.Marshal(&rule2)
		if err != nil {
			t.Error("bad marshall")
		}
		t.Errorf("Bad rule 2 %s", out)
	}

}

func TestRule3(t *testing.T) {
	var line = "aaaaaaaa"
	_, err3 := parseRule(line)
	if err3 == nil {
		t.Error("Failure was expected")
	}
}

type ExecuteForTest struct {
	Results []string
	Flow    string
	count   int
}

func (r ExecuteForTest) ExecCommand(com string, args ...string) ([]byte, error) {
	if r.count > len(r.Results) {
		return nil, errors.New("Too many requests")
	}
	result := r.Results[r.count]
	r.count++
	return []byte(result), nil
}

func (r ExecuteForTest) ExecCommandPipe(ctx context.Context, com string, args ...string) (io.Reader, error) {
	return strings.NewReader(r.Flow), nil
}

func TestMakeCommand(t *testing.T) {
	probe := &OvsOfProbe{
		Host:         "host",
		Graph:        nil,
		Root:         nil,
		BridgeProbes: make(map[string]*BridgeOfProbe),
		Translation:  make(map[string]string),
		Certificate:  "",
		PrivateKey:   "",
		CA:           "",
		sslOk:        false,
	}
	com := []string{"c1", "c2"}
	br := "br"
	ssl := "ssl://sw:8000"
	arg1, arg2 := "a1", "a2"
	r, err := probe.makeCommand(com, br, arg1, arg2)
	expected := []string{"c1", "c2", "br", "a1", "a2"}
	if err != nil || !reflect.DeepEqual(r, expected) {
		t.Errorf("makeCommand: simple case: %v", r)
	}
	_, err = probe.makeCommand(com, ssl, arg1, arg2)
	if err == nil {
		t.Error("ssl case: should fail")
	}
	probe.Certificate = "/cert"
	probe.PrivateKey = "/pk"
	probe.CA = "/ca"
	probe.sslOk = true
	r, err = probe.makeCommand(com, ssl, arg1, arg2)
	expected = []string{"c1", "c2", "ssl://sw:8000", "--certificate", "/cert", "--ca-cert", "/ca", "--private-key", "/pk", "a1", "a2"}
	if err != nil || !reflect.DeepEqual(r, expected) {
		t.Errorf("makeCommand: cert case: %v", r)
	}
}
func TestCompleteRule(t *testing.T) {
	old := executor
	defer func() { executor = old }()
	probe := &OvsOfProbe{
		Host:         "host",
		Graph:        nil,
		Root:         nil,
		BridgeProbes: make(map[string]*BridgeOfProbe),
		Translation:  make(map[string]string),
		Certificate:  "",
		PrivateKey:   "",
		CA:           "",
		sslOk:        false,
	}
	executor = ExecuteForTest{Results: []string{"NXST_FLOW reply (xid=0x4):\n cookie=0x20, duration=57227.249s, table=21, n_packets=0, n_bytes=0, idle_age=57227, priority=1,dl_src=01:00:00:00:00:00/01:00:00:00:00:00 actions=drop"}}
	var line = " event=ADDED table=21 cookie=32 dl_src=01:00:00:00:00:00/01:00:00:00:00:00\n"
	event, _ := parseEvent(line, "br", "host-br-")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := completeEvent(ctx, probe, &event, "host-br-")
	if err != nil {
		t.Error("completeRule: Should not err")
	}
	rule := event.Rules[0]
	if rule.Filter != "priority=1,dl_src=01:00:00:00:00:00/01:00:00:00:00:00" || rule.Actions != "drop" {
		t.Errorf("completeRule: fails action=%s, filter=%s", rule.Actions, rule.Filter)
	}
}
