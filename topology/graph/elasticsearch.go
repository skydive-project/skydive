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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/skydive-project/skydive/config"
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
			"createdat": {
				"match": "CreatedAt",
				"mapping": {
					"type":"date",
					"format": "epoch_second"
				}
			}
		},
		{
			"deletedat": {
				"match":"DeletedAt",
				"mapping": {
					"type":"date",
					"format": "epoch_second"
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

func (b *ElasticSearchBackend) getTimedQuery(t *time.Time) []map[string]interface{} {
	var t2 time.Time
	if t != nil {
		t2 = *t
	} else {
		t2 = time.Now()
	}

	return []map[string]interface{}{
		map[string]interface{}{
			"range": map[string]interface{}{
				"CreatedAt": &struct {
					Lte interface{} `json:"lte,omitempty"`
				}{
					Lte: t2.Unix(),
				},
			},
		},
		map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{
							"DeletedAt": 0,
						},
					},
					map[string]interface{}{
						"range": map[string]interface{}{
							"DeletedAt": &struct {
								Gt interface{} `json:"gt,omitempty"`
							}{
								Gt: t2.Unix(),
							},
						},
					},
				},
			},
		},
	}
}

func (b *ElasticSearchBackend) getMapping(e interface{}) string {
	switch e.(type) {
	case *Node:
		return "node"
	case *Edge:
		return "edge"
	}
	return ""
}

func (b *ElasticSearchBackend) createQuery(filters ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": filters,
			},
		},
		"size": 10000,
	}
}

func (b *ElasticSearchBackend) getRequest(i Identifier, t *time.Time) map[string]interface{} {
	idFilter := map[string]interface{}{
		"term": map[string]string{
			"ID": string(i),
		},
	}

	timedQuery := b.getTimedQuery(t)
	timedQuery = append(timedQuery, idFilter)
	return b.createQuery(timedQuery...)
}

func (b *ElasticSearchBackend) getElement(i Identifier, t *time.Time, element interface{}) error {
	var t2 time.Time
	if t != nil {
		t2 = *t
	} else {
		t2 = time.Now()
	}

	request := b.getRequest(i, t)
	q, err := json.Marshal(request)
	if err != nil {
		return err
	}

	kind := b.getMapping(element)
	out, err := b.client.Search(kind, string(q))
	if err != nil {
		return err
	}

	if out.Hits.Len() > 0 {
		for _, d := range out.Hits.Hits {
			switch e := element.(type) {
			case *Node:
				if err := b.hitToNode(d.Source, e); err != nil {
					return err
				}
			case *Edge:
				if err := b.hitToEdge(d.Source, e); err != nil {
					return err
				}
			}
			return nil
		}
	}

	resp, err := b.client.Get(kind, string(i))
	if err != nil {
		return err
	}

	if resp.Found {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(*resp.Source), &obj); err != nil {
			return err
		}

		createdAt, ok := obj["CreatedAt"].(float64)
		if !ok {
			return errors.New("Object has no attribute 'CreatedAt'")
		}

		deletedAt, ok := obj["DeletedAt"].(float64)
		if !ok {
			return errors.New("Object has no attribute 'DeletedAt'")
		}

		if createdAt <= float64(t2.Unix()) && (deletedAt == 0 || deletedAt > float64(t2.Unix())) {
			switch e := element.(type) {
			case *Node:
				e.Decode(obj)
			case *Edge:
				e.Decode(obj)
			}

			return nil
		}
	}

	return fmt.Errorf("No object found %s", string(i))
}

func (b *ElasticSearchBackend) unflattenMetadata(obj map[string]interface{}) {
	metadata := make(map[string]interface{})
	for k, v := range obj {
		if strings.HasPrefix(k, "Metadata/") {
			metadata[k[9:]] = v
			delete(obj, k)
		}
	}
	obj["Metadata"] = metadata
}

func (b *ElasticSearchBackend) hitToNode(source *json.RawMessage, node *Node) error {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(*source), &obj); err != nil {
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
	if err := json.Unmarshal([]byte(*source), &obj); err != nil {
		return err
	}
	b.unflattenMetadata(obj)
	if err := edge.Decode(obj); err != nil {
		return err
	}
	return nil
}

func (b *ElasticSearchBackend) AddNode(n *Node) bool {
	now := time.Now().Unix()
	obj := map[string]interface{}{
		"ID":        string(n.ID),
		"Host":      n.host,
		"CreatedAt": now,
		"DeletedAt": 0,
	}

	for k, v := range n.metadata {
		obj["Metadata/"+k] = v
	}

	if err := b.client.Index("node", string(n.ID), obj); err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err.Error())
		return false
	}

	return true
}

func (b *ElasticSearchBackend) DelNode(n *Node) bool {
	resp, err := b.client.Get("node", string(n.ID))
	if err != nil || !resp.Found {
		return false
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(*resp.Source), &obj); err != nil {
		return false
	}

	obj["DeletedAt"] = time.Now().Unix()
	obj["CreatedAt"] = int64(obj["CreatedAt"].(float64))

	if _, err := b.client.Delete("node", string(n.ID)); err != nil {
		logging.GetLogger().Errorf("Error while deleting node %s: %s", n.ID, err.Error())
	}

	if err := b.client.Index("node", string(GenID()), obj); err != nil {
		logging.GetLogger().Errorf("Error while deleting node %s: %s", n.ID, err.Error())
		return false
	}

	return true
}

func (b *ElasticSearchBackend) GetNode(i Identifier, t *time.Time) *Node {
	var node Node
	if b.getElement(i, t, &node) != nil {
		return nil
	}
	return &node
}

