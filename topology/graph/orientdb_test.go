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
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/storage/orientdb"
)

type op struct {
	name string
	data interface{}
}

type fakeOrientDBClient struct {
	ops          []op
	searchResult []orientdb.Document
}

func (f *fakeOrientDBClient) getOps() []op {
	return f.ops
}

func (f *fakeOrientDBClient) Request(method string, url string, body io.Reader) (*http.Response, error) {
	return nil, nil
}
func (f *fakeOrientDBClient) DeleteDocument(id string) error {
	return nil
}
func (f *fakeOrientDBClient) GetDocument(id string) (orientdb.Document, error) {
	return nil, nil
}
func (f *fakeOrientDBClient) CreateDocument(doc orientdb.Document) (orientdb.Document, error) {
	f.ops = append(f.ops, op{name: "CreateDocument", data: doc})
	return nil, nil
}
func (f *fakeOrientDBClient) Upsert(doc orientdb.Document, key string) (orientdb.Document, error) {
	return nil, nil
}
func (f *fakeOrientDBClient) GetDocumentClass(name string) (*orientdb.DocumentClass, error) {
	return nil, nil
}
func (f *fakeOrientDBClient) AlterProperty(className string, prop orientdb.Property) error {
	return nil
}
func (f *fakeOrientDBClient) CreateProperty(className string, prop orientdb.Property) error {
	return nil
}
func (f *fakeOrientDBClient) CreateClass(class orientdb.ClassDefinition) error {
	return nil
}
func (f *fakeOrientDBClient) CreateIndex(className string, index orientdb.Index) error {
	return nil
}
func (f *fakeOrientDBClient) CreateDocumentClass(class orientdb.ClassDefinition) error {
	return nil
}
func (f *fakeOrientDBClient) DeleteDocumentClass(name string) error {
	return nil
}
func (f *fakeOrientDBClient) GetDatabase() (orientdb.Document, error) {
	return nil, nil
}
func (f *fakeOrientDBClient) CreateDatabase() (orientdb.Document, error) {
	return nil, nil
}
func (f *fakeOrientDBClient) SQL(query string, result interface{}) error {
	return nil
}
func (f *fakeOrientDBClient) Search(query string) ([]orientdb.Document, error) {
	f.ops = append(f.ops, op{name: "Search", data: query})
	return f.searchResult, nil
}
func (f *fakeOrientDBClient) Query(obj string, query *filters.SearchQuery, result interface{}) error {
	return nil
}
func (f *fakeOrientDBClient) Connect() error {
	return nil
}

func newOrientDBGraph(t *testing.T) (*Graph, *fakeOrientDBClient) {
	client := &fakeOrientDBClient{}
	b, err := newOrientDBBackend(client)

	if err != nil {
		t.Error(err.Error())
	}

	return NewGraphFromConfig(b), client
}

