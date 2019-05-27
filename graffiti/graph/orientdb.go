/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package graph

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/storage/orientdb"
)

// OrientDBBackend describes an OrientDB backend
type OrientDBBackend struct {
	Backend
	client   orientdb.ClientInterface
	election common.MasterElection
}

type eventTime struct {
	name string
	t    Time
}

func graphElementToOrientDBSetString(e graphElement) (s string) {
	properties := []string{
		fmt.Sprintf("ID = \"%s\"", string(e.ID)),
		fmt.Sprintf("Host = \"%s\"", e.Host),
		fmt.Sprintf("Origin = \"%s\"", e.Origin),
		fmt.Sprintf("CreatedAt = %d", e.CreatedAt.Unix()),
		fmt.Sprintf("UpdatedAt = %d", e.UpdatedAt.Unix()),
		fmt.Sprintf("Revision = %d", e.Revision),
	}
	s = strings.Join(properties, ", ")
	if m := metadataToOrientDBSetString(e.Metadata); m != "" {
		s += ", " + m
	}
	return
}

func metadataToOrientDBSetString(m Metadata) string {
	if len(m) > 0 {
		if b, err := json.Marshal(m); err == nil {
			return "Metadata = " + string(b)
		}
	}
	return ""
}

func metadataToOrientDBSelectString(m ElementMatcher) string {
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

func (o *OrientDBBackend) updateTimes(e string, id string, events ...eventTime) error {
	attrs := []string{}
	for _, event := range events {
		attrs = append(attrs, fmt.Sprintf("%s = %d", event.name, event.t.Unix()))
	}
	query := fmt.Sprintf("UPDATE %s SET %s WHERE ID = '%s' AND DeletedAt IS NULL AND ArchivedAt IS NULL", e, strings.Join(attrs, ", "), id)
	result, err := o.client.SQL(query)
	if err != nil {
		return fmt.Errorf("Error while deleting %s: %s", id, err)
	}

	values := struct {
		Result []map[string]interface{}
	}{}

	if err := json.Unmarshal(result.Body, &values); err != nil {
		return fmt.Errorf("Error while parsing nodes: %s, %s", err, string(result.Body))
	}
	if len(values.Result) == 0 {
		return ErrElementNotFound
	}

	value, ok := values.Result[0]["value"]
	if !ok {
		return ErrElementNotFound
	}

	f, ok := value.(float64)
	if !ok || f != 1 {
		return ErrInternal
	}

	return nil
}

func (o *OrientDBBackend) createNode(n *Node) error {
	on := struct {
		*Node
		Class string `json:"@class"`
	}{
		Node:  n,
		Class: "Node",
	}

	data, err := json.Marshal(on)
	if err != nil {
		return fmt.Errorf("error while adding node %s: %s", n.ID, err)
	}

	if _, err := o.client.CreateDocument(json.RawMessage(data)); err != nil {
		return fmt.Errorf("Error while adding node %s: %s", n.ID, err)
	}
	return nil
}

func (o *OrientDBBackend) searchNodes(t Context, where string) []*Node {
	query := "SELECT FROM Node WHERE " + where
	if !t.TimePoint {
		query += " ORDER BY UpdatedAt"
	}

	result, err := o.client.SQL(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving nodes: %s", err)
		return nil
	}

	nodes := struct {
		Result []*Node
	}{}

	if err := json.Unmarshal(result.Body, &nodes); err != nil {
		logging.GetLogger().Errorf("Error while parsing nodes: %s, %s", err, string(result.Body))
	}

	if len(nodes.Result) > 1 && t.TimePoint {
		nodes.Result = dedupNodes(nodes.Result)
	}

	return nodes.Result
}

func (o *OrientDBBackend) searchEdges(t Context, where string) []*Edge {
	query := "SELECT FROM Link WHERE " + where
	if !t.TimePoint {
		query += " ORDER BY UpdatedAt"
	}

	result, err := o.client.SQL(query)
	if err != nil {
		logging.GetLogger().Errorf("Error while retrieving edges: %s", err)
		return nil
	}

	edges := struct {
		Result []*Edge
	}{}

	if err := json.Unmarshal(result.Body, &edges); err != nil {
		logging.GetLogger().Errorf("Error while parsing edges: %s, %s", err, string(result.Body))
	}

	if len(edges.Result) > 1 && t.TimePoint {
		edges.Result = dedupEdges(edges.Result)
	}

	return edges.Result
}

// NodeAdded add a node in the database
func (o *OrientDBBackend) NodeAdded(n *Node) error {
	return o.createNode(n)
}

// NodeDeleted delete a node in the database
func (o *OrientDBBackend) NodeDeleted(n *Node) error {
	return o.updateTimes("Node", string(n.ID), eventTime{"DeletedAt", n.DeletedAt}, eventTime{"ArchivedAt", n.DeletedAt})
}

// GetNode get a node within a time slice
func (o *OrientDBBackend) GetNode(i Identifier, t Context) (nodes []*Node) {
	query := orientdb.FilterToExpression(getTimeFilter(t.TimeSlice), nil)
	query += fmt.Sprintf(" AND ID = '%s' ORDER BY Revision", i)
	if t.TimePoint {
		query += " DESC LIMIT 1"
	}
	return o.searchNodes(t, query)
}

// GetNodeEdges returns a list of a node edges within time slice
func (o *OrientDBBackend) GetNodeEdges(n *Node, t Context, m ElementMatcher) (edges []*Edge) {
	query := orientdb.FilterToExpression(getTimeFilter(t.TimeSlice), nil)
	query += fmt.Sprintf(" AND (Parent = '%s' OR Child = '%s')", n.ID, n.ID)
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}
	return o.searchEdges(t, query)
}

