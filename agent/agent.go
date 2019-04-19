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
	fprobes "github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/pod"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/ui"
	"github.com/skydive-project/skydive/websocket"
	ws "github.com/skydive-project/skydive/websocket"
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
	onDemandProbeServer *ondemand.OnDemandProbeServer
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

// Status represents the status of an agent
type Status struct {
	Clients        map[string]ws.ConnStatus
	Analyzers      map[string]pod.ConnStatus
	TopologyProbes []string
	FlowProbes     []string
}

// GetStatus returns the status of an agent
func (a *Agent) GetStatus() interface{} {
	podStatus := a.pod.GetStatus()

	return &Status{
		Clients:        podStatus.Subscribers,
		Analyzers:      podStatus.Hubs,
		TopologyProbes: a.topologyProbeBundle.ActiveProbes(),
		FlowProbes:     a.flowProbeBundle.ActiveProbes(),
	}
}

// Start the agent services
func (a *Agent) Start() {
	if uid := os.Geteuid(); uid != 0 {
		logging.GetLogger().Warning("Agent needs root permissions for some feature like capture, network namespace introspection, some feature might not work as expected")
	}

	go a.httpServer.Serve()

	a.topologyProbeBundle.Start()
	a.flowProbeBundle.Start()
	a.onDemandProbeServer.Start()

	// everything is ready, then initiate the websocket connection
	go a.analyzerClientPool.ConnectAll()
}

// Stop agent services
func (a *Agent) Stop() {
	a.flowProbeBundle.Stop()
	a.analyzerClientPool.Stop()
	a.topologyProbeBundle.Stop()
	a.httpServer.Stop()
	a.flowClientPool.Close()
	a.onDemandProbeServer.Stop()

	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}

	a.tidMapper.Stop()
}

// NewAgent instantiates a new Agent aiming to launch probes (topology and flow)
func NewAgent() (*Agent, error) {
	backend, err := graph.NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	hostID := config.GetString("host_id")
	service := common.Service{ID: hostID, Type: common.AgentService}

	g := graph.NewGraph(hostID, backend, service.Type)

	tm := topology.NewTIDMapper(g)
	tm.Start()

	apiAuthBackendName := config.GetString("agent.auth.api.backend")
	apiAuthBackend, err := config.NewAuthenticationBackendByName(apiAuthBackendName)
	if err != nil {
		return nil, err
	}

	hserver, err := config.NewHTTPServer(service.Type)
	if err != nil {
		return nil, err
	}

	uiServer := ui.NewServer(hserver, config.GetString("ui.extra_assets"))
	uiServer.RegisterLoginRoute(apiAuthBackend)

	if err = hserver.Listen(); err != nil {
		return nil, err
	}

	apiServer, err := api.NewAPI(hserver, nil, service, apiAuthBackend)
	if err != nil {
		return nil, err
	}

	// declare all extension available through API and filtering
	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewMetricsTraversalExtension())
	tr.AddTraversalExtension(ge.NewSocketsTraversalExtension())
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())
	tr.AddTraversalExtension(ge.NewNextHopTraversalExtension())

	rootNode, err := createRootNode(g)
	if err != nil {
		return nil, err
	}

	api.RegisterTopologyAPI(hserver, g, tr, apiAuthBackend)

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
		ServerOpts: websocket.ServerOpts{
			WriteCompression: true,
			QueueSize:        10000,
			PingDelay:        2 * time.Second,
			PongTimeout:      5 * time.Second,
		},
		Validator: validator,
	}

	pod, err := pod.NewPod(apiServer, analyzerClientPool, g, apiAuthBackend, clusterAuthOptions, tr, opts)
	if err != nil {
		return nil, err
	}

	topologyProbeBundle, err := NewTopologyProbeBundleFromConfig(g, rootNode)
	if err != nil {
		return nil, err
	}

	updateTime := time.Duration(config.GetInt("flow.update")) * time.Second
	expireTime := time.Duration(config.GetInt("flow.expire")) * time.Second

	flowTableAllocator := flow.NewTableAllocator(updateTime, expireTime)

	// exposes a flow server through the client connections
	flow.NewWSTableServer(flowTableAllocator, analyzerClientPool)

	packetinjector.NewServer(g, analyzerClientPool)

	flowClientPool := client.NewFlowClientPool(analyzerClientPool, clusterAuthOptions)

	flowProbeBundle := fprobes.NewFlowProbeBundle(topologyProbeBundle, g, flowTableAllocator, flowClientPool)

	onDemandProbeServer, err := ondemand.NewOnDemandProbeServer(flowProbeBundle, g, analyzerClientPool)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize on-demand flow probe %s", err)
	}

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
		httpServer:          hserver,
		tidMapper:           tm,
	}

	api.RegisterStatusAPI(hserver, agent, apiAuthBackend)

	return agent, nil
}
