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

package main

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	cache "github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/contrib/pipelines/core"
	"github.com/skydive-project/skydive/flow"
)

type fakeSecurityAdvisorGraphClient struct {
}

func (c *fakeSecurityAdvisorGraphClient) getContainerName(ipString, nodeTID string) (string, error) {
	switch ipString {
	case "192.168.0.5":
		return "fake_container_one", nil
	case "173.194.40.147":
		return "fake_container_two", nil
	default:
		return "", fmt.Errorf("fake graph client")
	}
}

func (c *fakeSecurityAdvisorGraphClient) getNodeType(nodeTID string) (string, error) {
	return "fake_node_type", nil
}

func getTestTransformer() core.Transformer {
	return &securityAdvisorFlowTransformer{
		flowUpdateCountCache: cache.New(10*time.Minute, 10*time.Minute),
		graphClient:          &fakeSecurityAdvisorGraphClient{},
		containerNameCache:   cache.New(5*time.Minute, 10*time.Minute),
		nodeType:             cache.New(5*time.Minute, 10*time.Minute),
		excludeStartedFlows:  false,
	}
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

func getFlow() flow.Flow {
	t, _ := time.Parse(time.RFC3339, "2019-01-01T10:20:30Z")
	start := common.UnixMillis(t)
	return flow.Flow{
		UUID:        "66724f5d-718f-47a2-93a7-c807cd54241e",
		LayersPath:  "Ethernet/IPv4/TCP",
		Application: "TCP",
		Link: &flow.FlowLayer{
			Protocol: flow.FlowProtocol_ETHERNET,
			A:        "fa:16:3e:29:e0:82",
			B:        "fa:16:3e:96:06:e8",
		},
		Network: &flow.FlowLayer{
			Protocol: flow.FlowProtocol_IPV4,
			A:        "192.168.0.5",
			B:        "173.194.40.147",
		},
		Transport: &flow.TransportLayer{
			Protocol: flow.FlowProtocol_TCP,
			A:        47838,
			B:        80,
		},
		Metric: &flow.FlowMetric{
			ABPackets: 6,
			ABBytes:   516,
			BAPackets: 4,
			BABytes:   760,
		},
		Start:   start,
		Last:    start,
		NodeTID: "probe-tid",
	}
}

func Test_Transform_basic_flow(t *testing.T) {
	transformer := getTestTransformer()
	f := getFlow()
	secAdvFlow := transformer.Transform(&f).(*SecurityAdvisorFlow)
	assertEqual(t, version, secAdvFlow.Version)
	assertEqualInt64(t, 0, secAdvFlow.UpdateCount)
	assertEqual(t, "STARTED", secAdvFlow.Status)
	assertEqualInt64(t, 1546338030000, secAdvFlow.Start)
	assertEqualInt64(t, 1546338030000, secAdvFlow.Last)
	// Test container and node enrichment
	assertEqual(t, "fake_node_type", secAdvFlow.NodeType)
	assertEqual(t, "0_0_fake_container_one_0", secAdvFlow.Network.AName)
	assertEqual(t, "0_0_fake_container_two_0", secAdvFlow.Network.BName)
	// Test that transport layer ports are encoded as strings
	assertEqual(t, "47838", secAdvFlow.Transport.A)
	assertEqual(t, "80", secAdvFlow.Transport.B)
}

func Test_Transform_UpdateCount_increses(t *testing.T) {
	transformer := getTestTransformer()
	f := getFlow()
	for i := int64(0); i < 10; i++ {
		secAdvFlow := transformer.Transform(&f).(*SecurityAdvisorFlow)
		assertEqualInt64(t, i, secAdvFlow.UpdateCount)
	}
}

func Test_Transform_Status_updates(t *testing.T) {
	transformer := getTestTransformer()
	f := getFlow()
	secAdvFlow0 := transformer.Transform(&f).(*SecurityAdvisorFlow)
	assertEqual(t, "STARTED", secAdvFlow0.Status)
	secAdvFlow1 := transformer.Transform(&f).(*SecurityAdvisorFlow)
	assertEqual(t, "UPDATED", secAdvFlow1.Status)
	f.FinishType = flow.FlowFinishType_TCP_FIN
	secAdvFlow2 := transformer.Transform(&f).(*SecurityAdvisorFlow)
	assertEqual(t, "ENDED", secAdvFlow2.Status)
}
