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

package subscriber

import (
	"errors"
	"testing"
	"time"

	"github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
)

type fakeGraphClient struct {
	containerNameError     error
	nodeTypeError          error
	nodeTypeCallCount      int
	containerNameCallCount int
}

func (c *fakeGraphClient) getContainerName(ipString, nodeTID string) (string, error) {
	c.containerNameCallCount++
	return "container" + ipString, c.containerNameError
}

func (c *fakeGraphClient) getNodeType(nodeTID string) (string, error) {
	c.nodeTypeCallCount++
	return "type" + nodeTID, c.nodeTypeError
}

func getTestSecurityAdvisorTransformer() (*securityAdvisorFlowTransformer, *fakeGraphClient) {
	fakeClient := &fakeGraphClient{}
	return &securityAdvisorFlowTransformer{
		flowUpdateCount: cache.New(10*time.Minute, 10*time.Minute),
		graphClient:     fakeClient,
		containerName:   cache.New(5*time.Minute, 10*time.Minute),
		nodeType:        cache.New(5*time.Minute, 10*time.Minute),
	}, fakeClient
}

func testTransform(ft *securityAdvisorFlowTransformer, f *flow.Flow) *SecurityAdvisorFlow {
	res := ft.Transform(f)
	if res == nil {
		return nil
	}
	return res.(*SecurityAdvisorFlow)
}

func Test_Transform_UpdateCount(t *testing.T) {
	ft, _ := getTestSecurityAdvisorTransformer()
	f1 := &flow.Flow{UUID: "1", FinishType: flow.FlowFinishType_TCP_FIN}

	assertEqual(t, int64(0), testTransform(ft, f1).UpdateCount)
	f1.FinishType = flow.FlowFinishType_NOT_FINISHED
	assertEqual(t, int64(1), testTransform(ft, f1).UpdateCount)

	f2 := &flow.Flow{UUID: "2", FinishType: flow.FlowFinishType_TCP_FIN}
	assertEqual(t, int64(0), testTransform(ft, f2).UpdateCount)
	f2.FinishType = flow.FlowFinishType_NOT_FINISHED
	assertEqual(t, int64(2), testTransform(ft, f1).UpdateCount)

	f1.FinishType = flow.FlowFinishType_TIMEOUT
	assertEqual(t, int64(3), testTransform(ft, f1).UpdateCount)
	assertEqual(t, int64(0), testTransform(ft, f1).UpdateCount)

	f2.FinishType = flow.FlowFinishType_TCP_FIN
	assertEqual(t, int64(1), testTransform(ft, f2).UpdateCount)
	assertEqual(t, int64(2), testTransform(ft, f2).UpdateCount)
}

func Test_Transform_Status(t *testing.T) {
	ft, _ := getTestSecurityAdvisorTransformer()
	f := &flow.Flow{}

	assertEqual(t, "STARTED", testTransform(ft, f).Status)
	assertEqual(t, "UPDATED", testTransform(ft, f).Status)

	f.FinishType = flow.FlowFinishType_TCP_FIN
	assertEqual(t, "ENDED", testTransform(ft, f).Status)

	f.FinishType = flow.FlowFinishType_NOT_FINISHED
	assertEqual(t, "UPDATED", testTransform(ft, f).Status)
	f.FinishType = flow.FlowFinishType_TCP_RST
	assertEqual(t, "ENDED", testTransform(ft, f).Status)

	f.FinishType = flow.FlowFinishType_NOT_FINISHED
	assertEqual(t, "UPDATED", testTransform(ft, f).Status)
	f.FinishType = flow.FlowFinishType_TIMEOUT
	assertEqual(t, "ENDED", testTransform(ft, f).Status)
}

func Test_Transform_FinishType(t *testing.T) {
	ft, _ := getTestSecurityAdvisorTransformer()
	f := &flow.Flow{}

	assertEqual(t, "", testTransform(ft, f).FinishType)
	assertEqual(t, "", testTransform(ft, f).FinishType)

	f.FinishType = flow.FlowFinishType_TCP_FIN
	assertEqual(t, "SYN_FIN", testTransform(ft, f).FinishType)

	f.FinishType = flow.FlowFinishType_NOT_FINISHED
	assertEqual(t, "", testTransform(ft, f).FinishType)
	f.FinishType = flow.FlowFinishType_TCP_RST
	assertEqual(t, "SYN_RST", testTransform(ft, f).FinishType)

	f.FinishType = flow.FlowFinishType_NOT_FINISHED
	assertEqual(t, "", testTransform(ft, f).FinishType)
	f.FinishType = flow.FlowFinishType_TIMEOUT
	assertEqual(t, "Timeout", testTransform(ft, f).FinishType)
}

