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

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph/gremlin"
	"github.com/redhat-cip/skydive/topology/graph/orientdb"
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
	properties := []string{}
	for k, v := range m {
		encoder := gremlin.GremlinPropertiesEncoder{}
		encoder.Encode(v)
		properties = append(properties, fmt.Sprintf("%s = %s", k, encoder.String()))
	}
	return strings.Join(properties, ", ")
}

func orientDBDocumentToGraphElement(doc orientdb.Document) graphElement {
	element := graphElement{
		metadata: make(Metadata),
	}
	element.ID = Identifier(doc["ID"].(string))
	delete(doc, "ID")
	if host, ok := doc["Host"]; ok {
		element.host = host.(string)
		delete(doc, "Host")
	}
	for field, value := range doc {
		if !strings.HasPrefix(field, "@") && !strings.HasPrefix(field, "_") {
			field = strings.Replace(field, "/", ".", -1)
			if n, ok := value.(json.Number); ok {
				var err error
				if value, err = n.Int64(); err == nil {
					value = int(value.(int64))
				} else {
					value, _ = n.Float64()
				}
			}
			element.metadata[field] = value
		}
	}
	return element
}

func graphElementToOrientDBDocument(e graphElement) orientdb.Document {
	doc := make(orientdb.Document)
	doc["@class"] = "Node"
	doc["ID"] = e.ID
	doc["Host"] = e.host
	for k, v := range e.metadata {
		k = strings.Replace(k, ".", "/", -1)
		doc[k] = v
	}
	return doc
}

func orientDBDocumentToNode(doc orientdb.Document) *Node {
	delete(doc, "in_Link")
	delete(doc, "out_Link")
	return &Node{
		graphElement: orientDBDocumentToGraphElement(doc),
	}
}

func orientDBDocumentToEdge(doc orientdb.Document) *Edge {
	e := &Edge{}

	if parent, ok := doc["parent"]; ok {
		e.parent = Identifier(parent.(string))
		delete(doc, "parent")
	}

	if child, ok := doc["child"]; ok {
		e.child = Identifier(child.(string))
		delete(doc, "child")
	}

	delete(doc, "in")
	delete(doc, "out")

	e.graphElement = orientDBDocumentToGraphElement(doc)

	return e
}

func (o *OrientDBBackend) AddNode(n *Node) bool {
	doc := graphElementToOrientDBDocument(n.graphElement)
	doc["@class"] = "Node"
	_, err := o.client.CreateDocument(doc)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding node %s: %s", n.ID, err.Error())
		return false
	}
	return true
}

func (o *OrientDBBackend) DelNode(n *Node) bool {
	query := fmt.Sprintf("DELETE VERTEX Node WHERE ID = \"%s\"", n.ID)
	docs, err := o.client.Sql(query)
	if err != nil || (err == nil && len(docs) != 1) {
		logging.GetLogger().Errorf("Error while deleting node %s: %s", n.ID, err.Error())
		return false
	}
	value, ok := docs[0]["value"]
	if !ok {
		return false
	}
	i, err := value.(json.Number).Int64()
	return err != nil && i == 1
}

func (o *OrientDBBackend) GetNode(i Identifier) *Node {
	query := fmt.Sprintf("SELECT expand(rid) FROM INDEX:Node.ID WHERE key = \"%s\"", i)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving node %s: %s", i, err.Error())
		return nil
	}
	if len(docs) != 0 {
		return orientDBDocumentToNode(docs[0])
	}
	return nil
}

func (o *OrientDBBackend) GetNodeEdges(n *Node) (edges []*Edge) {
	query := fmt.Sprintf("SELECT expand(rid.bothe()) FROM INDEX:Node.ID WHERE key = \"%s\"", n.ID)
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
	query := fmt.Sprintf("CREATE EDGE Link FROM (SELECT FROM Node WHERE DeletedAt IS NULL AND ID = '%s') TO (SELECT FROM Node WHERE DeletedAt IS NULL AND ID = '%s') SET %s, parent = '%s', child = '%s', CreatedAt = '%s'", e.parent, e.child, graphElementToOrientDBSetString(e.graphElement), e.parent, e.child, time.Now().String())
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while adding edge %s: %s (sql: %s)", e.ID, err.Error(), query)
		return false
	}
	return len(docs) == 1
}

