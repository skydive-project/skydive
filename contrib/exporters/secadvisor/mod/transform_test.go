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
	"fmt"
	"io/ioutil"
	"runtime"
	"testing"
	"time"

	cache "github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
)

const testConfig = `---
pipeline:
  transform:
    type: sa
    extend:
      - CA_Name=G.V().Has('RoutingTables.Src','{{.Network.A}}').Values('Host')
      - CB_Name=G.V().Has('RoutingTables.Src','{{.Network.B}}').Values('Host')
`

func initConfig(conf string) error {
	f, _ := ioutil.TempFile("", "extend_gremlin_test")
	b := []byte(conf)
	f.Write(b)
	f.Close()
	err := config.InitConfig("file", []string{f.Name()})
	return err
}

func (e *extendGremlin) populateExtendGremlin(f *flow.Flow, transformer *securityAdvisorFlowTransformer) {
	substExprA := "G.V().Has('RoutingTables.Src','" + f.Network.A + "').Values('Host')"
	substExprB := "G.V().Has('RoutingTables.Src','" + f.Network.B + "').Values('Host')"
	AName, _ := transformer.resolver.IPToName(f.Network.A, "")
	BName, _ := transformer.resolver.IPToName(f.Network.B, "")
	// place substitue expressions into the cache
	e.gremlinExprCache.Set(substExprA, AName, cache.DefaultExpiration)
	e.gremlinExprCache.Set(substExprB, BName, cache.DefaultExpiration)
}

type fakeResolver struct {
}

func (c *fakeResolver) IPToName(ipString, nodeTID string) (string, error) {
	switch ipString {
	case "192.168.0.5":
		return "fake_container_one", nil
	case "173.194.40.147":
		return "fake_container_two", nil
	default:
		return "", fmt.Errorf("fake graph client")
	}
}

func (c *fakeResolver) TIDToType(nodeTID string) (string, error) {
	return "fake_node_type", nil
}

func getTestTransformer() *securityAdvisorFlowTransformer {
	initConfig(testConfig)
	cfg := config.GetConfig().Viper
	newExtend := NewExtendGremlin(cfg)
	return &securityAdvisorFlowTransformer{
		flowUpdateCountCache: cache.New(10*time.Minute, 10*time.Minute),
		resolver:             &fakeResolver{},
		excludeStartedFlows:  false,
		extendGremlin:        newExtend,
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

func assertEqualExtend(t *testing.T, expected, actual interface{}, suffix string) {
	actual2 := actual.(map[string]interface{})
	if expected != actual2[suffix] {
		msg := "Equal assertion failed"
		_, file, no, ok := runtime.Caller(1)
		if ok {
			msg += fmt.Sprintf(" on %s:%d", file, no)
		}
		t.Fatalf("%s: (expected: %v, actual: %v)", msg, expected, actual2[suffix])
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

func getFlow() *flow.Flow {
	t, _ := time.Parse(time.RFC3339, "2019-01-01T10:20:30Z")
	start := common.UnixMillis(t)
	return &flow.Flow{
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

func TestTransformBasicFlow(t *testing.T) {
	transformer := getTestTransformer()
	f := getFlow()
	transformer.extendGremlin.populateExtendGremlin(f, transformer)
	secAdvFlow := transformer.Transform(f).(*SecurityAdvisorFlow)
	assertEqual(t, version, secAdvFlow.Version)
	assertEqualInt64(t, 0, secAdvFlow.UpdateCount)
	assertEqual(t, "STARTED", secAdvFlow.Status)
	assertEqualInt64(t, 1546338030000, secAdvFlow.Start)
	assertEqualInt64(t, 1546338030000, secAdvFlow.Last)
	// Test container and node enrichment
	assertEqual(t, "fake_node_type", secAdvFlow.NodeType)
	assertEqual(t, "fake_container_one", secAdvFlow.Network.AName)
	assertEqual(t, "fake_container_two", secAdvFlow.Network.BName)
	// Test that transport layer ports are encoded as strings
	assertEqual(t, "47838", secAdvFlow.Transport.A)
	assertEqual(t, "80", secAdvFlow.Transport.B)
}

func TestTransformUpdateCountIncreases(t *testing.T) {
	transformer := getTestTransformer()
	f := getFlow()
	transformer.extendGremlin.populateExtendGremlin(f, transformer)
	for i := int64(0); i < 10; i++ {
		secAdvFlow := transformer.Transform(f).(*SecurityAdvisorFlow)
		assertEqualInt64(t, i, secAdvFlow.UpdateCount)
	}
}

func TestTransformStatusUpdates(t *testing.T) {
	transformer := getTestTransformer()
	f := getFlow()
	transformer.extendGremlin.populateExtendGremlin(f, transformer)
	secAdvFlow := transformer.Transform(f).(*SecurityAdvisorFlow)
	assertEqual(t, "STARTED", secAdvFlow.Status)
	secAdvFlow = transformer.Transform(f).(*SecurityAdvisorFlow)
	assertEqual(t, "UPDATED", secAdvFlow.Status)
	f.FinishType = flow.FlowFinishType_TCP_FIN
	secAdvFlow = transformer.Transform(f).(*SecurityAdvisorFlow)
	assertEqual(t, "ENDED", secAdvFlow.Status)
}

func TestExtendGremlin(t *testing.T) {
	transformer := getTestTransformer()
	f := getFlow()
	transformer.extendGremlin.populateExtendGremlin(f, transformer)
	secAdvFlow := transformer.Transform(f).(*SecurityAdvisorFlow)

	// verify that extension fields were properly created
	assertEqualExtend(t, "fake_container_one", secAdvFlow.Extend, "CA_Name")
	assertEqualExtend(t, "fake_container_two", secAdvFlow.Extend, "CB_Name")
}
