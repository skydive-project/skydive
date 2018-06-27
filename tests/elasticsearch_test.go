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
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/storage/elasticsearch"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology/graph"
)

const indexPrefix = "test_skydive_mwnxf0f2"
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

func getClient(t *testing.T, name string, mappings elasticsearch.Mappings, cfg elasticsearch.Config) (*elasticsearch.ElasticSearchClient, error) {
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

	return client, nil
}

func initBackend(t *testing.T, cfg elasticsearch.Config, name string) (*graph.ElasticSearchBackend, error) {
	mappings := elasticsearch.Mappings{
		{"node": []byte(graph.ESGraphElementMapping)},
		{"edge": []byte(graph.ESGraphElementMapping)},
	}

	cfg.ElasticHost = "http://localhost:9200"
	cfg.BulkMaxDocs = 1

	client, err := getClient(t, name, mappings, cfg)
	if err != nil {
		return nil, err
	}

	return graph.NewElasticSearchBackendFromClient(client)
}

// test active nodes after rolling elasticsearch indices
func TestElasticsearcActiveNodes(t *testing.T) {
	if helper.TopologyBackend != "elasticsearch" && helper.FlowBackend != "elasticsearch" {
		t.Skip("Elasticsearch is used neither for topology nor flows")
	}

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

	err = common.Retry(func() error {
		activeNodes := len(backend.GetNodes(graph.GraphContext{nil, false}, nil))
		if activeNodes != 1 {
			return fmt.Errorf("Found %d active nodes instead of 1", activeNodes)
		}
		return nil
	}, 10, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if err := delTestIndex(indexNodesName); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}

// test active edges after rolling elasticsearch indices
func TestElasticsearcActiveEdges(t *testing.T) {
	if helper.TopologyBackend != "elasticsearch" && helper.FlowBackend != "elasticsearch" {
		t.Skip("Elasticsearch is used neither for topology nor flows")
	}

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

	err = common.Retry(func() error {
		activeEdges := len(backend.GetEdges(graph.GraphContext{nil, false}, nil))
		time.Sleep(3 * time.Second)
		if activeEdges != 1 {
			t.Fatalf("Found %d active edges instead of 1", activeEdges)
		}

		if err := delTestIndex(indexEdgesName); err != nil {
			t.Fatalf("Failed to clear test indices: %s", err.Error())
		}

		return nil
	}, 10, time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func indexEntry(c *elasticsearch.ElasticSearchClient, id int) (bool, error) {
	return c.Index("test_type", fmt.Sprintf("id%d", id), `{"key": "val"}`)
}

// test rolling elasticsearch indices based on count limit
func TestElasticsearchShouldRollByCount(t *testing.T) {
	if helper.TopologyBackend != "elasticsearch" && helper.FlowBackend != "elasticsearch" {
		t.Skip("Elasticsearch is used neither for topology nor flows")
	}

	cfg := elasticsearch.NewConfig()
	cfg.EntriesLimit = 5
	name := "should_roll_by_count_test"

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}

	client, err := getClient(t, name, elasticsearch.Mappings{}, cfg)
	if err != nil {
		t.Fatalf("Initialization error: %s", err.Error())
	}

	for i := 1; i < cfg.EntriesLimit; i++ {
		if _, err := indexEntry(client, i); err != nil {
			t.Fatalf("Failed to index entry %d: %s", i, err.Error())
		}

		err = common.Retry(func() error {
			if client.ShouldRollIndex() {
				return fmt.Errorf("Index should not have rolled after %d entries (limit is %d)", i, cfg.EntriesLimit)
			}
			return nil
		}, 10, time.Second)
	}

	if _, err = indexEntry(client, cfg.EntriesLimit); err != nil {
		t.Fatalf("Failed to index entry %d: %s", cfg.EntriesLimit, err.Error())
	}
	err = common.Retry(func() error {
		if !client.ShouldRollIndex() {
			return fmt.Errorf("Index should have rolled after %d entries", cfg.EntriesLimit)
		}
		return nil
	}, 10, time.Second)

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}

// test rolling elasticsearch indices based on age limit
func TestElasticsearchShouldRollByAge(t *testing.T) {
	if helper.TopologyBackend != "elasticsearch" && helper.FlowBackend != "elasticsearch" {
		t.Skip("Elasticsearch is used neither for topology nor flows")
	}

	cfg := elasticsearch.NewConfig()
	cfg.AgeLimit = 5
	name := "should_roll_by_age_test"

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}

	client, err := getClient(t, name, elasticsearch.Mappings{}, cfg)
	if err != nil {
		t.Fatalf("Initialization error: %s", err.Error())
	}

	time.Sleep(time.Duration(cfg.AgeLimit-2) * time.Second)
	if client.ShouldRollIndex() {
		t.Fatalf("Index should not have rolled after %d seconds (limit is %d)", cfg.AgeLimit-2, cfg.AgeLimit)
	}

	time.Sleep(4 * time.Second)
	if !client.ShouldRollIndex() {
		t.Fatalf("Index should not have rolled after %d seconds (limit is %d)", cfg.AgeLimit+2, cfg.AgeLimit)
	}

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}

// test deletion of rolling elasticsearch indices
func TestElasticsearchDelIndices(t *testing.T) {
	if helper.TopologyBackend != "elasticsearch" && helper.FlowBackend != "elasticsearch" {
		t.Skip("Elasticsearch is used neither for topology nor flows")
	}

	cfg := elasticsearch.NewConfig()
	cfg.IndicesLimit = 5
	name := "del_indices_test"

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}

	client, err := getClient(t, name, elasticsearch.Mappings{}, cfg)
	if err != nil {
		t.Fatalf("Initialization error: %s", err.Error())
	}
	firstIndex := client.IndexPath()
	time.Sleep(1 * time.Second)

	for i := 1; i < cfg.IndicesLimit; i++ {
		if err := client.RollIndex(); err != nil {
			t.Fatalf("Failed to roll index %d: %s", i, err.Error())
		}
		common.Retry(func() error {
			indices, _ := client.GetClient().IndexNames()
			if len(indices) != i+1 {
				return fmt.Errorf("Should have had %d indices after %d rolls (limit is %d), but have %d", i+1, i, cfg.IndicesLimit, len(indices))
			}
			return nil
		}, 5, time.Second)
	}

	if err = client.RollIndex(); err != nil {
		t.Fatalf("Failed to roll index %d: %s", cfg.IndicesLimit, err.Error())
	}
	common.Retry(func() error {
		indices, _ := client.GetClient().IndexNames()
		if len(indices) != cfg.IndicesLimit {
			return fmt.Errorf("Should have had %d indices after %d rolls (limit is %d), but have %d", cfg.IndicesLimit, cfg.IndicesLimit, cfg.IndicesLimit, len(indices))
		}

		for _, esIndex := range indices {
			if esIndex == firstIndex {
				return fmt.Errorf("First index %s Should have been deleted", firstIndex)
			}
		}

		return nil
	}, 5, time.Second)

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}

