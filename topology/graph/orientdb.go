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

// OrientDBBackend describes an OrientDB backend
type OrientDBBackend struct {
	GraphBackend
	client orientdb.ClientInterface
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
		fmt.Sprintf("UpdatedAt = %d", common.UnixMillis(e.updatedAt)),
		fmt.Sprintf("Revision = %d", e.revision),
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

func metadataToOrientDBSelectString(m GraphElementMatcher) string {
	if m == nil {
		return ""
	}

	var filter *filters.Filter
	filter, err := m.Filter()
	if err != nil {
		return ""
	}

	return orientdb.FilterToExpression(filter, func(k string) string {
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
	doc["Metadata"] = e.metadata.Clone()
	doc["CreatedAt"] = common.UnixMillis(e.createdAt)
	doc["UpdatedAt"] = common.UnixMillis(e.updatedAt)
	if !e.deletedAt.IsZero() {
		doc["DeletedAt"] = common.UnixMillis(e.deletedAt)
	}
	doc["Revision"] = e.revision
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
	attrs := []string{}
	for _, event := range events {
		attrs = append(attrs, fmt.Sprintf("%s = %d", event.name, common.UnixMillis(event.t)))
	}
	query := fmt.Sprintf("UPDATE %s SET %s WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = '%s'", e, strings.Join(attrs, ", "), id)
	docs, err := o.client.Search(query)
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

func (o *OrientDBBackend) createNode(n *Node) bool {
	doc := graphElementToOrientDBDocument(n.graphElement)
	doc["@class"] = "Node"
	if _, err := o.client.CreateDocument(doc); err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err.Error())
		return false
	}
	return true
}

// NodeAdded add a node in the database
func (o *OrientDBBackend) NodeAdded(n *Node) bool {
	return o.createNode(n)
}

// NodeDeleted delete a node in the database
func (o *OrientDBBackend) NodeDeleted(n *Node) bool {
	return o.updateTimes("Node", string(n.ID), eventTime{"DeletedAt", n.deletedAt}, eventTime{"ArchivedAt", n.deletedAt})
}

// GetNode get a node within a time slice
func (o *OrientDBBackend) GetNode(i Identifier, t *common.TimeSlice) (nodes []*Node) {
	query := fmt.Sprintf("SELECT FROM Node WHERE %s AND ID = '%s' ORDER BY Revision", o.getTimeSliceClause(t), i)
	docs, err := o.client.Search(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving node %s: %s", i, err.Error())
		return
	}
	for _, doc := range docs {
		nodes = append(nodes, orientDBDocumentToNode(doc))
	}
	return
}

// GetNodeEdges returns a list of a node edges within time slice
func (o *OrientDBBackend) GetNodeEdges(n *Node, t *common.TimeSlice, m GraphElementMatcher) (edges []*Edge) {
	query := fmt.Sprintf("SELECT FROM Link WHERE %s AND (Parent = '%s' OR Child = '%s')", o.getTimeSliceClause(t), n.ID, n.ID)
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	docs, err := o.client.Search(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edges for node %s: %s", n.ID, err.Error())
		return nil
	}

	for _, doc := range docs {
		edges = append(edges, orientDBDocumentToEdge(doc))
	}
	return
}

func (o *OrientDBBackend) createEdge(e *Edge) bool {
	fromQuery := fmt.Sprintf("SELECT FROM Node WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = '%s'", e.parent)
	toQuery := fmt.Sprintf("SELECT FROM Node WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = '%s'", e.child)
	setQuery := fmt.Sprintf("%s, Parent = '%s', Child = '%s'", graphElementToOrientDBSetString(e.graphElement), e.parent, e.child)
	query := fmt.Sprintf("CREATE EDGE Link FROM (%s) TO (%s) SET %s RETRY 100 WAIT 20", fromQuery, toQuery, setQuery)
	docs, err := o.client.Search(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding edge %s: %s (sql: %s)", e.ID, err.Error(), query)
		return false
	}
	return len(docs) == 1
}

// EdgeAdded add a node in the database
func (o *OrientDBBackend) EdgeAdded(e *Edge) bool {
	return o.createEdge(e)
}

// EdgeDeleted delete a node in the database
func (o *OrientDBBackend) EdgeDeleted(e *Edge) bool {
	return o.updateTimes("Link", string(e.ID), eventTime{"DeletedAt", e.deletedAt}, eventTime{"ArchivedAt", e.deletedAt})
}

// GetEdge get an edge within a time slice
func (o *OrientDBBackend) GetEdge(i Identifier, t *common.TimeSlice) (edges []*Edge) {
	query := fmt.Sprintf("SELECT FROM Link WHERE %s AND ID = '%s' ORDER BY Revision", o.getTimeSliceClause(t), i)
	docs, err := o.client.Search(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edge %s: %s", i, err.Error())
		return nil
	}
	for _, doc := range docs {
		edges = append(edges, orientDBDocumentToEdge(doc))
	}
	return
}

// GetEdgeNodes returns the parents and child nodes of an edge within time slice, matching metadata
func (o *OrientDBBackend) GetEdgeNodes(e *Edge, t *common.TimeSlice, parentMetadata, childMetadata GraphElementMatcher) (parents []*Node, children []*Node) {
	query := fmt.Sprintf("SELECT FROM Node WHERE %s AND ID in [\"%s\", \"%s\"]", o.getTimeSliceClause(t), e.parent, e.child)
	docs, err := o.client.Search(query)
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

// MetadataUpdated returns true if a metadata has been updated in the database, based on ArchivedAt
func (o *OrientDBBackend) MetadataUpdated(i interface{}) bool {
	success := true

	switch i := i.(type) {
	case *Node:
		if !o.updateTimes("Node", string(i.ID), eventTime{"ArchivedAt", i.updatedAt}) {
			return false
		}

		success = o.createNode(i)
	case *Edge:
		if !o.updateTimes("Link", string(i.ID), eventTime{"ArchivedAt", i.updatedAt}) {
			return false
		}

		success = o.createEdge(i)
	}

	return success
}

func (*OrientDBBackend) getTimeSliceClause(t *common.TimeSlice) string {
	if t == nil {
		return "ArchivedAt is NULL"
	}
	query := fmt.Sprintf("CreatedAt <= %d AND (DeletedAt >= %d OR DeletedAt is NULL)", t.Last, t.Start)
	query += fmt.Sprintf(" AND UpdatedAt <= %d AND (ArchivedAt >= %d OR ArchivedAt is NULL)", t.Last, t.Start)
	return query
}

// GetNodes returns a list of nodes within time slice, matching metadata
func (o *OrientDBBackend) GetNodes(t *common.TimeSlice, m GraphElementMatcher) (nodes []*Node) {
	query := fmt.Sprintf("SELECT FROM Node WHERE %s ", o.getTimeSliceClause(t))
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	query += " ORDER BY UpdatedAt"

	docs, err := o.client.Search(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving nodes: %s (%+v)", err.Error(), docs)
		return
	}

	for _, doc := range docs {
		nodes = append(nodes, orientDBDocumentToNode(doc))
	}

	return
}

// GetEdges returns a list of edges within time slice, matching metadata
func (o *OrientDBBackend) GetEdges(t *common.TimeSlice, m GraphElementMatcher) (edges []*Edge) {
	query := fmt.Sprintf("SELECT FROM Link WHERE %s", o.getTimeSliceClause(t))
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	query += " ORDER BY UpdatedAt"

	docs, err := o.client.Search(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edges: %s (%+v)", err.Error(), docs)
		return
	}

	for _, doc := range docs {
		edges = append(edges, orientDBDocumentToEdge(doc))
	}

	return
}

// WithContext step
func (o *OrientDBBackend) WithContext(graph *Graph, context GraphContext) (*Graph, error) {
	return &Graph{
		backend: graph.backend,
		context: context,
		host:    graph.host,
	}, nil
}

func newOrientDBBackend(client orientdb.ClientInterface) (*OrientDBBackend, error) {
	if _, err := client.GetDocumentClass("Node"); err != nil {
		class := orientdb.ClassDefinition{
			Name:       "Node",
			SuperClass: "V",
			Properties: []orientdb.Property{
				{Name: "ID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Host", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "ArchivedAt", Type: "LONG", NotNull: true, ReadOnly: true},
				{Name: "CreatedAt", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "UpdatedAt", Type: "LONG"},
				{Name: "DeletedAt", Type: "LONG"},
				{Name: "Revision", Type: "LONG", Mandatory: true, NotNull: true},
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
				{Name: "ArchivedAt", Type: "LONG", NotNull: true, ReadOnly: true},
				{Name: "CreatedAt", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "UpdatedAt", Type: "LONG"},
				{Name: "DeletedAt", Type: "LONG"},
				{Name: "Revision", Type: "LONG", Mandatory: true, NotNull: true},
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

// NewOrientDBBackend creates a new graph backend and
// connect to an OrientDB instance
func NewOrientDBBackend(addr string, database string, username string, password string) (*OrientDBBackend, error) {
	client, err := orientdb.NewClient(addr, database, username, password)
	if err != nil {
		return nil, err
	}

	return newOrientDBBackend(client)
}

// NewOrientDBBackendFromConfig creates a new OrientDB database client based on configuration
func NewOrientDBBackendFromConfig() (*OrientDBBackend, error) {
	addr := config.GetString("storage.orientdb.addr")
	database := config.GetString("storage.orientdb.database")
	username := config.GetString("storage.orientdb.username")
	password := config.GetString("storage.orientdb.password")
	return NewOrientDBBackend(addr, database, username, password)
}