func (o *OrientDBBackend) DelEdge(e *Edge) bool {
	query := fmt.Sprintf("DELETE EDGE Link WHERE ID = \"%s\"", e.ID)
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

func (o *OrientDBBackend) GetEdge(i Identifier) *Edge {
	query := fmt.Sprintf("SELECT expand(rid) FROM INDEX:Link.ID WHERE key = \"%s\"", i)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edge %s: %s", i, err.Error())
		return nil
	}
	if len(docs) != 0 {
		return orientDBDocumentToEdge(docs[0])
	}
	return nil
}

func (o *OrientDBBackend) GetEdgeNodes(e *Edge) (n1 *Node, n2 *Node) {
	query := fmt.Sprintf("SELECT expand(rid) FROM INDEX:Node.ID WHERE key in [\"%s\", \"%s\"]", e.parent, e.child)
	docs, err := o.client.Sql(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving nodes for edge %s: %s", e.ID, err.Error())
		return nil, nil
	}

	var id1 Identifier
	if len(docs) > 0 {
		n1 = orientDBDocumentToNode(docs[0])
		id1 = Identifier(n1.ID)
	}

	if len(docs) > 1 {
		n2 = orientDBDocumentToNode(docs[1])
	}

	if id1 == e.parent {
		return n1, n2
	} else {
		return n2, n1
	}
}

func getElementAndClassType(i interface{}) (element graphElement, classType string) {
	switch i.(type) {
	case *Node:
		return i.(*Node).graphElement, "Node"
	case *Edge:
		return i.(*Edge).graphElement, "Link"
	}
	return
}

func (o *OrientDBBackend) AddMetadata(i interface{}, k string, v interface{}) bool {
	element, classType := getElementAndClassType(i)
	value, err := json.Marshal(v)
	if err != nil {
		return false
	}

	query := fmt.Sprintf("UPDATE %s SET %s = %s WHERE ID = '%s'", classType, k, string(value), element.ID)
	if _, err := o.client.Sql(query); err != nil {
		logging.GetLogger().Errorf("Error while adding metadata: %s", err.Error())
		return false
	}

	return true
}

func (o *OrientDBBackend) SetMetadata(i interface{}, m Metadata) bool {
	element, classType := getElementAndClassType(i)
	newElement := graphElement{
		ID:       element.ID,
		host:     element.host,
		metadata: m,
	}

	doc := graphElementToOrientDBDocument(newElement)
	delete(doc, "@class")
	content, err := json.Marshal(doc)
	if err != nil {
		return false
	}

	query := fmt.Sprintf("UPDATE %s CONTENT %s WHERE ID = '%s'", classType, content, element.ID)
	if _, err := o.client.Sql(query); err != nil {
		logging.GetLogger().Errorf("Error while setting metadata: %s", err.Error())
		return false
	}

	return true
}

func (o *OrientDBBackend) GetNodes() (nodes []*Node) {
	docs, err := o.client.Sql("SELECT FROM Node")
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving nodes: %s", err.Error(), docs)
		return
	}

	for _, doc := range docs {
		nodes = append(nodes, orientDBDocumentToNode(doc))
	}

	return
}

func (o *OrientDBBackend) GetEdges() (edges []*Edge) {
	docs, err := o.client.Sql("SELECT FROM Link")
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edges: %s", err.Error(), docs)
		return
	}

	for _, doc := range docs {
		edges = append(edges, orientDBDocumentToEdge(doc))
	}

	return
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
			},
			Indexes: []orientdb.Index{
				{Name: "Node.ID", Fields: []string{"ID"}, Type: "UNIQUE_HASH_INDEX"},
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
				{Name: "parent", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "child", Type: "STRING", Mandatory: true, NotNull: true},
			},
			Indexes: []orientdb.Index{
				{Name: "Link.ID", Fields: []string{"ID"}, Type: "UNIQUE_HASH_INDEX"},
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
	addr := config.GetConfig().GetString("orientdb.addr")
	database := config.GetConfig().GetString("orientdb.database")
	username := config.GetConfig().GetString("orientdb.username")
	password := config.GetConfig().GetString("orientdb.password")
	return NewOrientDBBackend(addr, database, username, password)
}