// test mappings before and after rolling elasticsearch indices
func TestElasticsearchMappings(t *testing.T) {
	if helper.TopologyBackend != "elasticsearch" && helper.FlowBackend != "elasticsearch" {
		t.Skip("Elasticsearch is used neither for topology nor flows")
	}

	cfg := elasticsearch.NewConfig()
	name := "mappings_test"
	mapKey := "testmap"

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}

	const testMapping = `
{
	"dynamic_templates": [
		{
			"strings": {
				"match": "*",
				"match_mapping_type": "string",
				"mapping": {
					"type":       "string",
					"index":      "not_analyzed",
					"doc_values": false
				}
			}
		}
	]
}`

	client, err := getClient(t, name, elasticsearch.Mappings{{mapKey: []byte(testMapping)}}, cfg)
	if err != nil {
		t.Fatalf("Initialization error: %s", err.Error())
	}

	if err := client.RollIndex(); err != nil {
		t.Fatalf("Failed to roll index: %s", err.Error())
	}
	time.Sleep(1 * time.Second)

	mappings, err := client.GetClient().GetMapping().Index(client.GetIndexAlias()).Do(context.Background())
	if err != nil {
		t.Fatalf("Failed to retrieve mappings: %s", err.Error())
	}

	for indexName, doc := range mappings {
		if strings.HasPrefix(indexName, client.GetIndexAlias()) {
			mapping := doc.(map[string]interface{})["mappings"]
			if _, ok := mapping.(map[string]interface{})[mapKey]; !ok {
				t.Fatalf("test mapping not found: %v", mapping)
			}
		}
	}

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}
