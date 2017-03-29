/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lebauce/elastigo/lib"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/storage/elasticsearch"
)

const graphElementMapping = `
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
		},
		{
			"timestamp": {
				"match": "Timestamp",
				"mapping": {
					"type":"date",
					"format": "epoch_millis"
				}
			}
		},
		{
			"updatedat": {
				"match": "UpdatedAt",
				"mapping": {
					"type":"date",
					"format": "epoch_millis"
				}
			}
		},
		{
			"createdat": {
				"match": "CreatedAt",
				"mapping": {
					"type":"date",
					"format": "epoch_millis"
				}
			}
		},
		{
			"deletedat": {
				"match":"DeletedAt",
				"mapping": {
					"type":"date",
					"format": "epoch_millis"
				}
			}
		}
	]
}
`

var ErrBadConfig = errors.New("elasticsearch : Config file is misconfigured, check elasticsearch key format")

type ElasticSearchBackend struct {
	client *elasticsearch.ElasticSearchClient
}

type TimedSearchQuery struct {
	filters.SearchQuery
	TimeFilter     *filters.Filter
	MetadataFilter *filters.Filter
}

func (b *ElasticSearchBackend) mapElement(e *graphElement) map[string]interface{} {
	obj := map[string]interface{}{
		"ID":        string(e.ID),
		"Host":      e.host,
		"CreatedAt": common.UnixMillis(e.createdAt),
	}

	if !e.deletedAt.IsZero() {
		obj["DeletedAt"] = common.UnixMillis(e.deletedAt)
	}

	for k, v := range e.metadata {
		obj["Metadata/"+elasticsearch.EscapeField(k)] = v
	}

	return obj
}

func (b *ElasticSearchBackend) mapNode(n *Node) map[string]interface{} {
	return b.mapElement(&n.graphElement)
}

func (b *ElasticSearchBackend) mapEdge(e *Edge) map[string]interface{} {
	obj := b.mapElement(&e.graphElement)
	obj["Parent"] = e.parent
	obj["Child"] = e.child
	return obj
}

func (b *ElasticSearchBackend) getElement(kind string, i Identifier, element interface{}) error {
	resp, err := b.client.Get(kind, string(i))
	if err != nil {
		return err
	}

	if resp.Found {
		switch e := element.(type) {
		case *Node:
			return b.hitToNode(resp.Source, e)
		case *Edge:
			return b.hitToEdge(resp.Source, e)
		}
	}

	return fmt.Errorf("No object found %s", string(i))
}

func (b *ElasticSearchBackend) archiveElement(kind string, i interface{}, t time.Time) bool {
	var obj map[string]interface{}
	var id string
	var timestamp time.Time
	switch i := i.(type) {
	case *Node:
		id = string(i.ID)
		timestamp = i.updatedAt
		obj = b.mapNode(i)
	case *Edge:
		id = string(i.ID)
		timestamp = i.updatedAt
		obj = b.mapEdge(i)
	}

	// Archive the element with a different ES id. As 'Timestamp' is not
	// part of the 'Node' or 'Edge' structures, we need to add it here so that
	// it is set in the archived version
	obj["UpdatedAt"] = common.UnixMillis(t)
	obj["Timestamp"] = common.UnixMillis(timestamp)
	if err := b.client.Index(kind, string(GenID()), obj); err != nil {
		logging.GetLogger().Errorf("Error while archiving %s %s: %s", kind, id, err.Error())
		return false
	}

	return true
}

func (b *ElasticSearchBackend) deleteElement(kind string, id string, t time.Time) bool {
	obj := map[string]interface{}{"DeletedAt": common.UnixMillis(t)}
	if err := b.client.UpdateWithPartialDoc(kind, id, obj); err != nil {
		logging.GetLogger().Errorf("Error while marking %s as deleted %s: %s", kind, id, err.Error())
		return false
	}

	return true
}

func (b *ElasticSearchBackend) unflattenMetadata(obj map[string]interface{}) {
	metadata := make(map[string]interface{})
	for k, v := range obj {
		if strings.HasPrefix(k, "Metadata/") {
			metadata[elasticsearch.UnescapeField(k[9:])] = v
			delete(obj, k)
		}
	}
	obj["Metadata"] = metadata
}

func (b *ElasticSearchBackend) hitToNode(source *json.RawMessage, node *Node) error {
	var obj map[string]interface{}
	if err := common.JsonDecode(bytes.NewReader([]byte(*source)), &obj); err != nil {
		return err
	}
	b.unflattenMetadata(obj)
	if err := node.Decode(obj); err != nil {
		return err
	}
	return nil
}

func (b *ElasticSearchBackend) hitToEdge(source *json.RawMessage, edge *Edge) error {
	var obj map[string]interface{}
	if err := common.JsonDecode(bytes.NewReader([]byte(*source)), &obj); err != nil {
		return err
	}
	b.unflattenMetadata(obj)
	if err := edge.Decode(obj); err != nil {
		return err
	}
	return nil
}

