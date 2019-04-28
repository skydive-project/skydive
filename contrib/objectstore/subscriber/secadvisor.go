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
	"strconv"
	"time"

	"github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/logging"
)

const version = "1.0.8"

// SecurityAdvisorFlowLayer is the flow layer for a security advisor flow
type SecurityAdvisorFlowLayer struct {
	Protocol string `json:"Protocol,omitempty"`
	A        string `json:"A,omitempty"`
	B        string `json:"B,omitempty"`
	AName    string `json:"A_Name,omitempty"`
	BName    string `json:"B_Name,omitempty"`
}

// SecurityAdvisorFlow represents a security advisor flow
type SecurityAdvisorFlow struct {
	UUID             string                    `json:"UUID,omitempty"`
	LayersPath       string                    `json:"LayersPath,omitempty"`
	Version          string                    `json:"Version,omitempty"`
	Status           string                    `json:"Status,omitempty"`
	FinishType       string                    `json:"FinishType,omitempty"`
	Network          *SecurityAdvisorFlowLayer `json:"Network,omitempty"`
	Transport        *SecurityAdvisorFlowLayer `json:"Transport,omitempty"`
	LastUpdateMetric *flow.FlowMetric          `json:"LastUpdateMetric,omitempty"`
	Metric           *flow.FlowMetric          `json:"Metric,omitempty"`
	Start            int64                     `json:"Start"`
	Last             int64                     `json:"Last"`
	UpdateCount      int64                     `json:"UpdateCount"`
	NodeType         string                    `json:"NodeType,omitempty"`
}

type securityAdvisorGraphClient interface {
	getContainerName(ipString, nodeTID string) (string, error)
	getNodeType(nodeTID string) (string, error)
}

// SecurityAdvisorFlowTransformer is a custom transformer for flows
type securityAdvisorFlowTransformer struct {
	flowUpdateCount     *cache.Cache
	graphClient         securityAdvisorGraphClient
	containerName       *cache.Cache
	nodeType            *cache.Cache
	excludeStartedFlows bool
}

type securityAdvisorGremlinClient struct {
	gremlinClient *client.GremlinQueryHelper
}

func (c *securityAdvisorGremlinClient) getContainerName(ipString, nodeTID string) (string, error) {
	node, err := c.gremlinClient.GetNode(g.G.V().Has("Runc.Hosts.IP", ipString).ShortestPathTo(g.Metadata("TID", nodeTID)))
	if err != nil {
		return "", err
	}

	return node.Metadata["Runc"].(map[string]interface{})["Hosts"].(map[string]interface{})["Hostname"].(string), nil
}

func (c *securityAdvisorGremlinClient) getNodeType(nodeTID string) (string, error) {
	node, err := c.gremlinClient.GetNode(g.G.V().Has("TID", nodeTID))
	if err != nil {
		return "", err
	}

	return node.Metadata["Type"].(string), nil
}

func (ft *securityAdvisorFlowTransformer) getContainerName(ipString, nodeTID string) string {
	if ipString == "" {
		return ""
	}
	containerName, ok := ft.containerName.Get(ipString)
	if !ok {
		var err error
		if containerName, err = ft.graphClient.getContainerName(ipString, nodeTID); err != nil {
			containerName = ""
		} else {
			containerName = "0_0_" + containerName.(string) + "_0"
		}
		if err != nil && err != common.ErrNotFound {
			logging.GetLogger().Warningf("Failed to query container name for IP '%s': %s", ipString, err.Error())
		} else {
			ft.containerName.Set(ipString, containerName, cache.DefaultExpiration)
		}
	}

	return containerName.(string)
}

func (ft *securityAdvisorFlowTransformer) getNodeType(f *flow.Flow) string {
	tid := f.NodeTID
	nodeType, ok := ft.nodeType.Get(tid)
	if !ok {
		var err error
		if nodeType, err = ft.graphClient.getNodeType(tid); err != nil {
			nodeType = ""
		}
		if err != nil && err != common.ErrNotFound {
			logging.GetLogger().Warningf("Failed to query node type for TID '%s': %s", tid, err.Error())
		} else {
			ft.nodeType.Set(tid, nodeType, cache.DefaultExpiration)
		}
	}

	return nodeType.(string)
}

