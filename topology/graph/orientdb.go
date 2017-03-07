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
	"fmt"
	"strings"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/storage/orientdb"
)

type OrientDBBackend struct {
	client *orientdb.Client
}

func graphElementToOrientDBSetString(e graphElement) (s string) {
	properties := []string{
		fmt.Sprintf("ID = \"%s\"", string(e.ID)),
		fmt.Sprintf("Host = \"%s\"", e.host),
	}
	s = strings.Join(properties, ", ")
	if m := metadataToOrientDBSetString(e.metadata); m != "" {
		s += ", " + m
	}
	return
}

func metadataToOrientDBSetString(m Metadata) string {
	if len(m) > 0 {
		b, err := json.Marshal(m)
		if err == nil {
			return "Metadata = " + string(b)
		}
	}
	return ""
}

func metadataFormatter(s string) string {
	return fmt.Sprintf("Metadata['%s']", s)
}

func metadataToOrientDBSelectString(m Metadata) string {
	i := 0
	props := make([]string, len(m))
	for key, value := range m {
		switch v := value.(type) {
		case string:
			props[i] = fmt.Sprintf("%s='%s'\n", metadataFormatter(key), v)
		case *filters.Filter:
			if expr := orientdb.FilterToExpression(v, metadataFormatter); expr != "" {
				props[i] = expr
			}
		default:
			props[i] = fmt.Sprintf("%s=%s", metadataFormatter(key), value)
		}
		i++
	}
	return strings.Join(props, " AND ")
}

func graphElementToOrientDBDocument(e graphElement) orientdb.Document {
	doc := make(orientdb.Document)
	doc["@class"] = "Node"
	doc["ID"] = e.ID
	doc["Host"] = e.host
	doc["Metadata"] = e.metadata
	doc["CreatedAt"] = e.createdAt.UTC().Unix()
	return doc
}

func orientDBDocumentToNode(doc orientdb.Document) *Node {
	n := new(Node)
	n.Decode(map[string]interface{}(doc))
	return n
}

func orientDBDocumentToEdge(doc orientdb.Document) *Edge {
	e := new(Edge)
	e.Decode(map[string]interface{}(doc))
	return e
}

func (o *OrientDBBackend) AddNode(n *Node) bool {
	doc := graphElementToOrientDBDocument(n.graphElement)
	doc["@class"] = "Node"
	doc["CreatedAt"] = n.createdAt.UTC().Unix()
	_, err := o.client.CreateDocument(doc)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err.Error())
		return false
	}
	return true
}

func (o *OrientDBBackend) DelNode(n *Node) bool {
	query := fmt.Sprintf("UPDATE Node SET DeletedAt = %d WHERE DeletedAt IS NULL AND ID = '%s'", n.deletedAt.UTC().Unix(), n.ID)
	docs, err := o.client.Sql(query)
	if err != nil || (err == nil && len(docs) != 1) {
		logging.GetLogger().Errorf("Error while deleting node %s: %s (sql: %s)", n.ID, err.Error(), query)
		return false
	}
	value, ok := docs[0]["value"]
	if !ok {
		return false
	}
	i, err := value.(json.Number).Int64()
	return err == nil && i == 1
}

func (o *OrientDBBackend) GetNode(i Identifier, t *common.TimeSlice) (nodes []*Node) {
	query := fmt.Sprintf("SELECT FROM Node WHERE %s AND ID = '%s' ORDER BY CreatedAt", o.getTimeSliceClause(t), i)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving node %s: %s", i, err.Error())
		return
	}
	for _, doc := range docs {
		nodes = append(nodes, orientDBDocumentToNode(doc))
	}
	return
}

func (o *OrientDBBackend) GetNodeEdges(n *Node, t *common.TimeSlice, m Metadata) (edges []*Edge) {
	query := fmt.Sprintf("SELECT FROM Link WHERE %s AND (Parent = '%s' OR Child = '%s') ORDER BY CreatedAt", o.getTimeSliceClause(t), n.ID, n.ID)
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edges for node %s: %s", n.ID, err.Error())
		return nil
	}

	for _, doc := range docs {
		edges = append(edges, orientDBDocumentToEdge(doc))
	}
	return
}