// test history when doing local modification
func TestLocalHistory(t *testing.T) {
	g, client := newOrientDBGraph(t)

	client.searchResult = []orientdb.Document{
		{"value": json.Number("1")},
	}

	node := g.newNode("aaa", Metadata{"MTU": 1500}, time.Unix(1, 0), "host1")
	g.addMetadata(node, "MTU", 1510, time.Unix(2, 0))

	expected := []op{
		{
			name: "CreateDocument",
			data: orientdb.Document{
				"UpdatedAt": int64(1000),
				"CreatedAt": int64(1000),
				"Revision":  int64(1),
				"@class":    "Node",
				"ID":        Identifier("aaa"),
				"Host":      "host1",
				"Metadata": Metadata{
					"MTU": 1500,
				},
			},
		},
		{
			name: "Search",
			data: "UPDATE Node SET ArchivedAt = 2000 WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = 'aaa'",
		},
		{
			name: "CreateDocument",
			data: orientdb.Document{
				"UpdatedAt": int64(2000),
				"CreatedAt": int64(1000),
				"Revision":  int64(2),
				"@class":    "Node",
				"ID":        Identifier("aaa"),
				"Host":      "host1",
				"Metadata": Metadata{
					"MTU": 1510,
				},
			},
		},
	}

	if !reflect.DeepEqual(client.getOps()[0].data, expected[0].data) {
		t.Fatalf("Expected orientdb records not found: \nexpected: %s\ngot: %s", spew.Sdump(expected), spew.Sdump(client.getOps()))
	}

	g.addMetadata(node, "MTU", 1520, time.Unix(3, 0))

	expected = []op{
		{
			name: "CreateDocument",
			data: orientdb.Document{
				"UpdatedAt": int64(1000),
				"CreatedAt": int64(1000),
				"Revision":  int64(1),
				"@class":    "Node",
				"ID":        Identifier("aaa"),
				"Host":      "host1",
				"Metadata": Metadata{
					"MTU": 1500,
				},
			},
		},
		{
			name: "Search",
			data: "UPDATE Node SET ArchivedAt = 2000 WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = 'aaa'",
		},
		{
			name: "CreateDocument",
			data: orientdb.Document{
				"UpdatedAt": int64(2000),
				"CreatedAt": int64(1000),
				"Revision":  int64(2),
				"@class":    "Node",
				"ID":        Identifier("aaa"),
				"Host":      "host1",
				"Metadata": Metadata{
					"MTU": 1510,
				},
			},
		},
		{
			name: "Search",
			data: "UPDATE Node SET ArchivedAt = 3000 WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = 'aaa'",
		},
		{
			name: "CreateDocument",
			data: orientdb.Document{
				"UpdatedAt": int64(3000),
				"CreatedAt": int64(1000),
				"Revision":  int64(3),
				"@class":    "Node",
				"ID":        Identifier("aaa"),
				"Host":      "host1",
				"Metadata": Metadata{
					"MTU": 1520,
				},
			},
		},
	}

	if !reflect.DeepEqual(client.getOps(), expected) {
		t.Fatalf("Expected orientdb records not found: \nexpected: %s\ngot: %s", spew.Sdump(expected), spew.Sdump(client.getOps()))
	}

	client.searchResult = []orientdb.Document{
		{"ID": "bbb"},
	}

	g.delNode(node, time.Unix(4, 0))

	expected = []op{
		{
			name: "CreateDocument",
			data: orientdb.Document{
				"UpdatedAt": int64(1000),
				"CreatedAt": int64(1000),
				"Revision":  int64(1),
				"@class":    "Node",
				"ID":        Identifier("aaa"),
				"Host":      "host1",
				"Metadata": Metadata{
					"MTU": 1500,
				},
			},
		},
		{
			name: "Search",
			data: "UPDATE Node SET ArchivedAt = 2000 WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = 'aaa'",
		},
		{
			name: "CreateDocument",
			data: orientdb.Document{
				"UpdatedAt": int64(2000),
				"CreatedAt": int64(1000),
				"Revision":  int64(2),
				"@class":    "Node",
				"ID":        Identifier("aaa"),
				"Host":      "host1",
				"Metadata": Metadata{
					"MTU": 1510,
				},
			},
		},
		{
			name: "Search",
			data: "UPDATE Node SET ArchivedAt = 3000 WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = 'aaa'",
		},
		{
			name: "CreateDocument",
			data: orientdb.Document{
				"UpdatedAt": int64(3000),
				"CreatedAt": int64(1000),
				"Revision":  int64(3),
				"@class":    "Node",
				"ID":        Identifier("aaa"),
				"Host":      "host1",
				"Metadata": Metadata{
					"MTU": 1520,
				},
			},
		},
		{
			name: "Search",
			data: "SELECT FROM Link WHERE ArchivedAt is NULL AND (Parent = 'aaa' OR Child = 'aaa')",
		},
		{
			name: "Search",
			data: "UPDATE Link SET DeletedAt = 4000, ArchivedAt = 4000 WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = 'bbb'",
		},
		{
			name: "Search",
			data: "UPDATE Node SET DeletedAt = 4000, ArchivedAt = 4000 WHERE DeletedAt IS NULL AND ArchivedAt IS NULL AND ID = 'aaa'",
		},
	}

	if !reflect.DeepEqual(client.getOps(), expected) {
		t.Fatalf("Expected orientdb records not found: \nexpected: %s\ngot: %s", spew.Sdump(expected), spew.Sdump(client.getOps()))
	}
}
