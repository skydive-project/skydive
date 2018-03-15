/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package graph

import (
	//"encoding/json"
	//"reflect"
	"sort"
	"strings"
	"testing"
	"time"
	"net/http"

	//"github.com/davecgh/go-spew/spew"
	elastigo "github.com/mattbaird/elastigo/lib"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/storage/elasticsearch"
	"fmt"
)

type revisionArray []interface{}

func (a revisionArray) Len() int {
	return len(a)
}
func (a revisionArray) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a revisionArray) Less(i, j int) bool {
	e := a[i].(map[string]interface{})
	f := a[j].(map[string]interface{})

	v1, v2 := e["Revision"].(int64), f["Revision"].(int64)

	return v1 < v2
}

type fakeElasticsearchClient struct {
	revisions    map[string]interface{}
	searches     []string
	searchResult elastigo.SearchResult
	shouldRoll   bool
}

func (f *fakeElasticsearchClient) getRevisions() []interface{} {
	v := make(revisionArray, 0, len(f.revisions))

	for _, value := range f.revisions {
		v = append(v, value)
	}

	sort.Sort(v)

	return []interface{}(v)
}

func (f *fakeElasticsearchClient) resetRevisions() {
	f.revisions = make(map[string]interface{})
}

func (f *fakeElasticsearchClient) FormatFilter(filter *filters.Filter, mapKey string) map[string]interface{} {
	es := &elasticsearch.ElasticSearchClient{}
	return es.FormatFilter(filter, mapKey)
}

func (f *fakeElasticsearchClient) GetIndexAlias() string {
	return "skydive_test"
}

func (f *fakeElasticsearchClient) GetIndexAllAlias() string {
	return "skydive_all"
}

func (f *fakeElasticsearchClient) RollIndex() error {
	f.revisions = make(map[string]interface{})
	f.shouldRoll = false
	return nil
}

func (f *fakeElasticsearchClient) Index(obj string, id string, data interface{}) (error, bool) {
	f.revisions[id] = data
	return nil, f.shouldRoll
}
func (f *fakeElasticsearchClient) BulkIndex(obj string, id string, data interface{}) (error, bool) {
	f.revisions[id] = data
	return nil,  f.shouldRoll
}
func (f *fakeElasticsearchClient) IndexChild(obj string, parent string, id string, data interface{}) (error, bool) {
	return nil,  f.shouldRoll
}
func (f *fakeElasticsearchClient) BulkIndexChild(obj string, parent string, id string, data interface{}) (error, bool) {
	return nil,  f.shouldRoll
}
func (f *fakeElasticsearchClient) Update(obj string, id string, data interface{}) error {
	return nil
}
func (f *fakeElasticsearchClient) BulkUpdate(obj string, id string, data interface{}) error {
	return nil
}
func (f *fakeElasticsearchClient) UpdateWithPartialDoc(obj string, id string, data interface{}) error {
	return nil
}
func (f *fakeElasticsearchClient) BulkUpdateWithPartialDoc(obj string, id string, data interface{}) error {
	if revision, ok := f.revisions[id]; ok {
		m := revision.(map[string]interface{})
		for k, v := range data.(map[string]interface{}) {
			m[k] = v
		}
	}
	return nil
}
func (f *fakeElasticsearchClient) Get(obj string, id string) (elastigo.BaseResponse, error) {
	return elastigo.BaseResponse{}, nil
}
func (f *fakeElasticsearchClient) Delete(obj string, id string) (elastigo.BaseResponse, error) {
	return elastigo.BaseResponse{}, nil
}
func (f *fakeElasticsearchClient) BulkDelete(obj string, id string) {
}
func (f *fakeElasticsearchClient) Search(obj string, query string, index string) (elastigo.SearchResult, error) {
	f.searches = append(f.searches, query)
	return f.searchResult, nil
}
func (f *fakeElasticsearchClient) Start(name string, mappings []map[string][]byte, entriesLimit, ageLimit, indicesLimit int) {
}

func newElasticsearchGraph(t *testing.T) (*Graph, *fakeElasticsearchClient) {
	client := &fakeElasticsearchClient{
		revisions: make(map[string]interface{}),
		shouldRoll: false,
	}
	b, err := newElasticSearchBackend(client)

	if err != nil {
		t.Error(err.Error())
	}

	return NewGraphFromConfig(b), client
}