func (b *ElasticSearchBackend) getTimeFilter(t *common.TimeSlice) *filters.Filter {
	if t == nil {
		now := common.UnixMillis(time.Now())
		t = common.NewTimeSlice(now, now)
	}

	return filters.NewAndFilter(
		NewFilterForTimeSlice(t),
		filters.NewAndFilter(
			// Timestamp holds the creation time of the revision
			filters.NewLteInt64Filter("Timestamp", t.Last),
			filters.NewOrFilter(
				// UpdatedAt holds the deletion time of the revision
				filters.NewNullFilter("UpdatedAt"),
				filters.NewGtInt64Filter("UpdatedAt", t.Start),
			),
		),
	)
}

func (b *ElasticSearchBackend) createNode(n *Node, t time.Time) bool {
	obj := b.mapNode(n)
	obj["Timestamp"] = common.UnixMillis(t)
	if err := b.client.Index("node", string(n.ID), obj); err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err.Error())
		return false
	}

	return true
}

func (b *ElasticSearchBackend) AddNode(n *Node) bool {
	return b.createNode(n, n.createdAt)
}

func (b *ElasticSearchBackend) DelNode(n *Node) bool {
	return b.deleteElement("node", string(n.ID), n.deletedAt)
}

func (b *ElasticSearchBackend) GetNode(i Identifier, t *common.TimeSlice) []*Node {
	var node Node
	if t == nil {
		if b.getElement("node", i, &node) != nil {
			return nil
		}
		return []*Node{&node}
	}
	return b.SearchNodes(&TimedSearchQuery{
		SearchQuery: filters.SearchQuery{
			Filter: filters.NewFilterForIds([]string{string(i)}, "ID"),
			Sort:   true,
			SortBy: "Timestamp",
		},
		TimeFilter: b.getTimeFilter(t),
	})
}

func (b *ElasticSearchBackend) createEdge(e *Edge, t time.Time) bool {
	obj := b.mapEdge(e)
	obj["Timestamp"] = common.UnixMillis(t)
	if err := b.client.Index("edge", string(e.ID), obj); err != nil {
		logging.GetLogger().Errorf("Error while adding edge %s: %s", e.ID, err.Error())
		return false
	}

	return true
}

func (b *ElasticSearchBackend) AddEdge(e *Edge) bool {
	return b.createEdge(e, e.createdAt)
}

func (b *ElasticSearchBackend) DelEdge(e *Edge) bool {
	return b.deleteElement("edge", string(e.ID), e.deletedAt)
}

func (b *ElasticSearchBackend) GetEdge(i Identifier, t *common.TimeSlice) []*Edge {
	var edge Edge
	if t == nil {
		if b.getElement("edge", i, &edge) != nil {
			return nil
		}
		return []*Edge{&edge}
	}
	return b.SearchEdges(&TimedSearchQuery{
		SearchQuery: filters.SearchQuery{
			Filter: filters.NewFilterForIds([]string{string(i)}, "ID"),
			Sort:   true,
			SortBy: "Timestamp",
		},
		TimeFilter: b.getTimeFilter(t),
	})
}

func (b *ElasticSearchBackend) updateMetadata(i interface{}, m Metadata, t time.Time) bool {
	success := true

	switch i := i.(type) {
	case *Node:
		if !b.archiveElement("node", i, t) {
			return false
		}

		var newNode = *i
		newNode.metadata = m
		if !b.createNode(&newNode, t) {
			return false
		}

	case *Edge:
		if !b.archiveElement("edge", i, t) {
			return false
		}

		var newEdge = *i
		newEdge.metadata = m
		success = b.createEdge(&newEdge, t)
	}

	return success
}

func (b *ElasticSearchBackend) AddMetadata(i interface{}, k string, v interface{}, t time.Time) bool {
	var m Metadata
	switch e := i.(type) {
	case *Node:
		m = e.Metadata()
	case *Edge:
		m = e.Metadata()
	}

	m[k] = v
	success := b.updateMetadata(i, m, t)
	if !success {
		logging.GetLogger().Errorf("Error while adding metadata")
	}
	return success
}

func (b *ElasticSearchBackend) SetMetadata(i interface{}, m Metadata, t time.Time) bool {
	success := b.updateMetadata(i, m, t)
	if !success {
		logging.GetLogger().Errorf("Error while setting metadata")
	}
	return success
}

func (b *ElasticSearchBackend) Query(obj string, tsq *TimedSearchQuery) (sr elastigo.SearchResult, _ error) {
	if tsq.TimeFilter == nil {
		t := common.UnixMillis(time.Now())
		tsq.TimeFilter = b.getTimeFilter(common.NewTimeSlice(t, t))
	}

	request := map[string]interface{}{"size": 10000}

	if tsq.PaginationRange != nil {
		if tsq.PaginationRange.To < tsq.PaginationRange.From {
			return sr, errors.New("Incorrect PaginationRange, To < From")
		}

		request["from"] = tsq.PaginationRange.From
		request["size"] = tsq.PaginationRange.To - tsq.PaginationRange.From
	}

	request["query"] = map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []map[string]interface{}{
				b.client.FormatFilter(tsq.TimeFilter, "", true),
				b.client.FormatFilter(tsq.Filter, "", true),
				b.client.FormatFilter(tsq.MetadataFilter, "Metadata/", true),
			},
		},
	}

	if tsq.Sort {
		request["sort"] = map[string]interface{}{
			tsq.SortBy: map[string]string{
				"order":         strings.ToLower(tsq.SortOrder),
				"unmapped_type": "date",
			},
		}
	}

	q, err := json.Marshal(request)
	if err != nil {
		return
	}

	return b.client.Search(obj, string(q))
}

