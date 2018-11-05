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
	"fmt"
	"time"

	"github.com/olivere/elastic"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	es "github.com/skydive-project/skydive/storage/elasticsearch"
)

// graphElementMapping  elasticsearch db mapping scheme
const graphElementMapping = `
{
	"dynamic_templates": [
		{
			"strings": {
				"match": "*",
				"match_mapping_type": "string",
				"mapping": {
					"type": "keyword"
				}
			}
		},
		{
			"archivedat": {
				"match": "ArchivedAt",
				"mapping": {
					"type": "date",
					"format": "epoch_millis"
				}
			}
		},
		{
			"updatedat": {
				"match": "UpdatedAt",
				"mapping": {
					"type": "date",
					"format": "epoch_millis"
				}
			}
		},
		{
			"createdat": {
				"match": "CreatedAt",
				"mapping": {
					"type": "date",
					"format": "epoch_millis"
				}
			}
		},
		{
			"deletedat": {
				"match":"DeletedAt",
				"mapping": {
					"type": "date",
					"format": "epoch_millis"
				}
			}
		}
	]
}
`

const (
	nodeType = "node"
	edgeType = "edge"
)

var topologyLiveIndex = es.Index{
	Name:    "topology_live",
	Type:    "graph_element",
	Mapping: graphElementMapping,
}

var topologyArchiveIndex = es.Index{
	Name:      "topology_archive",
	Type:      "graph_element",
	Mapping:   graphElementMapping,
	RollIndex: true,
}

// ElasticSearchBackend describes a presisent backend based on ElasticSearch
type ElasticSearchBackend struct {
	Backend
	client       es.ClientInterface
	prevRevision map[Identifier]*rawData
}

// TimedSearchQuery describes a search query within a time slice and metadata filters
type TimedSearchQuery struct {
	filters.SearchQuery
	TimeFilter     *filters.Filter
	MetadataFilter *filters.Filter
}

// easyjson:json
type rawData struct {
	Type       string `json:"_Type,omitempty"`
	ID         string
	Host       string
	Origin     string
	CreatedAt  int64
	UpdatedAt  int64
	Metadata   json.RawMessage
	Revision   int64
	DeletedAt  int64  `json:"DeletedAt,omitempty"`
	ArchivedAt int64  `json:"ArchivedAt,omitempty"`
	Parent     string `json:"Parent,omitempty"`
	Child      string `json:"Child,omitempty"`
}

func graphElementToRaw(typ string, e *graphElement) (*rawData, error) {
	data, err := json.Marshal(e.metadata)
	if err != nil {
		return nil, fmt.Errorf("Error while adding graph element %s: %s", e.ID, err)
	}

	raw := &rawData{
		Type:      typ,
		ID:        string(e.ID),
		Host:      e.host,
		Origin:    e.origin,
		CreatedAt: common.UnixMillis(e.createdAt),
		UpdatedAt: common.UnixMillis(e.updatedAt),
		Metadata:  json.RawMessage(data),
		Revision:  e.revision,
	}

	if !e.deletedAt.IsZero() {
		raw.DeletedAt = common.UnixMillis(e.deletedAt)
	}

	return raw, nil
}

func nodeToRaw(n *Node) (*rawData, error) {
	return graphElementToRaw(nodeType, &n.graphElement)
}

func edgeToRaw(e *Edge) (*rawData, error) {
	raw, err := graphElementToRaw(edgeType, &e.graphElement)
	if err != nil {
		return nil, err
	}
	raw.Parent = string(e.parent)
	raw.Child = string(e.child)
	return raw, nil
}

func (b *ElasticSearchBackend) archive(raw *rawData, at time.Time) bool {
	raw.ArchivedAt = common.UnixMillis(at)

	data, err := json.Marshal(raw)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding graph element %s: %s", raw.ID, err)
		return false
	}

	if err := b.client.BulkIndex(topologyArchiveIndex, "", json.RawMessage(data)); err != nil {
		logging.GetLogger().Errorf("Error while archiving %v: %s", raw, err)
		return false
	}
	return true
}

func (b *ElasticSearchBackend) hitToNode(source *json.RawMessage, node *Node) error {
	var obj map[string]interface{}
	if err := common.JSONDecode(bytes.NewReader([]byte(*source)), &obj); err != nil {
		return err
	}

	return node.Decode(obj)
}

func (b *ElasticSearchBackend) hitToEdge(source *json.RawMessage, edge *Edge) error {
	var obj map[string]interface{}
	if err := common.JSONDecode(bytes.NewReader([]byte(*source)), &obj); err != nil {
		return err
	}

	return edge.Decode(obj)
}

func (b *ElasticSearchBackend) indexNode(n *Node) bool {
	raw, err := nodeToRaw(n)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err)
		return false
	}

	data, err := json.Marshal(raw)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err)
		return false
	}

	if err := b.client.BulkIndex(topologyLiveIndex, string(n.ID), json.RawMessage(data)); err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err)
		return false
	}
	b.prevRevision[n.ID] = raw

	return true
}