func Test_Transform_Network(t *testing.T) {
	ft, fakeClient := getTestSecurityAdvisorTransformer()
	network := &flow.FlowLayer{
		Protocol: flow.FlowProtocol_IPV4,
		A:        "aValue",
		B:        "bValue",
	}
	f := &flow.Flow{Network: network}
	transformedNetwork := testTransform(ft, f).Network
	assertEqual(t, flow.FlowProtocol_IPV4.String(), transformedNetwork.Protocol)
	assertEqual(t, "aValue", transformedNetwork.A)
	assertEqual(t, "bValue", transformedNetwork.B)
	assertEqual(t, "0_0_containeraValue_0", transformedNetwork.AName)
	assertEqual(t, "0_0_containerbValue_0", transformedNetwork.BName)
	assertEqual(t, 2, fakeClient.containerNameCallCount)

	transformedNetwork = testTransform(ft, f).Network
	assertEqual(t, "0_0_containeraValue_0", transformedNetwork.AName)
	assertEqual(t, "0_0_containerbValue_0", transformedNetwork.BName)
	assertEqual(t, 2, fakeClient.containerNameCallCount)

	fakeClient.containerNameError = errors.New("myerror")
	network.A = "a2Value"
	transformedNetwork = testTransform(ft, f).Network
	assertEqual(t, "", transformedNetwork.AName)
	assertEqual(t, 3, fakeClient.containerNameCallCount)

	transformedNetwork = testTransform(ft, f).Network
	assertEqual(t, "", transformedNetwork.AName)
	assertEqual(t, 4, fakeClient.containerNameCallCount)

	fakeClient.containerNameError = common.ErrNotFound
	transformedNetwork = testTransform(ft, f).Network
	assertEqual(t, "", transformedNetwork.AName)
	assertEqual(t, 5, fakeClient.containerNameCallCount)

	transformedNetwork = testTransform(ft, f).Network
	assertEqual(t, "", transformedNetwork.AName)
	assertEqual(t, 5, fakeClient.containerNameCallCount)
}

func Test_Transform_Transport(t *testing.T) {
	ft, _ := getTestSecurityAdvisorTransformer()
	transport := &flow.TransportLayer{
		Protocol: flow.FlowProtocol_IPV4,
		A:        10,
		B:        20,
	}
	f := &flow.Flow{Transport: transport}
	transformedNetwork := testTransform(ft, f).Transport
	assertEqual(t, flow.FlowProtocol_IPV4.String(), transformedNetwork.Protocol)
	assertEqual(t, "10", transformedNetwork.A)
	assertEqual(t, "20", transformedNetwork.B)
}

func Test_Transform_NodeType(t *testing.T) {
	ft, fakeClient := getTestSecurityAdvisorTransformer()
	f := &flow.Flow{NodeTID: "tid"}
	assertEqual(t, "typetid", testTransform(ft, f).NodeType)
	assertEqual(t, 1, fakeClient.nodeTypeCallCount)

	assertEqual(t, "typetid", testTransform(ft, f).NodeType)
	assertEqual(t, 1, fakeClient.nodeTypeCallCount)

	fakeClient.nodeTypeError = errors.New("myerror")
	f.NodeTID = "tid2"
	assertEqual(t, "", testTransform(ft, f).NodeType)
	assertEqual(t, 2, fakeClient.nodeTypeCallCount)

	assertEqual(t, "", testTransform(ft, f).NodeType)
	assertEqual(t, 3, fakeClient.nodeTypeCallCount)

	fakeClient.nodeTypeError = common.ErrNotFound
	assertEqual(t, "", testTransform(ft, f).NodeType)
	assertEqual(t, 4, fakeClient.nodeTypeCallCount)

	assertEqual(t, "", testTransform(ft, f).NodeType)
	assertEqual(t, 4, fakeClient.nodeTypeCallCount)
}

func Test_Transform_ExcludeStartedFlows(t *testing.T) {
	ft, _ := getTestSecurityAdvisorTransformer()
	ft.excludeStartedFlows = true
	f := &flow.Flow{}

	assertEqual(t, nil, ft.Transform(f))
	assertEqual(t, "UPDATED", testTransform(ft, f).Status)
}