func (b *ElasticSearchBackend) SearchNodes(tsq *TimedSearchQuery) (nodes []*Node) {
	out, err := b.Query("node", tsq)
	if err != nil {
		logging.GetLogger().Errorf("Failed to query nodes: %s", err.Error())
		return
	}

	if out.Hits.Len() > 0 {
		for _, d := range out.Hits.Hits {
			var node Node
			if err := b.hitToNode(d.Source, &node); err != nil {
				logging.GetLogger().Debugf("Failed to unmarshal node: %+v", d.Source)
			}
			nodes = append(nodes, &node)
		}
	}

	return
}

func (b *ElasticSearchBackend) SearchEdges(tsq *TimedSearchQuery) (edges []*Edge) {
	out, err := b.Query("edge", tsq)
	if err != nil {
		logging.GetLogger().Errorf("Failed to query edges: %s", err.Error())
		return
	}

	if out.Hits.Len() > 0 {
		for _, d := range out.Hits.Hits {
			var edge Edge
			if err := b.hitToEdge(d.Source, &edge); err != nil {
				logging.GetLogger().Debugf("Failed to unmarshal edge: %+v", d.Source)
			}
			edges = append(edges, &edge)
		}
	}

	return
}

func (b *ElasticSearchBackend) GetEdges(t *common.TimeSlice, m Metadata) []*Edge {
	filter, err := NewFilterForMetadata(m)
	if err != nil {
		return []*Edge{}
	}

	return b.SearchEdges(&TimedSearchQuery{
		SearchQuery:    filters.SearchQuery{Sort: true, SortBy: "Timestamp"},
		TimeFilter:     NewFilterForTimeSlice(t),
		MetadataFilter: filter,
	})
}

func (b *ElasticSearchBackend) GetNodes(t *common.TimeSlice, m Metadata) []*Node {
	filter, err := NewFilterForMetadata(m)
	if err != nil {
		return []*Node{}
	}

	return b.SearchNodes(&TimedSearchQuery{
		SearchQuery:    filters.SearchQuery{Sort: true, SortBy: "Timestamp"},
		TimeFilter:     b.getTimeFilter(t),
		MetadataFilter: filter,
	})
}

func (b *ElasticSearchBackend) GetEdgeNodes(e *Edge, t *common.TimeSlice, parentMetadata, childMetadata Metadata) (parents []*Node, children []*Node) {
	for _, parent := range b.GetNode(e.parent, t) {
		if parent.MatchMetadata(parentMetadata) {
			parents = append(parents, parent)
		}
	}

	for _, child := range b.GetNode(e.child, t) {
		if child.MatchMetadata(childMetadata) {
			children = append(children, child)
		}
	}

	return
}

func (b *ElasticSearchBackend) GetNodeEdges(n *Node, t *common.TimeSlice, m Metadata) (edges []*Edge) {
	metadataFilter, err := NewFilterForMetadata(m)
	if err != nil {
		return
	}

	return b.SearchEdges(&TimedSearchQuery{
		SearchQuery: filters.SearchQuery{
			Filter: NewFilterForEdge(n.ID, n.ID),
			Sort:   true,
			SortBy: "Timestamp",
		},
		TimeFilter:     b.getTimeFilter(t),
		MetadataFilter: metadataFilter,
	})
}

func (b *ElasticSearchBackend) WithContext(graph *Graph, context GraphContext) (*Graph, error) {
	return &Graph{
		backend: graph.backend,
		context: context,
		host:    graph.host,
	}, nil
}

func NewElasticSearchBackend(addr string, port string, maxConns int, retrySeconds int, bulkMaxDocs int) (*ElasticSearchBackend, error) {
	client, err := elasticsearch.NewElasticSearchClient(addr, port, maxConns, retrySeconds, bulkMaxDocs)
	if err != nil {
		return nil, err
	}

	client.Start([]map[string][]byte{
		{"node": []byte(graphElementMapping)},
		{"edge": []byte(graphElementMapping)},
	})

	return &ElasticSearchBackend{
		client: client,
	}, nil
}

func NewElasticSearchBackendFromConfig() (*ElasticSearchBackend, error) {
	addr := config.GetConfig().GetString("storage.elasticsearch.host")
	c := strings.Split(addr, ":")
	if len(c) != 2 {
		return nil, ErrBadConfig
	}

	maxConns := config.GetConfig().GetInt("storage.elasticsearch.maxconns")
	retrySeconds := config.GetConfig().GetInt("storage.elasticsearch.retry")
	bulkMaxDocs := config.GetConfig().GetInt("storage.elasticsearch.bulk_maxdocs")

	return NewElasticSearchBackend(c[0], c[1], maxConns, retrySeconds, bulkMaxDocs)
}