// NodeAdded add a node
func (b *ElasticSearchBackend) NodeAdded(n *Node) bool {
	return b.indexNode(n)
}

// NodeDeleted delete a node
func (b *ElasticSearchBackend) NodeDeleted(n *Node) bool {
	raw, err := nodeToRaw(n)
	if err != nil {
		logging.GetLogger().Errorf("Error while deleting node %s: %s", n.ID, err)
		return false
	}

	success := true
	if !b.archive(raw, n.deletedAt) {
		success = false
	}

	if err := b.client.BulkDelete(topologyLiveIndex, string(n.ID)); err != nil {
		logging.GetLogger().Errorf("Error while deleting node %s: %s", n.ID, err)
		success = false
	}

	delete(b.prevRevision, n.ID)

	return success
}

// GetNode get a node within a time slice
func (b *ElasticSearchBackend) GetNode(i Identifier, t Context) []*Node {
	nodes := b.searchNodes(&TimedSearchQuery{
		SearchQuery: filters.SearchQuery{
			Filter: filters.NewTermStringFilter("ID", string(i)),
			Sort:   true,
			SortBy: "Revision",
		},
		TimeFilter: getTimeFilter(t.TimeSlice),
	})

	if len(nodes) > 1 && t.TimePoint {
		return []*Node{nodes[len(nodes)-1]}
	}

	return nodes
}

func (b *ElasticSearchBackend) indexEdge(e *Edge) bool {
	raw, err := edgeToRaw(e)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding edge %s: %s", e.ID, err)
		return false
	}

	data, err := json.Marshal(raw)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding edge %s: %s", e.ID, err)
		return false
	}

	if err := b.client.BulkIndex(topologyLiveIndex, string(e.ID), json.RawMessage(data)); err != nil {
		logging.GetLogger().Errorf("Error while indexing edge %s: %s", e.ID, err)
		return false
	}
	b.prevRevision[e.ID] = raw

	return true
}

// EdgeAdded add an edge in the database
func (b *ElasticSearchBackend) EdgeAdded(e *Edge) bool {
	return b.indexEdge(e)
}

// EdgeDeleted delete an edge in the database
func (b *ElasticSearchBackend) EdgeDeleted(e *Edge) bool {
	raw, err := edgeToRaw(e)
	if err != nil {
		logging.GetLogger().Errorf("Error while deleting edge %s: %s", e.ID, err)
		return false
	}

	success := true
	if !b.archive(raw, e.deletedAt) {
		success = false
	}

	if err := b.client.BulkDelete(topologyLiveIndex, string(e.ID)); err != nil {
		logging.GetLogger().Errorf("Error while deleting edge %s: %s", e.ID, err)
		success = false
	}

	delete(b.prevRevision, e.ID)

	return success
}

// GetEdge get an edge within a time slice
func (b *ElasticSearchBackend) GetEdge(i Identifier, t Context) []*Edge {
	edges := b.searchEdges(&TimedSearchQuery{
		SearchQuery: filters.SearchQuery{
			Filter: filters.NewTermStringFilter("ID", string(i)),
			Sort:   true,
			SortBy: "Revision",
		},
		TimeFilter: getTimeFilter(t.TimeSlice),
	})

	if len(edges) > 1 && t.TimePoint {
		return []*Edge{edges[len(edges)-1]}
	}

	return edges
}

// MetadataUpdated updates a node metadata in the database
func (b *ElasticSearchBackend) MetadataUpdated(i interface{}) bool {
	success := true

	switch i := i.(type) {
	case *Node:
		obj := b.prevRevision[i.ID]
		if obj == nil {
			logging.GetLogger().Errorf("Unable to update an unkwown node: %s", i.ID)
			return false
		}

		if !b.archive(obj, i.updatedAt) {
			return false
		}

		success = b.indexNode(i)
	case *Edge:
		obj := b.prevRevision[i.ID]
		if obj == nil {
			logging.GetLogger().Errorf("Unable to update an unkwown edge: %s", i.ID)
			return false
		}

		if !b.archive(obj, i.updatedAt) {
			return false
		}

		success = b.indexEdge(i)
	}

	return success
}

// Query the database for a "node" or "edge"
func (b *ElasticSearchBackend) Query(typ string, tsq *TimedSearchQuery) (sr *elastic.SearchResult, _ error) {
	fltrs := []elastic.Query{
		es.FormatFilter(filters.NewTermStringFilter("_Type", typ), ""),
	}

	if tf := es.FormatFilter(tsq.TimeFilter, ""); tf != nil {
		fltrs = append(fltrs, tf)
	}

	if f := es.FormatFilter(tsq.Filter, ""); f != nil {
		fltrs = append(fltrs, f)
	}

	if mf := es.FormatFilter(tsq.MetadataFilter, "Metadata"); mf != nil {
		fltrs = append(fltrs, mf)
	}

	mustQuery := elastic.NewBoolQuery().Must(fltrs...)

	return b.client.Search("graph_element", mustQuery, tsq.SearchQuery, topologyLiveIndex.Alias(), topologyArchiveIndex.IndexWildcard())
}