func (o *OrientDBBackend) AddEdge(e *Edge) bool {
	query := fmt.Sprintf("CREATE EDGE Link FROM (SELECT FROM Node WHERE DeletedAt IS NULL AND ID = '%s') TO (SELECT FROM Node WHERE DeletedAt IS NULL AND ID = '%s') SET %s, Parent = '%s', Child = '%s', CreatedAt = %d RETRY 100 WAIT 20", e.parent, e.child, graphElementToOrientDBSetString(e.graphElement), e.parent, e.child, time.Now().UTC().Unix())
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding edge %s: %s (sql: %s)", e.ID, err.Error(), query)
		return false
	}
	return len(docs) == 1
}

func (o *OrientDBBackend) DelEdge(e *Edge) bool {
	query := fmt.Sprintf("UPDATE Link SET DeletedAt = %d WHERE DeletedAt IS NULL AND ID = '%s'", e.deletedAt.UTC().Unix(), e.ID)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while deleting edge %s: %s", e.ID, err.Error())
		return false
	}
	value, ok := docs[0]["value"]
	if !ok {
		return false
	}
	i, err := value.(json.Number).Int64()
	return err == nil && i == 1
}

func (o *OrientDBBackend) GetEdge(i Identifier, t *common.TimeSlice) (edges []*Edge) {
	query := fmt.Sprintf("SELECT FROM Link WHERE %s AND ID = '%s'", o.getTimeSliceClause(t), i)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edge %s: %s", i, err.Error())
		return nil
	}
	for _, doc := range docs {
		edges = append(edges, orientDBDocumentToEdge(doc))
	}
	return
}

func (o *OrientDBBackend) GetEdgeNodes(e *Edge, t *common.TimeSlice, parentMetadata, childMetadata Metadata) (parents []*Node, children []*Node) {
	query := fmt.Sprintf("SELECT FROM Node WHERE %s AND ID in [\"%s\", \"%s\"]", o.getTimeSliceClause(t), e.parent, e.child)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving nodes for edge %s: %s", e.ID, err.Error())
		return nil, nil
	}

	for _, doc := range docs {
		node := orientDBDocumentToNode(doc)
		if node.ID == e.parent && node.MatchMetadata(parentMetadata) {
			parents = append(parents, node)
		} else if node.MatchMetadata(childMetadata) {
			children = append(children, node)
		}
	}

	return
}

func (o *OrientDBBackend) updateMetadata(i interface{}, m Metadata) bool {
	now := time.Now().UTC()

	switch i.(type) {
	case *Node:
		var oldNode = *i.(*Node)
		edges := o.GetNodeEdges(&oldNode, nil, nil)

		oldNode.deletedAt = now
		if !o.DelNode(&oldNode) {
			return false
		}

		var newNode = oldNode
		newNode.createdAt = now
		newNode.deletedAt = time.Time{}
		newNode.metadata = m
		if !o.AddNode(&newNode) {
			return false
		}

		for _, e := range edges {
			parent, child := o.GetEdgeNodes(e, nil, nil, nil)
			if parent == nil || child == nil {
				continue
			}

			var oldEdge = *e
			oldEdge.deletedAt = now
			if !o.DelEdge(&oldEdge) {
				return false
			}

			var newEdge = *e
			newEdge.createdAt = now
			newEdge.deletedAt = time.Time{}
			if !o.AddEdge(&newEdge) {
				return false
			}
		}

	case *Edge:
		var oldEdge = *i.(*Edge)

		oldEdge.deletedAt = now
		if !o.DelEdge(&oldEdge) {
			return false
		}

		var newEdge = oldEdge
		newEdge.createdAt = now
		newEdge.deletedAt = time.Time{}
		newEdge.metadata = m
		return o.AddEdge(&newEdge)
	}

	return true
}

func (o *OrientDBBackend) AddMetadata(i interface{}, k string, v interface{}) bool {
	var m Metadata
	switch e := i.(type) {
	case *Node:
		m = e.metadata
	case *Edge:
		m = e.metadata
	}

	m[k] = v
	success := o.updateMetadata(i, m)
	if !success {
		logging.GetLogger().Errorf("Error while adding metadata")
	}
	return success
}

