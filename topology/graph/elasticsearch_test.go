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
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	elastigo "github.com/mattbaird/elastigo/lib"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/storage/elasticsearch"
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

func (f *fakeElasticsearchClient) Index(obj string, id string, data interface{}) error {
	f.revisions[id] = data
	return nil
}
func (f *fakeElasticsearchClient) BulkIndex(obj string, id string, data interface{}) error {
	f.revisions[id] = data
	return nil
}
func (f *fakeElasticsearchClient) IndexChild(obj string, parent string, id string, data interface{}) error {
	return nil
}
func (f *fakeElasticsearchClient) BulkIndexChild(obj string, parent string, id string, data interface{}) error {
	return nil
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
func (f *fakeElasticsearchClient) Search(obj string, query string) (elastigo.SearchResult, error) {
	f.searches = append(f.searches, query)
	return f.searchResult, nil
}
func (f *fakeElasticsearchClient) Start(mappings []map[string][]byte) {
}

func newElasticsearchGraph(t *testing.T) (*Graph, *fakeElasticsearchClient) {
	client := &fakeElasticsearchClient{
		revisions: make(map[string]interface{}),
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
		"sort": map[string]interface{}{
			"Revision": map[string]interface{}{
				"order":         "asc",
				"unmapped_type": "date",
			},
		},
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
					map[string]interface{}{
						"bool": map[string]interface{}{
							"must": []interface{}{},
						},
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(searchExpected, searchEdge) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %s\ngot: %s", spew.Sdump(searchEdge), spew.Sdump(searchExpected))
	}

	client.resetRevisions()

	node1 := newNode("aaa", Metadata{"MTU": 1500}, time.Unix(1, 0), "host1")
	node2 := newNode("bbb", Metadata{"MTU": 1500}, time.Unix(1, 0), "host1")

	edge := g.newEdge("eee", node1, node2, Metadata{"Name": "eee"}, time.Unix(1, 0), "host1")
	g.addMetadata(edge, "Type", "veth", time.Unix(2, 0))

	expected = []interface{}{
		map[string]interface{}{
			"ArchivedAt": int64(2000),
			"CreatedAt":  int64(1000),
			"Host":       "host1",
			"ID":         "eee",
			"Parent":     Identifier("aaa"),
			"Child":      Identifier("bbb"),
			"Metadata": Metadata{
				"Name": "eee",
			},
			"Revision":  int64(1),
			"UpdatedAt": int64(1000),
		},
		map[string]interface{}{
			"CreatedAt": int64(1000),
			"Host":      "host1",
			"ID":        "eee",
			"Parent":    Identifier("aaa"),
			"Child":     Identifier("bbb"),
			"Metadata": Metadata{
				"Name": "eee",
				"Type": "veth",
			},
			"Revision":  int64(2),
			"UpdatedAt": int64(2000),
		},
	}

	if !reflect.DeepEqual(client.getRevisions(), expected) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %v\ngot: %v", expected, client.getRevisions())
	}
}

// test history when doing local modification
func TestElasticsearchForwarded(t *testing.T) {
	g, client := newElasticsearchGraph(t)
	mg := newGraph(t)

	node := mg.NewNode("aaa", nil, "host1")
	g.NodeAdded(node)
	updatedAt1 := node.updatedAt

	expected := []interface{}{
		map[string]interface{}{
			"UpdatedAt": common.UnixMillis(updatedAt1),
			"CreatedAt": common.UnixMillis(node.createdAt),
			"Host":      "host1",
			"ID":        "aaa",
			"Metadata":  Metadata{},
			"Revision":  int64(1),
		},
	}

	if !reflect.DeepEqual(client.getRevisions(), expected) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %v\ngot: %v", expected, client.getRevisions())
	}

	mg.AddMetadata(node, "MTU", 1500)
	updatedAt2 := node.updatedAt

	client.searchResult.Hits.Hits = []elastigo.Hit{
		{Source: node.JSONRawMessage()},
	}
	g.NodeUpdated(node)

	expected = []interface{}{
		map[string]interface{}{
			"ArchivedAt": common.UnixMillis(updatedAt2),
			"UpdatedAt":  common.UnixMillis(updatedAt1),
			"CreatedAt":  common.UnixMillis(node.createdAt),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata":   Metadata{},
			"Revision":   int64(1),
		},
		map[string]interface{}{
			"UpdatedAt": common.UnixMillis(updatedAt2),
			"CreatedAt": common.UnixMillis(node.createdAt),
			"Host":      "host1",
			"ID":        "aaa",
			"Metadata": Metadata{
				"MTU": 1500,
			},
			"Revision": int64(2),
		},
	}

	if !reflect.DeepEqual(client.getRevisions(), expected) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %v\ngot: %v", expected, client.getRevisions())
	}

	mg.AddMetadata(node, "MTU", 1510)
	updatedAt3 := node.updatedAt

	client.searchResult.Hits.Hits = []elastigo.Hit{
		{Source: node.JSONRawMessage()},
	}
	g.NodeUpdated(node)

	expected = []interface{}{
		map[string]interface{}{
			"ArchivedAt": common.UnixMillis(updatedAt2),
			"UpdatedAt":  common.UnixMillis(updatedAt1),
			"CreatedAt":  common.UnixMillis(node.createdAt),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata":   Metadata{},
			"Revision":   int64(1),
		},
		map[string]interface{}{
			"ArchivedAt": common.UnixMillis(updatedAt3),
			"UpdatedAt":  common.UnixMillis(updatedAt2),
			"CreatedAt":  common.UnixMillis(node.createdAt),
			"Host":       "host1",
			"ID":         "aaa",
			"Metadata": Metadata{
				"MTU": 1500,
			},
			"Revision": int64(2),
		},
		map[string]interface{}{
			"UpdatedAt": common.UnixMillis(updatedAt3),
			"CreatedAt": common.UnixMillis(node.createdAt),
			"Host":      "host1",
			"ID":        "aaa",
			"Metadata": Metadata{
				"MTU": 1510,
			},
			"Revision": int64(3),
		},
	}

	if !reflect.DeepEqual(client.getRevisions(), expected) {
		t.Fatalf("Expected elasticsearch records not found: \nexpected: %v\ngot: %v", expected, client.getRevisions())
	}
}