func (ft *securityAdvisorFlowTransformer) setUpdateCount(f *flow.Flow) int64 {
	var count int64
	if countRaw, ok := ft.flowUpdateCount.Get(f.UUID); ok {
		count = countRaw.(int64)
	}

	if f.FinishType != flow.FlowFinishType_TIMEOUT {
		if f.FinishType == flow.FlowFinishType_NOT_FINISHED {
			ft.flowUpdateCount.Set(f.UUID, count+1, cache.DefaultExpiration)
		} else {
			ft.flowUpdateCount.Set(f.UUID, count+1, time.Minute)
		}
	} else {
		ft.flowUpdateCount.Delete(f.UUID)
	}

	return count
}

func (ft *securityAdvisorFlowTransformer) getStatus(f *flow.Flow, updateCount int64) string {
	if f.FinishType != flow.FlowFinishType_NOT_FINISHED {
		return "ENDED"
	}

	if updateCount == 0 {
		return "STARTED"
	}

	return "UPDATED"
}

func (ft *securityAdvisorFlowTransformer) getFinishType(f *flow.Flow) string {
	if f.FinishType == flow.FlowFinishType_TCP_FIN {
		return "SYN_FIN"
	}
	if f.FinishType == flow.FlowFinishType_TCP_RST {
		return "SYN_RST"
	}
	if f.FinishType == flow.FlowFinishType_TIMEOUT {
		return "Timeout"
	}
	return ""
}

func (ft *securityAdvisorFlowTransformer) getNetwork(f *flow.Flow) *SecurityAdvisorFlowLayer {
	if f.Network == nil {
		return nil
	}

	return &SecurityAdvisorFlowLayer{
		Protocol: f.Network.Protocol.String(),
		A:        f.Network.A,
		B:        f.Network.B,
		AName:    ft.getContainerName(f.Network.A, f.NodeTID),
		BName:    ft.getContainerName(f.Network.B, f.NodeTID),
	}
}

func (ft *securityAdvisorFlowTransformer) getTransport(f *flow.Flow) *SecurityAdvisorFlowLayer {
	if f.Transport == nil {
		return nil
	}

	return &SecurityAdvisorFlowLayer{
		Protocol: f.Transport.Protocol.String(),
		A:        strconv.FormatInt(f.Transport.A, 10),
		B:        strconv.FormatInt(f.Transport.B, 10),
	}
}

// Transform transforms a flow before being stored
func (ft *securityAdvisorFlowTransformer) Transform(f *flow.Flow) interface{} {
	updateCount := ft.setUpdateCount(f)
	status := ft.getStatus(f, updateCount)

	// do not report new flows (i.e. the first time you see them)
	if ft.excludeStartedFlows && status == "STARTED" {
		return nil
	}

	return &SecurityAdvisorFlow{
		UUID:             f.UUID,
		LayersPath:       f.LayersPath,
		Version:          version,
		Status:           status,
		FinishType:       ft.getFinishType(f),
		Network:          ft.getNetwork(f),
		Transport:        ft.getTransport(f),
		LastUpdateMetric: f.LastUpdateMetric,
		Metric:           f.Metric,
		Start:            f.Start,
		Last:             f.Last,
		UpdateCount:      updateCount,
		NodeType:         ft.getNodeType(f),
	}
}

// newSecurityAdvisorFlowTransformer returns a new SecurityAdvisorFlowTransformer
func newSecurityAdvisorFlowTransformer(gremlinClient *client.GremlinQueryHelper) *securityAdvisorFlowTransformer {
	cfg := config.GetConfig()
	cfgPrefix := "objectstore.security_advisor."

	excludeStartedFlows := cfg.GetBool(cfgPrefix + "exclude_started_flows")
	return &securityAdvisorFlowTransformer{
		flowUpdateCount:     cache.New(10*time.Minute, 10*time.Minute),
		graphClient:         &securityAdvisorGremlinClient{gremlinClient},
		containerName:       cache.New(5*time.Minute, 10*time.Minute),
		nodeType:            cache.New(5*time.Minute, 10*time.Minute),
		excludeStartedFlows: excludeStartedFlows,
	}
}
