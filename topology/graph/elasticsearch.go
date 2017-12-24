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
	"strconv"
	"strings"
	"time"

	"github.com/mattbaird/elastigo/lib"

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
			"archivedat": {
				"match": "ArchivedAt",
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

// ErrBadConfig elasticsearch configuration file is incorrect
var ErrBadConfig = errors.New("elasticsearch : Config file is misconfigured, check elasticsearch key format")

type StorageCmd struct {
	isUpdate bool
	obj      string
	id       string
	data     interface{}
}

// ElasticSearchBackend describes a presisent backend based on ElasticSearch
type ElasticSearchBackend struct {
	GraphBackend
	client       		elasticsearch.ElasticSearchClientInterface
	prevRevision 		map[Identifier]int64
	bulkInsert   		int
	bulkInsertMax		int
	bulInsertDeadline 	time.Duration
	ch           		chan *StorageCmd
	quit         		chan struct{}
}

// TimedSearchQuery describes a search query within a time slice and metadata filters
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
		"UpdatedAt": common.UnixMillis(e.updatedAt),
		"Metadata":  e.metadata.Clone(),
		"Revision":  e.revision,
	}

	if !e.deletedAt.IsZero() {
		obj["DeletedAt"] = common.UnixMillis(e.deletedAt)
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

func (b *ElasticSearchBackend) updateTimes(i interface{}) bool {
	obj := make(map[string]interface{})
	var id, kind string
	switch i := i.(type) {
	case *Node:
		kind = "node"

		revision, ok := b.prevRevision[i.ID]
		if !ok {
			logging.GetLogger().Errorf("Update from an unknow revision, node: %s", i.ID)
			return false
		}
		id = string(i.ID) + "-" + strconv.FormatInt(revision, 10)

		obj["ArchivedAt"] = common.UnixMillis(i.updatedAt)
	case *Edge:
		kind = "edge"

		revision, ok := b.prevRevision[i.ID]
		if !ok {
			logging.GetLogger().Errorf("Update from an unknow revision, edge: %s", i.ID)
			return false
		}
		id = string(i.ID) + "-" + strconv.FormatInt(revision, 10)

		obj["ArchivedAt"] = common.UnixMillis(i.updatedAt)
	}

	b.ch <- &StorageCmd{isUpdate:true, obj:kind,id:id, data:obj}

	return true
}

func (b *ElasticSearchBackend) hitToNode(source *json.RawMessage, node *Node) error {
	var obj map[string]interface{}
	if err := common.JSONDecode(bytes.NewReader([]byte(*source)), &obj); err != nil {
		return err
	}
	if err := node.Decode(obj); err != nil {
		return err
	}
	return nil
}

func (b *ElasticSearchBackend) hitToEdge(source *json.RawMessage, edge *Edge) error {
	var obj map[string]interface{}
	if err := common.JSONDecode(bytes.NewReader([]byte(*source)), &obj); err != nil {
		return err
	}
	if err := edge.Decode(obj); err != nil {
		return err
	}
	return nil
}

func (b *ElasticSearchBackend) getTimeFilter(t *common.TimeSlice) *filters.Filter {
	if t == nil {
		return filters.NewNullFilter("ArchivedAt")
	}

	return filters.NewAndFilter(
		NewFilterForTimeSlice(t),
		filters.NewAndFilter(
			filters.NewLteInt64Filter("UpdatedAt", t.Last),
			filters.NewOrFilter(
				filters.NewNullFilter("ArchivedAt"),
				filters.NewGtInt64Filter("ArchivedAt", t.Start),
			),
		),
	)
}

func (b *ElasticSearchBackend) createNode(n *Node) bool {
	obj := b.mapNode(n)

	id := string(n.ID) + "-" + strconv.FormatInt(n.revision, 10)

	b.ch <- &StorageCmd{isUpdate:false, obj:"node",id:id, data:obj}

	b.prevRevision[n.ID] = n.revision

	return true
}

// NodeAdded add a node
func (b *ElasticSearchBackend) NodeAdded(n *Node) bool {
	return b.createNode(n)
}

// NodeDeleted delete a node
func (b *ElasticSearchBackend) NodeDeleted(n *Node) bool {
	delete(b.prevRevision, n.ID)

	ms := common.UnixMillis(n.deletedAt)
	obj := map[string]interface{}{"DeletedAt": ms, "ArchivedAt": ms}

	id := string(n.ID) + "-" + strconv.FormatInt(n.revision, 10)

	b.ch <- &StorageCmd{isUpdate:true, obj:"node", id:id, data:obj}

	return true
}

// GetNode get a node within a time slice
func (b *ElasticSearchBackend) GetNode(i Identifier, t *common.TimeSlice) []*Node {
	return b.SearchNodes(&TimedSearchQuery{
		SearchQuery: filters.SearchQuery{
			Filter: filters.NewFilterForIds([]string{string(i)}, "ID"),
			Sort:   true,
			SortBy: "Revision",
		},
		TimeFilter: b.getTimeFilter(t),
	})
}

func (b *ElasticSearchBackend) createEdge(e *Edge) bool {
	obj := b.mapEdge(e)

	id := string(e.ID) + "-" + strconv.FormatInt(e.revision, 10)

	b.ch <- &StorageCmd{isUpdate:false, obj:"edge",id:id, data:obj}
	b.prevRevision[e.ID] = e.revision

	return true
}

// EdgeAdded add an edge in the database
func (b *ElasticSearchBackend) EdgeAdded(e *Edge) bool {
	return b.createEdge(e)
}

// EdgeDeleted delete an edge in the database
func (b *ElasticSearchBackend) EdgeDeleted(e *Edge) bool {
	delete(b.prevRevision, e.ID)

	ms := common.UnixMillis(e.deletedAt)
	obj := map[string]interface{}{"DeletedAt": ms, "ArchivedAt": ms}

	id := string(e.ID) + "-" + strconv.FormatInt(e.revision, 10)

	b.ch <- &StorageCmd{isUpdate:true, obj:"edge", id:id, data:obj}

	return true
}

// GetEdge get an edge within a time slice
func (b *ElasticSearchBackend) GetEdge(i Identifier, t *common.TimeSlice) []*Edge {
	return b.SearchEdges(&TimedSearchQuery{
		SearchQuery: filters.SearchQuery{
			Filter: filters.NewFilterForIds([]string{string(i)}, "ID"),
			Sort:   true,
			SortBy: "Revision",
		},
		TimeFilter: b.getTimeFilter(t),
	})
}

// MetadataUpdated updates a node metadata in the database
func (b *ElasticSearchBackend) MetadataUpdated(i interface{}) bool {
	if !b.updateTimes(i) {
		return false
	}

	success := true
	switch i := i.(type) {
	case *Node:
		success = b.createNode(i)
	case *Edge:
		success = b.createEdge(i)
	}

	return success
}

// Query the database for a "node" or "edge"
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
				b.client.FormatFilter(tsq.TimeFilter, ""),
				b.client.FormatFilter(tsq.Filter, ""),
				b.client.FormatFilter(tsq.MetadataFilter, "Metadata"),
			},
		},
	}

	if tsq.Sort {
		sortOrder := tsq.SortOrder
		if sortOrder == "" {
			sortOrder = "asc"
		}

		request["sort"] = map[string]interface{}{
			tsq.SortBy: map[string]string{
				"order":         strings.ToLower(sortOrder),
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

// SearchNodes search nodes matching the query
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

// SearchEdges search edges matching the query
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

// GetEdges returns a list of edges within time slice, matching metadata
func (b *ElasticSearchBackend) GetEdges(t *common.TimeSlice, m Metadata) []*Edge {
	filter, err := NewFilterForMetadata(m)
	if err != nil {
		return []*Edge{}
	}

	return b.SearchEdges(&TimedSearchQuery{
		SearchQuery:    filters.SearchQuery{Sort: true, SortBy: "UpdatedAt"},
		TimeFilter:     NewFilterForTimeSlice(t),
		MetadataFilter: filter,
	})
}

// GetNodes returns a list of nodes within time slice, matching metadata
func (b *ElasticSearchBackend) GetNodes(t *common.TimeSlice, m Metadata) []*Node {
	filter, err := NewFilterForMetadata(m)
	if err != nil {
		return []*Node{}
	}

	return b.SearchNodes(&TimedSearchQuery{
		SearchQuery:    filters.SearchQuery{Sort: true, SortBy: "UpdatedAt"},
		TimeFilter:     b.getTimeFilter(t),
		MetadataFilter: filter,
	})
}

// GetEdgeNodes returns the parents and child nodes of an edge within time slice, matching metadatas
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

// GetNodeEdges returns a list of a node edges within time slice
func (b *ElasticSearchBackend) GetNodeEdges(n *Node, t *common.TimeSlice, m Metadata) (edges []*Edge) {
	metadataFilter, err := NewFilterForMetadata(m)
	if err != nil {
		return
	}

	return b.SearchEdges(&TimedSearchQuery{
		SearchQuery: filters.SearchQuery{
			Filter: NewFilterForEdge(n.ID, n.ID),
			Sort:   true,
			SortBy: "Revision",
		},
		TimeFilter:     b.getTimeFilter(t),
		MetadataFilter: metadataFilter,
	})
}

// WithContext step
func (b *ElasticSearchBackend) WithContext(graph *Graph, context GraphContext) (*Graph, error) {
	return &Graph{
		backend: graph.backend,
		context: context,
		host:    graph.host,
	}, nil
}

func (b *ElasticSearchBackend) runStorageCmds(cmds []*StorageCmd) {
	for _, cmd := range cmds {
		if cmd.isUpdate {
			if err := b.client.BulkUpdateWithPartialDoc(cmd.obj, cmd.id, cmd.data); err != nil {
				logging.GetLogger().Errorf("Error while archiving %s %s: %s", cmd.obj, cmd.id, err.Error())
			}
		} else {
			if err := b.client.BulkIndex(cmd.obj, cmd.id, cmd.data); err != nil {
				logging.GetLogger().Errorf("Error while archiving %s %s: %s", cmd.obj, cmd.id, err.Error())
			}
		}
	}
}

func (b *ElasticSearchBackend) Start() {
	go func() {
		dlTimer := time.NewTicker(b.bulInsertDeadline)
		defer dlTimer.Stop()

		var cmdBuffer []*StorageCmd
		defer b.runStorageCmds(cmdBuffer)
		for {
			select {
			case <-b.quit:
				return
			case <-dlTimer.C:
				b.runStorageCmds(cmdBuffer)
				cmdBuffer = cmdBuffer[:0]
			case cmd := <-b.ch:
				cmdBuffer = append(cmdBuffer, cmd)
				if len(cmdBuffer) >= b.bulkInsertMax {
					logging.GetLogger().Errorf("Buffer overflow - too many topology updates, skipping %d messages", len(cmdBuffer))
					cmdBuffer = cmdBuffer[:0]
				} else if len(cmdBuffer) >= b.bulkInsert {
					b.runStorageCmds(cmdBuffer)
					cmdBuffer = cmdBuffer[:0]
				}
			}
		}
	}()
}

func newElasticSearchBackend(client elasticsearch.ElasticSearchClientInterface, bulkInsert int, bulkInsertMax int, bulkDeadLine int) (*ElasticSearchBackend, error) {
	client.Start([]map[string][]byte{
		{"node": []byte(graphElementMapping)},
		{"edge": []byte(graphElementMapping)},
	})

	return &ElasticSearchBackend{
		client:       		client,
		prevRevision: 		make(map[Identifier]int64),
		bulkInsert: 		bulkInsert,
		bulkInsertMax: 		bulkInsertMax,
		bulInsertDeadline: 	time.Duration(time.Duration(bulkDeadLine) * time.Second),
		ch: 			make(chan *StorageCmd, bulkInsertMax*2),
		quit: 			make(chan struct{}, 2),
	}, nil
}

// NewElasticSearchBackend creates a new graph backend and connect to an ElasticSearch database
func NewElasticSearchBackend(addr string, port string, maxConns int, retrySeconds int, bulkMaxDocs int, bulkMaxDelay int) (*ElasticSearchBackend, error) {
	client, err := elasticsearch.NewElasticSearchClient(addr, port, maxConns, retrySeconds, bulkMaxDocs, bulkMaxDelay)
	if err != nil {
		return nil, err
	}
	bulkInsert := config.GetConfig().GetInt("storage.elasticsearch.bulk_topology_update")
	bulkInsertMax := config.GetConfig().GetInt("storage.elasticsearch.bulk_topology_update_max")
	bulkDeadLine := config.GetConfig().GetInt("storage.elasticsearch.topology_update_timer")
	backend, err := newElasticSearchBackend(client, bulkInsert, bulkInsertMax, bulkDeadLine)
	if err != nil {
		return nil, err
	}
	backend.Start()

	return backend, err
}

// NewElasticSearchBackendFromConfig creates a new graph backend based on configuration file parameters
func NewElasticSearchBackendFromConfig() (*ElasticSearchBackend, error) {
	addr := config.GetConfig().GetString("storage.elasticsearch.host")
	c := strings.Split(addr, ":")
	if len(c) != 2 {
		return nil, ErrBadConfig
	}

	maxConns := config.GetConfig().GetInt("storage.elasticsearch.maxconns")
	retrySeconds := config.GetConfig().GetInt("storage.elasticsearch.retry")
	bulkMaxDocs := config.GetConfig().GetInt("storage.elasticsearch.bulk_maxdocs")
	bulkMaxDelay := config.GetConfig().GetInt("storage.elasticsearch.bulk_maxdelay")

	return NewElasticSearchBackend(c[0], c[1], maxConns, retrySeconds, bulkMaxDocs, bulkMaxDelay)
}