func (o *OrientDBBackend) createEdge(e *Edge) error {
	fromQuery := fmt.Sprintf("SELECT FROM Node WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = '%s'", e.Parent)
	toQuery := fmt.Sprintf("SELECT FROM Node WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = '%s'", e.Child)
	setQuery := fmt.Sprintf("%s, Parent = '%s', Child = '%s'", graphElementToOrientDBSetString(e.graphElement), e.Parent, e.Child)
	query := fmt.Sprintf("CREATE EDGE Link FROM (%s) TO (%s) SET %s RETRY 100 WAIT 20", fromQuery, toQuery, setQuery)

	if _, err := o.client.SQL(query); err != nil {
		return fmt.Errorf("Error while adding edge %s: %s (sql: %s)", e.ID, err, query)
	}

	return nil
}

// EdgeAdded add a node in the database
func (o *OrientDBBackend) EdgeAdded(e *Edge) error {
	return o.createEdge(e)
}

// EdgeDeleted delete a node in the database
func (o *OrientDBBackend) EdgeDeleted(e *Edge) error {
	return o.updateTimes("Link", string(e.ID), eventTime{"DeletedAt", e.DeletedAt}, eventTime{"ArchivedAt", e.DeletedAt})
}

// GetEdge get an edge within a time slice
func (o *OrientDBBackend) GetEdge(i Identifier, t Context) []*Edge {
	query := orientdb.FilterToExpression(getTimeFilter(t.TimeSlice), nil)
	query += fmt.Sprintf(" AND ID = '%s' ORDER BY Revision", i)
	if t.TimePoint {
		query += " DESC LIMIT 1"
	}
	return o.searchEdges(t, query)
}

// GetEdgeNodes returns the parents and child nodes of an edge within time slice, matching metadata
func (o *OrientDBBackend) GetEdgeNodes(e *Edge, t Context, parentMetadata, childMetadata ElementMatcher) (parents []*Node, children []*Node) {
	query := orientdb.FilterToExpression(getTimeFilter(t.TimeSlice), nil)
	query += fmt.Sprintf(" AND ID in [\"%s\", \"%s\"]", e.Parent, e.Child)

	for _, node := range o.searchNodes(t, query) {
		if node.ID == e.Parent && node.MatchMetadata(parentMetadata) {
			parents = append(parents, node)
		} else if node.MatchMetadata(childMetadata) {
			children = append(children, node)
		}
	}

	return
}

// MetadataUpdated returns true if a metadata has been updated in the database, based on ArchivedAt
func (o *OrientDBBackend) MetadataUpdated(i interface{}) error {
	var err error

	switch i := i.(type) {
	case *Node:
		if err := o.updateTimes("Node", string(i.ID), eventTime{"ArchivedAt", i.UpdatedAt}); err != nil {
			return err
		}

		err = o.createNode(i)
	case *Edge:
		if err := o.updateTimes("Link", string(i.ID), eventTime{"ArchivedAt", i.UpdatedAt}); err != nil {
			return err
		}

		err = o.createEdge(i)
	}

	return err
}

