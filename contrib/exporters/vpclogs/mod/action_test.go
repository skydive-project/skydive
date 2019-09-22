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
	"fmt"
	"io/ioutil"
	"runtime"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
)

func myPrint(a interface{}) {
	out, _ := json.Marshal(a)
	fmt.Println(string(out))
}

const testConfig = `---
pipeline:
  action:
    type: aws
    tid1: fake_tid_1
    tid2: fake_tid_2
`

func initConfig(conf string) error {
	f, _ := ioutil.TempFile("", "action_test")
	b := []byte(conf)
	f.Write(b)
	f.Close()
	err := config.InitConfig("file", []string{f.Name()})
	return err
}

func assertEqual(t *testing.T, expected, actual interface{}) {
	if expected != actual {
		msg := "Equal assertion failed"
		_, file, no, ok := runtime.Caller(1)
		if ok {
			msg += fmt.Sprintf(" on %s:%d", file, no)
		}
		t.Fatalf("%s: (expected: %v, actual: %v)", msg, expected, actual)
	}
}

func assertEqualInt64(t *testing.T, expected, actual int64) {
	if expected != actual {
		msg := "Equal assertion failed"
		_, file, no, ok := runtime.Caller(1)
		if ok {
			msg += fmt.Sprintf(" on %s:%d", file, no)
		}
		t.Fatalf("%s: (expected: %v, actual: %v)", msg, expected, actual)
	}
}

type fakeActioner interface {
	// Action determines the Accept/Reject action and combines common flows
	Action(f interface{}) interface{}
}

func getBasicFlow() *VpclogsFlow {
	t, _ := time.Parse(time.RFC3339, "2019-01-01T10:20:30Z")
	start := common.UnixMillis(t)
	return &VpclogsFlow{
		UUID:       "66724f5d-718f-47a2-93a7-c807cd54241e",
		LayersPath: "Ethernet/IPv4/TCP",
		Network: &VpclogsFlowLayer{
			Protocol: "IPv4",
			A:        "192.168.0.5",
			B:        "173.194.40.147",
		},
		Transport: &VpclogsFlowLayer{
			Protocol: "TCP",
			A:        "47838",
			B:        "80",
		},
		Metric: &flow.FlowMetric{
			ABPackets: 6,
			ABBytes:   516,
			BAPackets: 4,
			BABytes:   760,
		},
		LastUpdateMetric: &flow.FlowMetric{
			ABPackets: 2,
			ABBytes:   200,
			BAPackets: 3,
			BABytes:   300,
		},
		Start: start,
		Last:  start,
	}
}

func getFlowArrayLive(iter int) interface{} {
	iteration := int64(iter)
	flow1 := getBasicFlow()
	flow1.TrackingID = "flow1-id"
	flow1.Metric.ABPackets += iteration * 2
	flow1.Metric.ABBytes += iteration * 200
	flow1.Metric.BAPackets += iteration * 3
	flow1.Metric.BABytes += iteration * 300
	flow1.Last = flow1.Start + iteration*300
	flow1.UUID = "UUID1"
	flow1.TID = "fake_tid_1"

	flow2 := getBasicFlow()
	flow2.TrackingID = "flow2-id"
	flow2.Metric.ABPackets += iteration * 2
	flow2.Metric.ABBytes += iteration * 200
	flow2.Metric.BAPackets += iteration * 3
	flow2.Metric.BABytes += iteration * 300
	flow2.Last = flow2.Start + iteration*300
	flow2.UUID = "UUID2"
	flow2.TID = "fake_tid_1"

	flow3 := getBasicFlow()
	flow3.TrackingID = "flow3-id"
	flow3.Metric.ABPackets += iteration * 2
	flow3.Metric.ABBytes += iteration * 200
	flow3.Metric.BAPackets += iteration * 3
	flow3.Metric.BABytes += iteration * 300
	flow3.Last = flow3.Start + iteration*300
	flow3.UUID = "UUID3"
	flow3.TID = "fake_tid_2"

	flow4 := getBasicFlow()
	flow4.TrackingID = "flow2-id"
	flow4.Metric.ABPackets += iteration*2 - 1
	flow4.Metric.ABBytes += iteration*200 - 100
	flow4.Metric.BAPackets += iteration*3 + 1
	flow4.Metric.BABytes += iteration*300 + 100
	flow4.Last = flow4.Start + iteration*300 - 20
	flow4.UUID = "UUID4"
	flow4.TID = "fake_tid_2"

	flow5 := getBasicFlow()
	flow5.TrackingID = "flow1-id"
	flow5.Metric.ABPackets += iteration*2 + 1
	flow5.Metric.ABBytes += iteration*200 + 100
	flow5.Metric.BAPackets += iteration*3 - 1
	flow5.Metric.BABytes += iteration*300 - 100
	flow5.Last = flow5.Start + iteration*300 + 20
	flow5.UUID = "UUID5"
	flow5.TID = "fake_tid_2"

	var ff []interface{}
	ff = append(ff, flow1)
	ff = append(ff, flow2)
	ff = append(ff, flow3)
	ff = append(ff, flow4)
	ff = append(ff, flow5)
	return ff
}

func checkStats(t *testing.T, fArray1 interface{}, fArray2 interface{}) {
	f1 := fArray1.([]interface{})
	f2 := fArray2.([]interface{})
	var i, j int
	for _, k := range f2 {
		f := k.(*VpclogsFlow)
		if f.TrackingID == "flow3-id" {
			assertEqual(t, "REJECT", f.Action)
			continue
		} else if f.TrackingID == "flow1-id" {
			i = 0
			j = 4
		} else if f.TrackingID == "flow2-id" {
			i = 1
			j = 3
		}
		f1a := f1[i].(*VpclogsFlow)
		f1b := f1[j].(*VpclogsFlow)
		assertEqualInt64(t, common.MinInt64(f1a.Metric.ABPackets, f1b.Metric.ABPackets), f.Metric.ABPackets)
		assertEqualInt64(t, common.MinInt64(f1a.Metric.BAPackets, f1b.Metric.BAPackets), f.Metric.BAPackets)
		assertEqualInt64(t, common.MinInt64(f1a.Metric.ABBytes, f1b.Metric.ABBytes), f.Metric.ABBytes)
		assertEqualInt64(t, common.MinInt64(f1a.Metric.BABytes, f1b.Metric.BABytes), f.Metric.BABytes)
		assertEqual(t, "ACCEPT", f.Action)
	}
}

func TestActionAcceptMetric(t *testing.T) {
	initConfig(testConfig)
	cfg := config.GetConfig().Viper
	action, _ := NewAction(cfg)
	action1 := action.(fakeActioner)
	for i := 0; i < 4; i++ {
		f1 := getFlowArrayLive(i)
		f2 := action1.Action(f1)
		checkStats(t, f1, f2)
		// we expect flow3-id to not show up on the first iteration
		// we expect flow3-id to show up as Rejected from second iteration and onwards
		f2b := f2.([]interface{})
		if i == 0 {
			assertEqual(t, 2, len(f2b))
		} else {
			assertEqual(t, 3, len(f2b))
		}
	}
}
