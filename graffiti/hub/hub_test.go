/*
 * Copyright (C) 2021 Sylvain Baubeau
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

package hub

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/api/types"
	etcdclient "github.com/skydive-project/skydive/graffiti/etcd/client"
	etcdserver "github.com/skydive-project/skydive/graffiti/etcd/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/graffiti/websocket"
)

var (
	hubCount     = 0
	clusterCount = 0
)

type testCluster struct {
	hubs []*testHub
}

func (c *testCluster) start(t *testing.T) {
	startHubs(t, c.hubs)
}

func (c *testCluster) GetNode(id graph.Identifier) error {
	for _, hub := range c.hubs {
		if err := hub.GetNode(id); err != nil {
			return err
		}
	}
	return nil
}

type testHub struct {
	*Hub
	crudClient *shttp.CrudClient
}

func (h *testHub) GetNode(id graph.Identifier) error {
	var node graph.Node
	if err := h.crudClient.Get("node", string(id), &node); err != nil {
		return err
	}
	return nil
}

func newTestHub(index int, opts Opts) (*Hub, error) {
	authBackend := shttp.NewNoAuthenticationBackend()

	etcdAddr := fmt.Sprintf("http://%s:%d", etcdclient.DefaultServer, etcdclient.DefaultPort+(clusterCount*2))

	etcdClientOpts := etcdclient.Opts{
		Servers: []string{etcdAddr},
		Timeout: 5 * time.Second,
	}

	hostname := opts.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	opts.Hostname = hostname

	etcdClient, err := etcdclient.NewClient(hostname, etcdClientOpts)
	if err != nil {
		return nil, err
	}

	origin := "graffiti-hub"
	cached, err := graph.NewCachedBackend(nil, etcdClient, hostname, service.Type(origin))
	if err != nil {
		return nil, err
	}

	if opts.APIAuthBackend == nil {
		opts.APIAuthBackend = authBackend
	}

	if opts.ClusterAuthBackend == nil {
		opts.ClusterAuthBackend = authBackend
	}

	g := graph.NewGraph(hostname, cached, origin)

	opts.WebsocketOpts = websocket.ServerOpts{
		WriteCompression: false,
		QueueSize:        10000,
		PingDelay:        time.Second * time.Duration(2),
		PongTimeout:      time.Second * time.Duration(10),
		Logger:           opts.Logger,
	}

	opts.EtcdClient = etcdClient

	if index == 0 {
		opts.EtcdServerOpts = &etcdserver.EmbeddedServerOpts{
			Name:    "localhost",
			Listen:  fmt.Sprintf("127.0.0.1:%d", 12379+(clusterCount*2)),
			DataDir: "/tmp/etcd-" + hostname,
			Logger:  opts.Logger,
		}
	}

	hub, err := NewHub(hostname, service.Type("Hub"), fmt.Sprintf("127.0.0.1:%d", 8082+hubCount), g, cached, "/ws/pod", opts)
	if err != nil {
		return nil, err
	}

	return hub, nil
}

func newTestCluster(t *testing.T, name string, count int, defaultOpts Opts) (*testCluster, error) {
	hubs := make([]*testHub, count)

	oldHubCount := hubCount
	for i := 0; i < count; i++ {
		var replicationPeers []service.Address
		if count > 1 {
			for j := 0; j < count; j++ {
				if i != j {
					sa, err := service.AddressFromString(fmt.Sprintf("127.0.0.1:%d", 8082+oldHubCount+j))
					if err != nil {
						return nil, err
					}
					replicationPeers = append(replicationPeers, sa)
				}
			}
		}

		hostname := fmt.Sprintf("%s-%d", name, i)
		logging.InitLogging(hostname, true, []*logging.LoggerConfig{logging.NewLoggerConfig(logging.NewStdioBackend(os.Stderr), "DEBUG", "")})

		opts := defaultOpts

		for cluster := range opts.ClusterPeers {
			peeringOpts := opts.ClusterPeers[cluster]
			peeringOpts.WebsocketClientOpts = websocket.NewClientOpts()
		}

		opts.ClusterName = name
		opts.Hostname = hostname
		opts.ReplicationPeers = replicationPeers
		opts.Logger = logging.GetLogger()

		t.Logf("Hub %s options: %+v", name, opts)

		hub, err := newTestHub(i, opts)
		if err != nil {
			return nil, err
		}

		hubCount++

		crudClient, err := getHubClient(hub)
		if err != nil {
			return nil, err
		}

		hubs[i] = &testHub{
			Hub:        hub,
			crudClient: crudClient,
		}
	}

	clusterCount++
	return &testCluster{
		hubs: hubs,
	}, nil
}

func getHubClient(hub *Hub) (*shttp.CrudClient, error) {
	var authenticationOpts shttp.AuthenticationOpts
	url, err := shttp.MakeURL("http", "127.0.0.1", hub.httpServer.Port, "/api/", false)
	if err != nil {
		return nil, err
	}

	restClient := shttp.NewRestClient(url, &authenticationOpts, nil)
	crudClient := shttp.NewCrudClient(restClient)
	return crudClient, nil
}

func TestReplication(t *testing.T) {
	clusterSize := 2
	cluster, err := newTestCluster(t, "replication", clusterSize, Opts{})
	if err != nil {
		t.Fatal(err)
	}

	cluster.start(t)

	for _, hub := range cluster.hubs {
		status := hub.GetStatus().(*Status)

		if len(status.Peers.Incomers)+len(status.Peers.Outgoers) != clusterSize-1 {
			t.Errorf("Wrong number of incomers %d and outgoers %d", len(status.Peers.Incomers), len(status.Peers.Outgoers))
		}

		t.Logf("%+v", status)
	}

	firstHub := cluster.hubs[0]
	crudClient := firstHub.crudClient

	hostname := firstHub.httpServer.Host
	origin := graph.Origin(hostname, "test")
	metadata := graph.Metadata{
		"Name": "test-name",
		"Type": "test-type",
	}
	id := graph.GenID()
	node := types.Node(*graph.CreateNode(id, metadata, graph.Time(time.Now()), hostname, origin))

	if err := crudClient.Create("node", &node, nil); err != nil {
		t.Error(err)
	}

	if err := crudClient.Get("node", string(id), &node); err != nil {
		t.Error(err)
	}

	crudClient2, err := getHubClient(cluster.hubs[1].Hub)
	if err != nil {
		t.Fatal(err)
	}

	if err := crudClient2.Get("node", string(id), &node); err != nil {
		t.Error(err)
	}

	if err := crudClient2.Delete("node", string(id)); err != nil {
		t.Error(err)
	}

	if err := crudClient.Get("node", string(id), &node); err == nil {
		t.Errorf("should not find node %s", id)
	}

	t.Log(node)
}

func createHubNode(hub *Hub, id graph.Identifier, metadata graph.Metadata) types.Node {
	hostname := hub.httpServer.Host
	origin := graph.Origin(hostname, "test")
	return types.Node(*graph.CreateNode(id, metadata, graph.Time(time.Now()), hostname, origin))
}

func startHubs(t *testing.T, hubs []*testHub) {
	for i, hub := range hubs {
		t.Logf("Starting hub %d", i)

		if err := hub.Start(); err != nil {
			t.Error(err)
		}
	}
	time.Sleep(3 * time.Second)
}

func TestPeering(t *testing.T) {
	clusterSize := 2

	peering1Cluster, err := newTestCluster(t, "peering1", clusterSize, Opts{})
	if err != nil {
		t.Fatal(err)
	}

	sa := service.Address{
		Addr: peering1Cluster.hubs[0].httpServer.Addr,
		Port: peering1Cluster.hubs[0].httpServer.Port,
	}

	opts := Opts{
		ClusterPeers: map[string]*PeeringOpts{
			"peering1": {
				Endpoints:          []service.Address{sa},
				SubscriptionFilter: "*",
			},
		},
	}

	peering2Cluster, err := newTestCluster(t, "peering2", clusterSize, opts)
	if err != nil {
		t.Fatal(err)
	}

	hubs := append(peering1Cluster.hubs, peering2Cluster.hubs...)

	opts = Opts{
		ClusterPeers: map[string]*PeeringOpts{
			"peering1": {
				Endpoints:       []service.Address{sa},
				PublisherFilter: "*",
			},
		},
	}

	peering3Cluster, err := newTestCluster(t, "peering3", clusterSize, opts)
	if err != nil {
		t.Fatal(err)
	}

	hubs = append(hubs, peering3Cluster.hubs...)

	opts = Opts{
		ClusterPeers: map[string]*PeeringOpts{
			"peering1": {
				Endpoints:          []service.Address{sa},
				PublisherFilter:    "*",
				SubscriptionFilter: "*",
			},
		},
	}

	peering4Cluster, err := newTestCluster(t, "peering4", clusterSize, opts)
	if err != nil {
		t.Fatal(err)
	}

	hubs = append(hubs, peering4Cluster.hubs...)

	startHubs(t, hubs)

	peering1Hub := peering1Cluster.hubs[0]

	peering3Hub := peering3Cluster.hubs[0]
	peering4Hub := peering4Cluster.hubs[0]

	metadata := graph.Metadata{
		"Name": "test-name",
		"Type": "test-type",
	}
	id := graph.GenID()

	node := createHubNode(peering1Hub.Hub, id, metadata)

	// Test subscription
	t.Logf("Test subscription")

	if err := peering1Hub.crudClient.Create("node", &node, nil); err != nil {
		t.Error(err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := peering1Cluster.GetNode(id); err != nil {
		t.Error(err)
	}

	status := peering2Cluster.hubs[0].GetStatus().(*Status)

	if len(status.PeeredClusters) != 1 {
		t.Errorf("Should have one peered cluster, got %d", len(status.PeeredClusters))
	}

	t.Logf("%+v", status)

	status = peering2Cluster.hubs[1].GetStatus().(*Status)
	t.Logf("%+v", status)

	if err := peering2Cluster.GetNode(id); err != nil {
		t.Error(err)
	}

	if err := peering1Hub.crudClient.Delete("node", string(id)); err != nil {
		t.Error(err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := peering1Cluster.GetNode(id); err == nil {
		t.Errorf("should not find node %s", id)
	}

	if err := peering2Cluster.GetNode(id); err == nil {
		t.Errorf("should not find node %s", id)
	}

	// Test publishing
	t.Logf("Test publishing")

	node = createHubNode(peering3Cluster.hubs[0].Hub, id, metadata)

	if err := peering3Hub.crudClient.Create("node", &node, nil); err != nil {
		t.Error(err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := peering1Cluster.GetNode(id); err != nil {
		t.Error(err)
	}

	if err := peering2Cluster.GetNode(id); err != nil {
		t.Error(err)
	}

	if err := peering3Cluster.GetNode(id); err != nil {
		t.Error(err)
	}

	if err := peering4Cluster.GetNode(id); err != nil {
		t.Error(err)
	}

	if err := peering3Hub.crudClient.Delete("node", string(id)); err != nil {
		t.Error(err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := peering1Cluster.GetNode(id); err == nil {
		t.Errorf("should not find node %s", id)
	}

	if err := peering2Cluster.GetNode(id); err == nil {
		t.Errorf("should not find node %s", id)
	}

	if err := peering3Cluster.GetNode(id); err == nil {
		t.Errorf("should not find node %s", id)
	}

	if err := peering4Cluster.GetNode(id); err == nil {
		t.Errorf("should not find node %s", id)
	}

	// Test pubsub
	t.Logf("Test pubsub")

	node = createHubNode(peering4Cluster.hubs[0].Hub, id, metadata)

	if err := peering4Hub.crudClient.Create("node", &node, nil); err != nil {
		t.Error(err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := peering1Cluster.GetNode(id); err != nil {
		t.Error(err)
	}

	if err := peering2Cluster.GetNode(id); err != nil {
		t.Error(err)
	}

	if err := peering3Cluster.GetNode(id); err == nil {
		t.Errorf("should not find node %s", id)
	}

	t.Log(node)
}
