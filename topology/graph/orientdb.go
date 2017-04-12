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
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/storage/orientdb"
)

type OrientDBBackend struct {
	client *orientdb.Client
}

type eventTime struct {
	name string
	t    time.Time
}

func graphElementToOrientDBSetString(e graphElement) (s string) {
	properties := []string{
		fmt.Sprintf("ID = \"%s\"", string(e.ID)),
		fmt.Sprintf("Host = \"%s\"", e.host),
		fmt.Sprintf("CreatedAt = %d", common.UnixMillis(e.createdAt)),
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

func metadataToOrientDBSelectString(m Metadata) string {
	metadataFilter, err := NewFilterForMetadata(m)
	if err != nil {
		return ""
	}
	return orientdb.FilterToExpression(metadataFilter, func(k string) string {
		key := "Metadata"
		for _, s := range strings.Split(k, ".") {
			key += "['" + s + "']"
		}
		return key
	})
}

func graphElementToOrientDBDocument(e graphElement) orientdb.Document {
	doc := make(orientdb.Document)
	doc["@class"] = "Node"
	doc["ID"] = e.ID
	doc["Host"] = e.host
	doc["Metadata"] = e.metadata
	doc["CreatedAt"] = common.UnixMillis(e.createdAt)
	if !e.deletedAt.IsZero() {
		doc["DeletedAt"] = common.UnixMillis(e.deletedAt)
	}
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

func (o *OrientDBBackend) updateTimes(e string, id string, events ...eventTime) bool {
	var attrs []string
	for _, event := range events {
		attrs = append(attrs, fmt.Sprintf("%s = %d", event.name, common.UnixMillis(event.t)))
	}
	query := fmt.Sprintf("UPDATE %s SET %s WHERE DeletedAt IS NULL AND UpdatedAt IS NULL AND ID = '%s'", e, strings.Join(attrs, ", "), id)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while deleting %s: %s", id, err.Error())
		return false
	}
	value, ok := docs[0]["value"]
	if !ok {
		return false
	}
	i, err := value.(json.Number).Int64()
	return err == nil && i == 1
}

func (o *OrientDBBackend) createNode(n *Node, t time.Time) bool {
	doc := graphElementToOrientDBDocument(n.graphElement)
	doc["@class"] = "Node"
	doc["Timestamp"] = common.UnixMillis(t)
	_, err := o.client.CreateDocument(doc)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err.Error())
		return false
	}
	return true
}

func (o *OrientDBBackend) AddNode(n *Node) bool {
	return o.createNode(n, n.createdAt)
}

func (o *OrientDBBackend) DelNode(n *Node) bool {
	return o.updateTimes("Node", string(n.ID), eventTime{"DeletedAt", n.deletedAt}, eventTime{"UpdatedAt", n.deletedAt})
}

func (o *OrientDBBackend) GetNode(i Identifier, t *common.TimeSlice) (nodes []*Node) {
	query := fmt.Sprintf("SELECT FROM Node WHERE %s AND ID = '%s' ORDER BY Timestamp", o.getTimeSliceClause(t), i)
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
	query := fmt.Sprintf("SELECT FROM Link WHERE %s AND (Parent = '%s' OR Child = '%s')", o.getTimeSliceClause(t), n.ID, n.ID)
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	query += " ORDER BY Timestamp"
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

func (o *OrientDBBackend) createEdge(e *Edge, t time.Time) bool {
	timeSlice := common.NewTimeSlice(common.UnixMillis(t), common.UnixMillis(t))
	fromQuery := fmt.Sprintf("SELECT FROM Node WHERE %s AND ID = '%s'", o.getTimeSliceClause(timeSlice), e.parent)
	toQuery := fmt.Sprintf("SELECT FROM Node WHERE %s AND ID = '%s'", o.getTimeSliceClause(timeSlice), e.child)
	setQuery := fmt.Sprintf("%s, Parent = '%s', Child = '%s', Timestamp = %d", graphElementToOrientDBSetString(e.graphElement), e.parent, e.child, common.UnixMillis(t))
	query := fmt.Sprintf("CREATE EDGE Link FROM (%s) TO (%s) SET %s RETRY 100 WAIT 20", fromQuery, toQuery, setQuery)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding edge %s: %s (sql: %s)", e.ID, err.Error(), query)
		return false
	}
	return len(docs) == 1
}

func (o *OrientDBBackend) AddEdge(e *Edge) bool {
	return o.createEdge(e, e.createdAt)
}

func (o *OrientDBBackend) DelEdge(e *Edge) bool {
	return o.updateTimes("Link", string(e.ID), eventTime{"DeletedAt", e.deletedAt}, eventTime{"UpdatedAt", e.deletedAt})
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

func (o *OrientDBBackend) updateMetadata(i interface{}, m Metadata, t time.Time) bool {
	switch i := i.(type) {
	case *Node:
		if !o.updateTimes("Node", string(i.ID), eventTime{"UpdatedAt", t}) {
			return false
		}

		var newNode = *i
		newNode.metadata = m
		if !o.createNode(&newNode, t) {
			return false
		}

	case *Edge:
		if !o.updateTimes("Link", string(i.ID), eventTime{"UpdatedAt", t}) {
			return false
		}

		var newEdge = *i
		newEdge.metadata = m
		return o.createEdge(&newEdge, t)
	}

	return true
}

func (o *OrientDBBackend) AddMetadata(i interface{}, k string, v interface{}, t time.Time) bool {
	var m Metadata
	switch e := i.(type) {
	case *Node:
		m = e.Metadata()
	case *Edge:
		m = e.Metadata()
	}

	m[k] = v
	success := o.updateMetadata(i, m, t)
	if !success {
		logging.GetLogger().Errorf("Error while adding metadata")
	}
	return success
}

func (o *OrientDBBackend) SetMetadata(i interface{}, m Metadata, t time.Time) bool {
	success := o.updateMetadata(i, m, t)
	if !success {
		logging.GetLogger().Errorf("Error while setting metadata")
	}
	return success
}

func (*OrientDBBackend) getTimeSliceClause(t *common.TimeSlice) string {
	if t == nil {
		now := common.UnixMillis(time.Now())
		t = common.NewTimeSlice(now, now)
	}
	query := fmt.Sprintf("CreatedAt <= %d AND (DeletedAt > %d OR DeletedAt is NULL)", t.Last, t.Start)
	query += fmt.Sprintf(" AND Timestamp <= %d AND (UpdatedAt > %d OR UpdatedAt is NULL)", t.Last, t.Start)
	return query
}

func (o *OrientDBBackend) GetNodes(t *common.TimeSlice, m Metadata) (nodes []*Node) {
	query := fmt.Sprintf("SELECT FROM Node WHERE %s ", o.getTimeSliceClause(t))
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	query += " ORDER BY Timestamp"

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
	query += " ORDER BY Timestamp"

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
				{Name: "Timestamp", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "CreatedAt", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "UpdatedAt", Type: "LONG"},
				{Name: "DeletedAt", Type: "LONG"},
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
				{Name: "Timestamp", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "CreatedAt", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "UpdatedAt", Type: "LONG"},
				{Name: "DeletedAt", Type: "LONG"},
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
