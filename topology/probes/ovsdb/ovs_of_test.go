/*
 * Copyright (C) 2017 Orange.
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

package ovsdb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/skydive-project/skydive/graffiti/logging"
	tp "github.com/skydive-project/skydive/topology/probes"
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

func newOfctlProbeTest() *OfctlProbe {
	return &OfctlProbe{
		Ctx:    tp.Context{Logger: logging.GetLogger()},
		Host:   "host",
		Bridge: "br",
	}
}

func TestParseEvent(t *testing.T) {
	probe := newOfctlProbeTest()

	var line = " event=ADDED table=21 cookie=32 dl_src=01:00:00:00:00:00/01:00:00:00:00:00\n"
	event, err := probe.parseEvent(line)
	if err != nil {
		t.Error("parseEvent should succeed")
	}
	if event.Bridge != "br" {
		t.Error("wrong bridge assigned")
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
	probe := newOfctlProbeTest()

	var line = " event=ADDED table=21 cookie=32 actions=resubmit(,1)\n"
	event, err := probe.parseEvent(line)
	if err != nil {
		t.Error("parseEvent should succeed")
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
	probe := newOfctlProbeTest()

	var line = " event=DELETED reason=delete table=66 cookie=0 ip,nw_dst=192.168.0.1\n"
	event, err := probe.parseEvent(line)
	if err != nil {
		t.Error("parseEvent should succeed")
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

func TestFillRawUUID(t *testing.T) {
	var (
		rule1   = RawRule{Cookie: 1, Table: 1, Filter: "a"}
		rule2   = RawRule{Cookie: 1, Table: 1, Filter: "b"}
		rule3   = RawRule{Cookie: 2, Table: 1, Filter: "a"}
		rule4   = RawRule{Cookie: 1, Table: 2, Filter: "a"}
		prefix1 = "XX"
		prefix2 = "YY"
	)
	fillRawUUID(&rule1, prefix1)
	var uuid = rule1.UUID
	if uuid == "" {
		t.Error("UUID not filled in")
	}
	fillRawUUID(&rule1, prefix2)
	if uuid == rule1.UUID {
		t.Error("Same UUID with distinct prefix")
	}
	fillRawUUID(&rule2, prefix1)
	if uuid == rule2.UUID {
		t.Error("Same UUID with distinct filter")
	}
	fillRawUUID(&rule3, prefix1)
	if uuid != rule3.UUID {
		t.Error("Distinct UUID with only distinct cookie")
	}
	fillRawUUID(&rule4, prefix1)
	if uuid == rule4.UUID {
		t.Error("Same UUID with distinct table")
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

func (r ExecuteForTest) ExecCommandPipe(ctx context.Context, com string, args ...string) (io.Reader, Waiter, error) {
	return strings.NewReader(r.Flow), r, nil
}

func (r ExecuteForTest) Wait() error {
	return nil
}

func TestMakeCommand(t *testing.T) {
	probe := &OfctlProbe{
		Handler: &OvsOfProbeHandler{
			host:           "host",
			bridgeOfProbes: make(map[string]*bridgeOfProbe),
			translation:    make(map[string]string),
			certificate:    "",
			privateKey:     "",
			ca:             "",
			sslOk:          false,
		},
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
	probe.Handler.certificate = "/cert"
	probe.Handler.privateKey = "/pk"
	probe.Handler.ca = "/ca"
	probe.Handler.sslOk = true
	r, err = probe.makeCommand(com, ssl, arg1, arg2)
	expected = []string{"c1", "c2", "ssl://sw:8000", "--certificate", "/cert", "--ca-cert", "/ca", "--private-key", "/pk", "a1", "a2"}
	if err != nil || !reflect.DeepEqual(r, expected) {
		t.Errorf("makeCommand: cert case: %v", r)
	}
}
func TestCompleteRule(t *testing.T) {
	old := executor
	defer func() { executor = old }()

	ctx := tp.Context{Logger: logging.GetLogger()}

	probe := &OfctlProbe{
		Ctx:    ctx,
		Host:   "host",
		Bridge: "br",
		Handler: &OvsOfProbeHandler{
			Ctx:            ctx,
			host:           "host",
			bridgeOfProbes: make(map[string]*bridgeOfProbe),
			translation:    make(map[string]string),
			certificate:    "",
			privateKey:     "",
			ca:             "",
			sslOk:          false,
		},
	}
	executor = ExecuteForTest{Results: []string{"NXST_FLOW reply (xid=0x4):\n cookie=0x20, duration=57227.249s, table=21, n_packets=0, n_bytes=0, idle_age=57227, priority=1,dl_src=01:00:00:00:00:00/01:00:00:00:00:00 actions=drop"}}
	var line = " event=ADDED table=21 cookie=32 dl_src=01:00:00:00:00:00/01:00:00:00:00:00\n"
	event, _ := probe.parseEvent(line)

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	err := probe.completeEvent(cancelCtx, &event)
	if err != nil {
		t.Error("completeRule: Should not err")
	}
	rule := event.Rules[0]
	filters, err1 := json.Marshal(rule.Filters)
	actions, err2 := json.Marshal(rule.Actions)
	if err1 != nil || err2 != nil {
		t.Error("cannot marshal filters or actions")
	}
	expectedAction := `[{"Function":"drop"}]`
	expectedFilter := `[{"Key":"dl_src","Value":"01:00:00:00:00:00","Mask":"01:00:00:00:00:00"}]`
	if string(filters) != expectedFilter || string(actions) != expectedAction {
		t.Errorf("completeRule: fails action=%s, filter=%s", actions, filters)
	}
}