// searchNodes search nodes matching the query
func (b *ElasticSearchBackend) searchNodes(tsq *TimedSearchQuery) (nodes []*Node) {
	out, err := b.Query(nodeType, tsq)
	if err != nil {
		logging.GetLogger().Errorf("Failed to query nodes: %s", err)
		return
	}

	if out != nil && len(out.Hits.Hits) > 0 {
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

// searchEdges search edges matching the query
func (b *ElasticSearchBackend) searchEdges(tsq *TimedSearchQuery) (edges []*Edge) {
	out, err := b.Query(edgeType, tsq)
	if err != nil {
		logging.GetLogger().Errorf("Failed to query edges: %s", err)
		return
	}

	if out != nil && len(out.Hits.Hits) > 0 {
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

// GetEdges returns a list of edges within time slice, matching metadata
func (b *ElasticSearchBackend) GetEdges(t Context, m ElementMatcher) []*Edge {
	var filter *filters.Filter
	if m != nil {
		f, err := m.Filter()
		if err != nil {
			return []*Edge{}
		}
		filter = f
	}

	var searchQuery filters.SearchQuery
	if !t.TimePoint {
		searchQuery = filters.SearchQuery{Sort: true, SortBy: "UpdatedAt"}
	}

	edges := b.searchEdges(&TimedSearchQuery{
		SearchQuery:    searchQuery,
		TimeFilter:     getTimeFilter(t.TimeSlice),
		MetadataFilter: filter,
	})

	if t.TimePoint {
		edges = dedupEdges(edges)
	}

	return edges
}

// GetNodes returns a list of nodes within time slice, matching metadata
func (b *ElasticSearchBackend) GetNodes(t Context, m ElementMatcher) []*Node {
	var filter *filters.Filter
	if m != nil {
		f, err := m.Filter()
		if err != nil {
			return []*Node{}
		}
		filter = f
	}

	var searchQuery filters.SearchQuery
	if !t.TimePoint {
		searchQuery = filters.SearchQuery{Sort: true, SortBy: "UpdatedAt"}
	}

	nodes := b.searchNodes(&TimedSearchQuery{
		SearchQuery:    searchQuery,
		TimeFilter:     getTimeFilter(t.TimeSlice),
		MetadataFilter: filter,
	})

	if len(nodes) > 1 && t.TimePoint {
		nodes = dedupNodes(nodes)
	}

	return nodes
}

// GetEdgeNodes returns the parents and child nodes of an edge within time slice, matching metadatas
func (b *ElasticSearchBackend) GetEdgeNodes(e *Edge, t Context, parentMetadata, childMetadata ElementMatcher) (parents []*Node, children []*Node) {
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

// GetNodeEdges returns a list of a node edges within time slice
func (b *ElasticSearchBackend) GetNodeEdges(n *Node, t Context, m ElementMatcher) (edges []*Edge) {
	var filter *filters.Filter
	if m != nil {
		f, err := m.Filter()
		if err != nil {
			return []*Edge{}
		}
		filter = f
	}

	var searchQuery filters.SearchQuery
	if !t.TimePoint {
		searchQuery = filters.SearchQuery{Sort: true, SortBy: "UpdatedAt"}
	}
	searchQuery.Filter = NewFilterForEdge(n.ID, n.ID)

	edges = b.searchEdges(&TimedSearchQuery{
		SearchQuery:    searchQuery,
		TimeFilter:     getTimeFilter(t.TimeSlice),
		MetadataFilter: filter,
	})

	if len(edges) > 1 && t.TimePoint {
		edges = dedupEdges(edges)
	}

	return
}

// IsHistorySupported returns that this backend does support history
func (b *ElasticSearchBackend) IsHistorySupported() bool {
	return true
}

// NewElasticSearchBackendFromClient creates a new graph backend using the given elasticsearch
// client connection
func NewElasticSearchBackendFromClient(client es.ClientInterface) (*ElasticSearchBackend, error) {
	client.Start()

	return &ElasticSearchBackend{
		client:       client,
		prevRevision: make(map[Identifier]*rawData),
	}, nil
}

// NewElasticSearchBackendFromConfig creates a new graph backend based on configuration file parameters
func NewElasticSearchBackendFromConfig(backend string, etcdClient *etcd.Client) (*ElasticSearchBackend, error) {
	cfg := es.NewConfig(backend)

	indices := []es.Index{
		topologyLiveIndex,
		topologyArchiveIndex,
	}

	client, err := es.NewClient(indices, cfg, etcdClient)
	if err != nil {
		return nil, err
	}

	return NewElasticSearchBackendFromClient(client)
}