func (o *OrientDBBackend) SetMetadata(i interface{}, m Metadata) bool {
	success := o.updateMetadata(i, m)
	if !success {
		logging.GetLogger().Errorf("Error while setting metadata")
	}
	return success
}

func (*OrientDBBackend) getTimeSliceClause(t *common.TimeSlice) string {
	if t == nil {
		now := time.Now().UTC().Unix()
		t = common.NewTimeSlice(now, now)
	}
	return fmt.Sprintf("CreatedAt <= %d AND (DeletedAt > %d OR DeletedAt is NULL)", t.Last, t.Start)
}

func (*OrientDBBackend) getTimeClause(t *time.Time) string {
	var e int64
	if t == nil {
		e = time.Now().UTC().Unix()
	} else {
		e = t.UTC().Unix()
	}
	return fmt.Sprintf("CreatedAt <= %d AND (DeletedAt > %d OR DeletedAt is NULL)", e, e)
}

func (o *OrientDBBackend) GetNodes(t *common.TimeSlice, m Metadata) (nodes []*Node) {
	query := fmt.Sprintf("SELECT FROM Node WHERE %s ", o.getTimeSliceClause(t))
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	query += " ORDER BY CreatedAt"

	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving nodes: %s (%+v)", err.Error(), docs)
		return
	}

	for _, doc := range docs {
		nodes = append(nodes, orientDBDocumentToNode(doc))
	}

	return
}

func (o *OrientDBBackend) GetEdges(t *common.TimeSlice, m Metadata) (edges []*Edge) {
	query := fmt.Sprintf("SELECT FROM Link WHERE %s", o.getTimeSliceClause(t))
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	query += " ORDER BY CreatedAt"

	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edges: %s (%+v)", err.Error(), docs)
		return
	}

	for _, doc := range docs {
		edges = append(edges, orientDBDocumentToEdge(doc))
	}

	return
}

func (o *OrientDBBackend) WithContext(graph *Graph, context GraphContext) (*Graph, error) {
	return &Graph{
		backend: graph.backend,
		context: context,
		host:    graph.host,
	}, nil
}

func NewOrientDBBackend(addr string, database string, username string, password string) (*OrientDBBackend, error) {
	client, err := orientdb.NewClient(addr, database, username, password)
	if err != nil {
		return nil, err
	}

	if _, err := client.GetDocumentClass("Node"); err != nil {
		class := orientdb.ClassDefinition{
			Name:       "Node",
			SuperClass: "V",
			Properties: []orientdb.Property{
				{Name: "ID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Host", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "CreatedAt", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "DeletedAt", Type: "LONG", NotNull: true},
				{Name: "Metadata", Type: "EMBEDDEDMAP"},
			},
			Indexes: []orientdb.Index{
				{Name: "Node.TimeSpan", Fields: []string{"CreatedAt", "DeletedAt"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class Node: %s", err.Error())
		}
	}

	if _, err := client.GetDocumentClass("Link"); err != nil {
		class := orientdb.ClassDefinition{
			Name:       "Link",
			SuperClass: "E",
			Properties: []orientdb.Property{
				{Name: "ID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Host", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "CreatedAt", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "DeletedAt", Type: "LONG", NotNull: true},
				{Name: "Parent", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Child", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Metadata", Type: "EMBEDDEDMAP"},
			},
			Indexes: []orientdb.Index{
				{Name: "Link.TimeSpan", Fields: []string{"CreatedAt", "DeletedAt"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class Link: %s", err.Error())
		}
	}

	return &OrientDBBackend{
		client: client,
	}, nil
}

func NewOrientDBBackendFromConfig() (*OrientDBBackend, error) {
	addr := config.GetConfig().GetString("storage.orientdb.addr")
	database := config.GetConfig().GetString("storage.orientdb.database")
	username := config.GetConfig().GetString("storage.orientdb.username")
	password := config.GetConfig().GetString("storage.orientdb.password")
	return NewOrientDBBackend(addr, database, username, password)
}