func (b *ElasticSearchBackend) GetNodeEdges(n *Node, t *time.Time) (edges []*Edge) {
	idsFilter := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []map[string]interface{}{
				map[string]interface{}{
					"term": map[string]interface{}{
						"Parent": n.ID,
					},
				},
				map[string]interface{}{
					"term": map[string]interface{}{
						"Child": n.ID,
					},
				},
			},
		},
	}

	timedQuery := b.getTimedQuery(t)
	timedQuery = append(timedQuery, idsFilter)
	query := b.createQuery(timedQuery...)

	q, err := json.Marshal(query)
	if err != nil {
		return
	}

	out, err := b.client.Search("edge", string(q))
	if err != nil {
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

	return edges
}

func (b *ElasticSearchBackend) AddEdge(e *Edge) bool {
	now := time.Now().Unix()
	obj := map[string]interface{}{
		"ID":        string(e.ID),
		"Host":      e.host,
		"CreatedAt": now,
		"DeletedAt": 0,
		"Parent":    e.parent,
		"Child":     e.child,
	}

	for k, v := range e.metadata {
		obj["Metadata/"+k] = v
	}

	if err := b.client.Index("edge", string(e.ID), obj); err != nil {
		logging.GetLogger().Errorf("Error while adding edge %s: %s", e.ID, err.Error())
		return false
	}

	return true
}

func (b *ElasticSearchBackend) DelEdge(e *Edge) bool {
	resp, err := b.client.Get("edge", string(e.ID))
	if err != nil || !resp.Found {
		return false
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(*resp.Source), &obj); err != nil {
		return false
	}

	obj["DeletedAt"] = time.Now().Unix()
	obj["CreatedAt"] = int64(obj["CreatedAt"].(float64))

	if _, err := b.client.Delete("edge", string(e.ID)); err != nil {
		logging.GetLogger().Errorf("Error while deleting edge %s: %s", e.ID, err.Error())
	}

	if err := b.client.Index("edge", string(GenID()), obj); err != nil {
		logging.GetLogger().Errorf("Error while updating edge %s: %s", e.ID, err.Error())
		return false
	}

	return true
}

func (b *ElasticSearchBackend) GetEdge(i Identifier, t *time.Time) *Edge {
	var edge Edge
	if b.getElement(i, t, &edge) != nil {
		return nil
	}
	return &edge
}

func (b *ElasticSearchBackend) GetEdgeNodes(e *Edge, t *time.Time) (*Node, *Node) {
	return b.GetNode(e.parent, t), b.GetNode(e.child, t)
}

func (b *ElasticSearchBackend) updateGraphElement(i interface{}) bool {
	success := true

	switch i.(type) {
	case *Node:
		node := i.(*Node)
		edges := b.GetNodeEdges(node, nil)
		b.DelNode(node)
		b.AddNode(node)
		for _, e := range edges {
			parent, child := b.GetEdgeNodes(e, nil)
			if parent == nil || child == nil {
				continue
			}

			if success = b.DelEdge(e); !success {
				break
			}

			if success = b.AddEdge(e); !success {
				break
			}
		}

	case *Edge:
		edge := i.(*Edge)
		if success = b.DelEdge(edge); !success {
			break
		}

		success = b.AddEdge(edge)
	}

	return success
}

func (b *ElasticSearchBackend) AddMetadata(i interface{}, k string, v interface{}) bool {
	success := b.updateGraphElement(i)
	if !success {
		logging.GetLogger().Errorf("Error while adding metadata")
	}
	return success
}

func (b *ElasticSearchBackend) SetMetadata(i interface{}, m Metadata) bool {
	success := b.updateGraphElement(i)
	if !success {
		logging.GetLogger().Errorf("Error while setting metadata")
	}
	return success
}

func (b *ElasticSearchBackend) GetNodes(t *time.Time) (nodes []*Node) {
	timedQuery := b.getTimedQuery(t)
	query := b.createQuery(timedQuery...)

	q, err := json.Marshal(query)
	if err != nil {
		return
	}

	out, err := b.client.Search("node", string(q))
	if err != nil {
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

	return nodes
}

func (b *ElasticSearchBackend) GetEdges(t *time.Time) (edges []*Edge) {
	timedQuery := b.getTimedQuery(t)
	query := b.createQuery(timedQuery...)

	q, err := json.Marshal(query)
	if err != nil {
		return
	}

	out, err := b.client.Search("edge", string(q))
	if err != nil {
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

	return edges
}

func NewElasticSearchBackend(addr string, port string, maxConns int, retrySeconds int) (*ElasticSearchBackend, error) {
	client, err := elasticsearch.NewElasticSearchClient(addr, port, maxConns, retrySeconds)
	if err != nil {
		return nil, err
	}

	client.Start([]map[string][]byte{
		{"node": []byte(graphElementMapping)},
		{"edge": []byte(graphElementMapping)},
	})

	backend := &ElasticSearchBackend{
		client: client,
	}

	return backend, nil
}

func NewElasticSearchBackendFromConfig() (*ElasticSearchBackend, error) {
	addr := config.GetConfig().GetString("storage.elasticsearch.host")
	c := strings.Split(addr, ":")
	if len(c) != 2 {
		return nil, ErrBadConfig
	}

	maxConns := config.GetConfig().GetInt("storage.elasticsearch.maxconns")
	if maxConns == 0 {
		maxConns = 10
	}

	retrySeconds := config.GetConfig().GetInt("storage.elasticsearch.retry")
	if retrySeconds == 0 {
		retrySeconds = 60
	}

	return NewElasticSearchBackend(c[0], c[1], maxConns, retrySeconds)
}
