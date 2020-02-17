/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package agent

import (
	"fmt"
	"net/http"
	"os"
	"time"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/client"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/pod"
	"github.com/skydive-project/skydive/graffiti/websocket"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/ondemand/server"
	"github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/ui"
)

// Agent object started on each hosts/namespaces
type Agent struct {
	ws.DefaultSpeakerEventHandler
	pod                 *pod.Pod
	graph               *graph.Graph
	analyzerClientPool  *ws.StructClientPool
	rootNode            *graph.Node
	topologyProbeBundle *probe.Bundle
	flowProbeBundle     *probe.Bundle
	flowTableAllocator  *flow.TableAllocator
	flowClientPool      *client.FlowClientPool
	onDemandProbeServer *server.OnDemandServer
	onDemandPIServer    *server.OnDemandServer
	httpServer          *shttp.Server
	tidMapper           *topology.TIDMapper
}

// NewAnalyzerStructClientPool creates a new http WebSocket client Pool
// with authentication
func NewAnalyzerStructClientPool(authOpts *shttp.AuthenticationOpts) (*ws.StructClientPool, error) {
	pool := ws.NewStructClientPool("AnalyzerClientPool", ws.PoolOpts{})

	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err)
	}

	if len(addresses) == 0 {
		logging.GetLogger().Info("Agent is running in standalone mode")
		return pool, nil
	}

	for _, sa := range addresses {
		url := config.GetURL("ws", sa.Addr, sa.Port, "/ws/agent/topology")
		c, err := config.NewWSClient(common.AgentService, url, websocket.ClientOpts{AuthOpts: authOpts, Protocol: websocket.ProtobufProtocol})
		if err != nil {
			return nil, err
		}
		pool.AddClient(c)
	}

	return pool, nil
}

// Status agent object
//
// Status describes the status of an agent
//
// swagger:model AgentStatus
type Status struct {
	Clients        map[string]ws.ConnStatus
	Analyzers      map[string]pod.ConnStatus
	TopologyProbes map[string]interface{}
	FlowProbes     []string
}

// GetStatus returns the status of an agent
func (a *Agent) GetStatus() interface{} {
	podStatus := a.pod.GetStatus()

	return &Status{
		Clients:        podStatus.Subscribers,
		Analyzers:      podStatus.Hubs,
		TopologyProbes: a.topologyProbeBundle.GetStatus(),
		FlowProbes:     a.flowProbeBundle.EnabledProbes(),
	}
}

// Start the agent services
func (a *Agent) Start() {
	a.pod.Start()
	a.topologyProbeBundle.Start()
	a.flowProbeBundle.Start()
	a.onDemandPIServer.Start()
	a.onDemandProbeServer.Start()

	// everything is ready, then initiate the websocket connection
	go a.analyzerClientPool.ConnectAll()
}

// Stop agent services
func (a *Agent) Stop() {
	a.pod.Stop()
	a.onDemandPIServer.Stop()
	a.onDemandProbeServer.Stop()
	a.flowProbeBundle.Stop()
	a.analyzerClientPool.Stop()
	a.topologyProbeBundle.Stop()
	a.flowClientPool.Close()

	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}

	a.tidMapper.Stop()
}

// NewAgent instantiates a new Agent aiming to launch probes (topology and flow)
func NewAgent() (*Agent, error) {
	if uid := os.Geteuid(); uid != 0 {
		logging.GetLogger().Warning("Agent needs root permissions for some feature like capture, network namespace introspection, some feature might not work as expected")
	}

	backend, err := graph.NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	hostID := config.GetString("host_id")
	g := graph.NewGraph(hostID, backend, common.AgentService)

	tm := topology.NewTIDMapper(g)
	tm.Start()

	apiAuthBackendName := config.GetString("agent.auth.api.backend")
	apiAuthBackend, err := config.NewAuthenticationBackendByName(apiAuthBackendName)
	if err != nil {
		return nil, err
	}

	rootNode, err := createRootNode(g)
	if err != nil {
		return nil, err
	}

	clusterAuthOptions := &shttp.AuthenticationOpts{
		Username: config.GetString("agent.auth.cluster.username"),
		Password: config.GetString("agent.auth.cluster.password"),
		Cookie:   config.GetStringMapString("http.cookie"),
	}

	analyzerClientPool, err := NewAnalyzerStructClientPool(clusterAuthOptions)
	if err != nil {
		return nil, err
	}

	validator, err := topology.NewSchemaValidator()
	if err != nil {
		return nil, fmt.Errorf("Unable to instantiate a schema validator: %s", err)
	}

	opts := pod.Opts{
		WebsocketOpts: websocket.ServerOpts{
			WriteCompression: true,
			QueueSize:        10000,
			PingDelay:        2 * time.Second,
			PongTimeout:      5 * time.Second,
		},
		Validator:          validator,
		APIAuthBackend:     apiAuthBackend,
		ClusterAuthOptions: clusterAuthOptions,
	}

	listenAddr := config.GetString("agent.listen")
	pod, err := pod.NewPod(hostID, common.AgentService, listenAddr, analyzerClientPool, g, opts)
	if err != nil {
		return nil, err
	}

	// declare all extension available through API and filtering
	tr := pod.GremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewMetricsTraversalExtension())
	tr.AddTraversalExtension(ge.NewSocketsTraversalExtension())
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())
	tr.AddTraversalExtension(ge.NewNextHopTraversalExtension())

	topologyProbeBundle, err := NewTopologyProbeBundle(g, rootNode)
	if err != nil {
		return nil, err
	}

	updateEvery := time.Duration(config.GetInt("flow.update")) * time.Second
	expireAfter := time.Duration(config.GetInt("flow.expire")) * time.Second

	flowClientPool := client.NewFlowClientPool(analyzerClientPool, clusterAuthOptions)
	flowTableAllocator := flow.NewTableAllocator(updateEvery, expireAfter, flowClientPool)

	// exposes a flow server through the client connections
	flow.NewWSTableServer(flowTableAllocator, analyzerClientPool)

	flowProbeBundle := NewFlowProbeBundle(topologyProbeBundle, g, flowTableAllocator)

	onDemandPIServer, err := packetinjector.NewOnDemandInjectionServer(g, analyzerClientPool)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize on-demand packet injection: %s", err)
	}

	onDemandProbeServer, err := ondemand.NewOnDemandFlowProbeServer(flowProbeBundle, g, analyzerClientPool)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize on-demand flow probe: %s", err)
	}

	httpServer := pod.HTTPServer()
	ui.NewServer(httpServer, config.GetString("ui.extra_assets"))

	agent := &Agent{
		pod:                 pod,
		graph:               g,
		analyzerClientPool:  analyzerClientPool,
		rootNode:            rootNode,
		topologyProbeBundle: topologyProbeBundle,
		flowProbeBundle:     flowProbeBundle,
		flowTableAllocator:  flowTableAllocator,
		flowClientPool:      flowClientPool,
		onDemandProbeServer: onDemandProbeServer,
		onDemandPIServer:    onDemandPIServer,
		tidMapper:           tm,
	}

	api.RegisterStatusAPI(httpServer, agent, apiAuthBackend)

	return agent, nil
}