// GetNodes returns a list of nodes within time slice, matching metadata
func (o *OrientDBBackend) GetNodes(t Context, m ElementMatcher) (nodes []*Node) {
	query := orientdb.FilterToExpression(getTimeFilter(t.TimeSlice), nil)
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}

	logging.GetLogger().Debugf("GetNodes %+v => %s (%+v)", t, query, getTimeFilter(t.TimeSlice))
	return o.searchNodes(t, query)
}

// GetEdges returns a list of edges within time slice, matching metadata
func (o *OrientDBBackend) GetEdges(t Context, m ElementMatcher) (edges []*Edge) {
	query := orientdb.FilterToExpression(getTimeFilter(t.TimeSlice), nil)
	if metadataQuery := metadataToOrientDBSelectString(m); metadataQuery != "" {
		query += " AND " + metadataQuery
	}

	return o.searchEdges(t, query)
}

// IsHistorySupported returns that this backend does support history
func (o *OrientDBBackend) IsHistorySupported() bool {
	return true
}

func (o *OrientDBBackend) flushGraph() error {
	logging.GetLogger().Info("Flush graph elements")

	now := TimeUTC().Unix()

	query := fmt.Sprintf("UPDATE Node SET DeletedAt = %d, ArchivedAt = %d WHERE DeletedAt IS NULL", now, now)
	if _, err := o.client.SQL(query); err != nil {
		return fmt.Errorf("Error while flushing graph: %s", err)
	}

	query = fmt.Sprintf("UPDATE Edge SET DeletedAt = %d, ArchivedAt = %d WHERE DeletedAt IS NULL", now, now)
	if _, err := o.client.SQL(query); err != nil {
		return fmt.Errorf("Error while flushing graph: %s", err)
	}

	return nil
}

// OnStarted implements storage client listener interface
func (o *OrientDBBackend) OnStarted() {
	if o.election != nil && o.election.IsMaster() {
		o.flushGraph()
	}
}

func newOrientDBBackend(client orientdb.ClientInterface, electionService common.MasterElectionService) (*OrientDBBackend, error) {
	o := &OrientDBBackend{
		client: client,
	}

	if electionService != nil {
		o.election = electionService.NewElection("orientdb-graph-flush")
		o.election.StartAndWait()
	}

	client.AddEventListener(o)
	client.Connect()

	if _, err := client.GetDocumentClass("Node"); err != nil {
		class := orientdb.ClassDefinition{
			Name:       "Node",
			SuperClass: "V",
			Properties: []orientdb.Property{
				{Name: "ID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Host", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Origin", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "ArchivedAt", Type: "LONG", NotNull: true, ReadOnly: true},
				{Name: "CreatedAt", Type: "LONG", Mandatory: true, NotNull: true, ReadOnly: true},
				{Name: "UpdatedAt", Type: "LONG"},
				{Name: "DeletedAt", Type: "LONG"},
				{Name: "Revision", Type: "LONG", Mandatory: true, NotNull: true},
				{Name: "Metadata", Type: "EMBEDDEDMAP"},
			},
			Indexes: []orientdb.Index{
				{Name: "Node.ID", Fields: []string{"ID"}, Type: "NOTUNIQUE"},
				{Name: "Node.Lifetime", Fields: []string{"CreatedAt", "DeletedAt"}, Type: "NOTUNIQUE"},
				{Name: "Node.ArchiveTime", Fields: []string{"UpdatedAt", "ArchivedAt"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class Node: %s", err)
		}
	}

	if _, err := client.GetDocumentClass("Link"); err != nil {
		class := orientdb.ClassDefinition{
			Name:       "Link",
			SuperClass: "E",
			Properties: []orientdb.Property{
				{Name: "ID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Host", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Origin", Type: "STRING", Mandatory: true, NotNull: true},
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
				{Name: "Link.ID", Fields: []string{"ID"}, Type: "NOTUNIQUE"},
				{Name: "Link.TimeSpan", Fields: []string{"CreatedAt", "DeletedAt"}, Type: "NOTUNIQUE"},
				{Name: "Link.ArchiveTime", Fields: []string{"UpdatedAt", "ArchivedAt"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class Link: %s", err)
		}
	}

	return o, nil
}

// NewOrientDBBackend creates a new graph backend and
// connect to an OrientDB instance
func NewOrientDBBackend(addr string, database string, username string, password string, electionService common.MasterElectionService) (*OrientDBBackend, error) {
	client, err := orientdb.NewClient(addr, database, username, password)
	if err != nil {
		return nil, err
	}

	return newOrientDBBackend(client, electionService)
}
