/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/skydive-project/skydive/storage/elasticsearch"
	"github.com/skydive-project/skydive/topology/graph"
)

const indexNodesName string = "test_nodes_2uzxp52i"
const indexEdgesName string = "test_edges_tz4tz7er"

func newGraph(t *testing.T) *graph.Graph {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	return graph.NewGraphFromConfig(b)
}

func delTestIndex(name string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://localhost:9200/skydive_%s*", name), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return nil
}

func initBackend(t *testing.T, cfg elasticsearch.Config, name string) (*graph.ElasticSearchBackend, error) {
	mappings := elasticsearch.Mappings{
		{"node": []byte(graph.ESGraphElementMapping)},
		{"edge": []byte(graph.ESGraphElementMapping)},
	}

	cfg.ElasticHost = "http://localhost:9200"
	cfg.BulkMaxDocs = 1

	client, err := elasticsearch.NewElasticSearchClient(name, mappings, cfg)
	if err != nil {
		return nil, err
	}

	client.Start()

	timeout := 50
	for client.Started() == false {
		time.Sleep(200 * time.Millisecond)
		timeout--
		if timeout == 0 {
			t.Fatal("Failed to connect to elasticsearch")
		}
	}

	return graph.NewElasticSearchBackendFromClient(client)
}

// test active nodes after rolling elasticsearch indices
func TestElasticsearcActiveNodes(t *testing.T) {
	cfg := elasticsearch.NewConfig()
	cfg.EntriesLimit = 10

	if err := delTestIndex(indexNodesName); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}

	backend, err := initBackend(t, cfg, indexNodesName)
	if err != nil {
		t.Fatalf("Failed to create backend: %s", err.Error())
	}
	g := graph.NewGraphFromConfig(backend)

	mg := newGraph(t)
	node := mg.NewNode("aaa", nil, "host1")

	g.NodeAdded(node)
	for i := 1; i <= cfg.EntriesLimit+1; i++ {
		time.Sleep(1 * time.Second)
		g.SetMetadata(node, graph.Metadata{"Temp": i})
	}
	time.Sleep(3 * time.Second)

	activeNodes := len(backend.GetNodes(graph.GraphContext{nil, false}, nil))
	if activeNodes != 1 {
		t.Fatalf("Found %d active nodes instead of 1", activeNodes)
	}

	if err := delTestIndex(indexNodesName); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}

// test active edges after rolling elasticsearch indices
func TestElasticsearcActiveEdges(t *testing.T) {
	cfg := elasticsearch.NewConfig()
	cfg.EntriesLimit = 10

	if err := delTestIndex(indexEdgesName); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}

	backend, err := initBackend(t, cfg, indexEdgesName)
	if err != nil {
		t.Fatalf("Failed to create backend: %s", err.Error())
	}
	g := graph.NewGraphFromConfig(backend)

	mg := newGraph(t)
	node1 := mg.NewNode("aaa", nil, "host1")
	node2 := mg.NewNode("bbb", nil, "host1")
	edge := mg.NewEdge("ccc", node1, node2, nil, "host1")

	g.NodeAdded(node1)
	g.NodeAdded(node2)
	g.EdgeAdded(edge)
	for i := 1; i < cfg.EntriesLimit; i++ {
		time.Sleep(1 * time.Second)
		g.SetMetadata(edge, graph.Metadata{"Temp": i})
	}
	time.Sleep(3 * time.Second)

	activeEdges := len(backend.GetEdges(graph.GraphContext{nil, false}, nil))
	time.Sleep(3 * time.Second)
	if activeEdges != 1 {
		t.Fatalf("Found %d active edges instead of 1", activeEdges)
	}

	if err := delTestIndex(indexEdgesName); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}