// test history when doing local modification
func TestElasticsearchLocal(t *testing.T) {
	g, client := newElasticsearchGraph(t)

	node := g.newNode("aaa", Metadata{"MTU": 1500}, time.Unix(1, 0), "host1")
	g.addMetadata(node, "MTU", 1510, time.Unix(2, 0))

	expected := []interface{}{
		map[string]interface{}{
			"ArchivedAt": int64(2000),
			"CreatedAt":  int64(1000),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata": Metadata{
				"MTU": 1500,
			},
			"Revision":  int64(1),
			"UpdatedAt": int64(1000),
		},
		map[string]interface{}{
			"CreatedAt": int64(1000),
			"Host":      "host1",
			"ID":        "aaa",
			"Metadata": Metadata{
				"MTU": 1510,
			},
			"Revision":  int64(2),
			"UpdatedAt": int64(2000),
		},
	}

	if !reflect.DeepEqual(client.getRevisions(), expected) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %v\ngot: %v", expected, client.getRevisions())
	}

	g.addMetadata(node, "MTU", 1520, time.Unix(3, 0))

	expected = []interface{}{
		map[string]interface{}{
			"ArchivedAt": int64(2000),
			"UpdatedAt":  int64(1000),
			"CreatedAt":  int64(1000),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata": Metadata{
				"MTU": 1500,
			},
			"Revision": int64(1),
		},
		map[string]interface{}{
			"ArchivedAt": int64(3000),
			"UpdatedAt":  int64(2000),
			"CreatedAt":  int64(1000),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata": Metadata{
				"MTU": 1510,
			},
			"Revision": int64(2),
		},
		map[string]interface{}{
			"UpdatedAt": int64(3000),
			"CreatedAt": int64(1000),
			"Host":      "host1",
			"ID":        "aaa",
			"Metadata": Metadata{
				"MTU": 1520,
			},
			"Revision": int64(3),
		},
	}

	if !reflect.DeepEqual(client.getRevisions(), expected) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %v\ngot: %v", expected, client.getRevisions())
	}

	client.searches = []string{}

	g.delNode(node, time.Unix(4, 0))

	expected = []interface{}{
		map[string]interface{}{
			"ArchivedAt": int64(2000),
			"UpdatedAt":  int64(1000),
			"CreatedAt":  int64(1000),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata": Metadata{
				"MTU": 1500,
			},
			"Revision": int64(1),
		},
		map[string]interface{}{
			"ArchivedAt": int64(3000),
			"UpdatedAt":  int64(2000),
			"CreatedAt":  int64(1000),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata": Metadata{
				"MTU": 1510,
			},
			"Revision": int64(2),
		},
		map[string]interface{}{
			"ArchivedAt": int64(4000),
			"DeletedAt":  int64(4000),
			"UpdatedAt":  int64(3000),
			"CreatedAt":  int64(1000),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata": Metadata{
				"MTU": 1520,
			},
			"Revision": int64(3),
		},
	}

	if !reflect.DeepEqual(client.getRevisions(), expected) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %v\ngot: %v", expected, client.getRevisions())
	}

	var searchEdge interface{}
	json.Unmarshal([]byte(client.searches[0]), &searchEdge)

	searchExpected := map[string]interface{}{
		"size": float64(10000),
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": "ArchivedAt",
								},
							},
						},
					},
					map[string]interface{}{
						"bool": map[string]interface{}{
							"should": []interface{}{
								map[string]interface{}{
									"term": map[string]interface{}{
										"Parent": "aaa",
									},
								},
								map[string]interface{}{
									"term": map[string]interface{}{
										"Child": "aaa",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	maxConns := config.GetInt("storage.elasticsearch.maxconns")
	retrySeconds := config.GetInt("storage.elasticsearch.retry")
	bulkMaxDocs := 1
	bulkMaxDelay := config.GetInt("storage.elasticsearch.bulk_maxdelay")

	client, err := elasticsearch.NewElasticSearchClient(c[0], c[1], maxConns, retrySeconds, bulkMaxDocs, bulkMaxDelay)
	if err != nil {
		return nil, err
	}

	ageLimit := config.GetInt("storage.elasticsearch.index_age_limit")
	indicesLimit := config.GetInt("storage.elasticsearch.indices_to_keep")
	client.Start("test", []map[string][]byte{
		{"node": []byte(graphElementMapping)},
		{"edge": []byte(graphElementMapping)}},
		entriesLimit, ageLimit, indicesLimit,
	)

	return &ElasticSearchBackend{
		client:       client,
		prevRevision: make(map[Identifier]int64),
	}, nil
}

// test active nodes after rolling elasticsearch indices
//func TestElasticsearcActiveNodes(t *testing.T) {
//	entriesLimit := 3
//	backend, err := initBackend(entriesLimit)
//	if err != nil {
//		t.Fatalf("Failed to create backend: %s", err.Error())
//	}
//	g := NewGraphFromConfig(backend)
//
//	mg := newGraph(t)
//	node := mg.NewNode("aaa", nil, "host1")
//
//	g.NodeAdded(node)
//	for i := 1; i <= entriesLimit * 2; i++ {
//		time.Sleep(2 * time.Second)
//		g.SetMetadata(node, Metadata{"Temp": i})
//	}
//	time.Sleep(5 * time.Second)
//
//	activeNodes := len(backend.GetNodes(nil, nil))
//	if activeNodes != 1 {
//		t.Fatalf("Found %d active nodes instead of 1", activeNodes)
//	}
//}

// test active edges after rolling elasticsearch indices
func TestElasticsearcActiveEdges(t *testing.T) {
	entriesLimit := 3
	backend, err := initBackend(entriesLimit)
	if err != nil {
		t.Fatalf("Failed to create backend: %s", err.Error())
	}
	g := NewGraphFromConfig(backend)

	mg := newGraph(t)
	node1 := mg.NewNode("aaa", nil, "host1")
	node2 := mg.NewNode("bbb", nil, "host1")
	edge := mg.NewEdge("ccc", node1, node2, nil, "host1")

	g.NodeAdded(node1)
	g.NodeAdded(node2)
	g.EdgeAdded(edge)
	for i := 1; i < entriesLimit; i++ {
		time.Sleep(2 * time.Second)
		g.SetMetadata(edge, Metadata{"Temp": i})
	}
	time.Sleep(5 * time.Second)

	metadat := map[string]interface{}{"ArchivedAt": nil}
	activeEdges := len(backend.GetEdges(nil, Metadata(metadat)))
	//filters.NewNullFilter("ArchivedAt")
	time.Sleep(5 * time.Second)
	if activeEdges != 1 {

		t.Fatalf("Found %d active edges instead of 1", activeEdges)
	}

	if !reflect.DeepEqual(client.getRevisions(), expected) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %v\ngot: %v", expected, client.getRevisions())
	}
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

func initBackend(entriesLimit int, name string) (*ElasticSearchBackend, error){
	addr := config.GetString("storage.elasticsearch.host")
	c := strings.Split(addr, ":")
	if len(c) != 2 {
		return nil, ErrBadConfig
	}

	maxConns := config.GetInt("storage.elasticsearch.maxconns")
	retrySeconds := config.GetInt("storage.elasticsearch.retry")
	bulkMaxDocs := 1
	bulkMaxDelay := config.GetInt("storage.elasticsearch.bulk_maxdelay")

	client, err := elasticsearch.NewElasticSearchClient(c[0], c[1], maxConns, retrySeconds, bulkMaxDocs, bulkMaxDelay)
	if err != nil {
		return nil, err
	}

	ageLimit := config.GetInt("storage.elasticsearch.index_age_limit")
	indicesLimit := config.GetInt("storage.elasticsearch.indices_to_keep")
	client.Start(name, []map[string][]byte{
		{"node": []byte(graphElementMapping)},
		{"edge": []byte(graphElementMapping)}},
		entriesLimit, ageLimit, indicesLimit,
	)

	return &ElasticSearchBackend{
		client:       client,
		prevRevision: make(map[Identifier]int64),
	}, nil
}

// test active nodes after rolling elasticsearch indices
func TestElasticsearcActiveNodes(t *testing.T) {
	entriesLimit := 3
	name := "test_nodes"
	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}


	backend, err := initBackend(entriesLimit, name)
	if err != nil {
		t.Fatalf("Failed to create backend: %s", err.Error())
	}
	g := NewGraphFromConfig(backend)

	mg := newGraph(t)
	node := mg.NewNode("aaa", nil, "host1")

	g.NodeAdded(node)
	for i := 1; i <= entriesLimit+1; i++ {
		time.Sleep(3 * time.Second)
		g.SetMetadata(node, Metadata{"Temp": i})
	}
	time.Sleep(5 * time.Second)

	activeNodes := len(backend.GetNodes(GraphContext{nil, false}, nil))
	if activeNodes != 1 {
		t.Fatalf("Found %d active nodes instead of 1", activeNodes)
	}

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}

// test active edges after rolling elasticsearch indices
func TestElasticsearcActiveEdges(t *testing.T) {
	entriesLimit := 3
	name := "test_edges"
	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}

	backend, err := initBackend(entriesLimit, name)
	if err != nil {
		t.Fatalf("Failed to create backend: %s", err.Error())
	}
	g := NewGraphFromConfig(backend)

	mg := newGraph(t)
	node1 := mg.NewNode("aaa", nil, "host1")
	node2 := mg.NewNode("bbb", nil, "host1")
	edge := mg.NewEdge("ccc", node1, node2, nil, "host1")

	g.NodeAdded(node1)
	g.NodeAdded(node2)
	g.EdgeAdded(edge)
	for i := 1; i < entriesLimit; i++ {
		time.Sleep(3 * time.Second)
		g.SetMetadata(edge, Metadata{"Temp": i})
	}
	time.Sleep(5 * time.Second)

	activeEdges := len(backend.GetEdges(GraphContext{nil, false}, nil))
	time.Sleep(5 * time.Second)
	if activeEdges != 1 {
		t.Fatalf("Found %d active edges instead of 1", activeEdges)
	}

	if err := delTestIndex(name); err != nil {
		t.Fatalf("Failed to clear test indices: %s", err.Error())
	}
}
