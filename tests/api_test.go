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

package tests

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/avast/retry-go"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	gtypes "github.com/skydive-project/skydive/graffiti/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	g "github.com/skydive-project/skydive/gremlin"
)

func getCrudClient() (c *shttp.CrudClient, err error) {
	retry.Do(func() error {
		c, err = client.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
		return err
	})
	return
}
func TestAlertAPI(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	alert := gtypes.NewAlert()
	alert.Expression = g.G.V().Has("MTU", g.Gt(1500)).String()
	if err := client.Create("alert", alert, nil); err != nil {
		t.Errorf("Failed to create alert: %s", err.Error())
	}

	alert2 := gtypes.NewAlert()
	alert2.Expression = g.G.V().Has("MTU", g.Gt(1500)).String()
	if err := client.Get("alert", alert.UUID, &alert2); err != nil {
		t.Error(err)
	}

	if *alert != *alert2 {
		t.Errorf("Alert corrupted: %+v != %+v", alert, alert2)
	}

	var alerts map[string]gtypes.Alert
	if err := client.List("alert", &alerts); err != nil {
		t.Error(err)
	} else {
		if len(alerts) != 1 {
			t.Errorf("Wrong number of alerts: got %d, expected 1 (%+v)", len(alerts), alerts)
		}
	}

	if alerts[alert.UUID] != *alert {
		t.Errorf("Alert corrupted: %+v != %+v", alerts[alert.UUID], alert)
	}

	if err := client.Delete("alert", alert.UUID); err != nil {
		t.Errorf("Failed to delete alert: %s", err.Error())
	}

	var alerts2 map[string]gtypes.Alert
	if err := client.List("alert", &alerts2); err != nil {
		t.Errorf("Failed to list alerts: %s", err.Error())
	} else {
		if len(alerts2) != 0 {
			t.Errorf("Wrong number of alerts: got %d, expected 0 (%+v)", len(alerts2), alerts2)
		}
	}
}

func TestCaptureAPI(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	var captures map[string]types.Capture
	if err := client.List("capture", &captures); err != nil {
		t.Error(err)
	}
	nbCaptures := len(captures)

	capture := types.NewCapture(g.G.V().Has("Name", "br-int").String(), "port 80")
	if err := client.Create("capture", capture, nil); err != nil {
		t.Fatalf("Failed to create alert: %s", err.Error())
	}

	capture2 := &types.Capture{}
	if err := client.Get("capture", capture.GetID(), &capture2); err != nil {
		t.Error(err)
	}

	if *capture != *capture2 {
		t.Errorf("Capture corrupted: %+v != %+v", capture, capture2)
	}

	if err := client.List("capture", &captures); err != nil {
		t.Error(err)
	} else {
		if (len(captures) - nbCaptures) != 1 {
			t.Errorf("Wrong number of captures: got %d, expected 1", len(captures))
		}
	}

	if captures[capture.GetID()] != *capture {
		t.Errorf("Capture corrupted: %+v != %+v", captures[capture.GetID()], capture)
	}

	if err := client.Delete("capture", capture.GetID()); err != nil {
		t.Errorf("Failed to delete capture: %s", err.Error())
	}

	var captures2 map[string]types.Capture
	if err := client.List("capture", &captures2); err != nil {
		t.Errorf("Failed to list captures: %s", err.Error())
	} else {
		if (len(captures2) - nbCaptures) != 0 {
			t.Errorf("Wrong number of captures: got %d, expected 0 (%+v)", len(captures2)-nbCaptures, captures2)
		}
	}

	if err := client.Get("capture", capture.GetID(), &capture2); err == nil {
		t.Errorf("Found delete capture: %s", capture.GetID())
	}
}

func TestNodeAPI(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	node := new(types.Node)

	if err := client.Create("node", node, nil); err == nil {
		t.Errorf("Expected error when creating a node without ID")
	}

	node.ID = graph.GenID()
	node.Metadata = graph.Metadata{"Type": "mytype"}

	if err := client.Create("node", node, nil); err == nil {
		t.Errorf("Expected error when creating a node without name")
	}

	node.Metadata["Name"] = "myname"
	if err := client.Create("node", node, nil); err != nil {
		t.Error(err)
	}

	if err := client.Get("node", string(node.ID), &node); err != nil {
		t.Error(err)
	}

	name, _ := node.GetFieldString("Name")
	typ, _ := node.GetFieldString("Type")
	if name != "myname" || typ != "mytype" {
		t.Errorf("Expected node with name 'myname' and type 'mytype'")
	}
}

func TestEdgeAPI(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	node1 := new(types.Node)
	node1.ID = graph.GenID()
	node1.Metadata = graph.Metadata{"Type": "mytype", "Name": "node1"}

	node2 := new(types.Node)
	node2.ID = graph.GenID()
	node2.Metadata = graph.Metadata{"Type": "mytype", "Name": "node2"}

	if err := client.Create("node", node1, nil); err != nil {
		t.Fatal(err)
	}

	if err := client.Create("node", node2, nil); err != nil {
		t.Fatal(err)
	}

	edge := new(types.Edge)

	if err := client.Create("edge", edge, nil); err == nil {
		t.Errorf("Expected error when creating a edge without ID")
	}

	edge.ID = graph.GenID()

	if err := client.Create("edge", edge, nil); err == nil {
		t.Errorf("Expected error when creating a edge without relation type")
	}

	edge.Metadata = graph.Metadata{"RelationType": "mylink"}

	if err := client.Create("edge", edge, nil); err == nil {
		t.Errorf("Expected error when creating a edge without parent and child")
	}

	edge.Parent = node1.ID
	edge.Child = node2.ID

	if err := client.Create("edge", edge, nil); err != nil {
		t.Error(err)
	}

	if err := client.Get("edge", string(edge.ID), &edge); err != nil {
		t.Error(err)
	}

	if relationType, _ := edge.GetFieldString("RelationType"); relationType != "mylink" {
		t.Errorf("Expected edge with relation type 'mylink'")
	}
}

// PATCH
type JSONPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// Create node, patch node, check modification has been made and updatedAt/revision updated
func TestAPIPatchNodeTable(t *testing.T) {
	tests := []struct {
		name         string
		originalNode []byte
		patch        []JSONPatch
		expectedNode []byte
		updated      bool
	}{
		{
			name: "PATCH node, add new attribute",
			originalNode: []byte(`{
							"ID": "test1",
							"Metadata": {
								"TID": "test1",
								"Name": "name1",
								"Type": "type1"
							},
							"Host": "host1",
							"Origin": "origin1",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:    "add",
					Path:  "/Metadata/foo",
					Value: "bar",
				},
			},
			expectedNode: []byte(`{
							"ID": "test1",
							"Metadata": {
								"TID":  "test1",
								"Name": "name1",
								"Type": "type1",
								"foo": "bar"
							},
							"Host": "host1",
							"Origin": "origin1",
							"CreatedAt": 0,
							"UpdatedAt": 1,
							"Revision":  1
						}`),
			updated: true,
		},
		{
			name: "PATCH node, modify attribute",
			originalNode: []byte(`{
							"ID": "test2",
							"Metadata": {
								"TID": "test2",
								"Name": "name2",
								"Type": "type2",
								"foo": "bar"
							},
							"Host": "host2",
							"Origin": "origin2",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/foo",
					Value: "foo",
				},
			},
			expectedNode: []byte(`{
							"ID": "test2",
							"Metadata": {
								"TID":  "test2",
								"Name": "name2",
								"Type": "type2",
								"foo": "foo"
							},
							"Host": "host2",
							"Origin": "origin2",
							"CreatedAt": 0,
							"UpdatedAt": 1,
							"Revision":  1
						}`),
			updated: true,
		},
		{
			name: "PATCH node, delete attribute",
			originalNode: []byte(`{
							"ID": "test3",
							"Metadata": {
								"TID": "test3",
								"Name": "name3",
								"Type": "type3",
								"foo": "bar"
							},
							"Host": "host3",
							"Origin": "origin3",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:   "remove",
					Path: "/Metadata/foo",
				},
			},
			expectedNode: []byte(`{
							"ID": "test3",
							"Metadata": {
								"TID":  "test3",
								"Name": "name3",
								"Type": "type3"
							},
							"Host": "host3",
							"Origin": "origin3",
							"CreatedAt": 0,
							"UpdatedAt": 1,
							"Revision":  1
						}`),
			updated: true,
		},
		{
			name: "PATCH node, modify attribute Metadata.TID is ignored",
			originalNode: []byte(`{
							"ID": "test4",
							"Metadata": {
								"TID": "test4",
								"Name": "name4",
								"Type": "type4"
							},
							"Host": "host4",
							"Origin": "origin4",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/TID",
					Value: "123",
				},
			},
			expectedNode: []byte(`{
							"ID": "test4",
							"Metadata": {
								"TID":  "test4",
								"Name": "name4",
								"Type": "type4"
							},
							"Host": "host4",
							"Origin": "origin4",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			updated: false,
		},
		{
			name: "PATCH node, modify attribute Metadata.Name is ignored",
			originalNode: []byte(`{
							"ID": "test5",
							"Metadata": {
								"TID": "test5",
								"Name": "name5",
								"Type": "type5"
							},
							"Host": "host5",
							"Origin": "origin5",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/Name",
					Value: "noop",
				},
			},
			expectedNode: []byte(`{
							"ID": "test5",
							"Metadata": {
								"TID":  "test5",
								"Name": "name5",
								"Type": "type5"
							},
							"Host": "host5",
							"Origin": "origin5",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			updated: false,
		},
		{
			name: "PATCH node, modify attribute Metadata.Type is ignored",
			originalNode: []byte(`{
							"ID": "test6",
							"Metadata": {
								"TID": "test6",
								"Name": "name6",
								"Type": "type6"
							},
							"Host": "host6",
							"Origin": "origin6",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/Type",
					Value: "noop",
				},
			},
			expectedNode: []byte(`{
							"ID": "test6",
							"Metadata": {
								"TID":  "test6",
								"Name": "name6",
								"Type": "type6"
							},
							"Host": "host6",
							"Origin": "origin6",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			updated: false,
		},
		{
			name: "PATCH node, delete attributes Metadata.TID is ignored",
			originalNode: []byte(`{
							"ID": "test7",
							"Metadata": {
								"TID": "test7",
								"Name": "name7",
								"Type": "type7"
							},
							"Host": "host7",
							"Origin": "origin7",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:   "remove",
					Path: "/Metadata/TID",
				},
			},
			expectedNode: []byte(`{
							"ID": "test7",
							"Metadata": {
								"TID":  "test7",
								"Name": "name7",
								"Type": "type7"
							},
							"Host": "host7",
							"Origin": "origin7",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			updated: false,
		},
		{
			name: "PATCH node, modifying values outside Metadata are ignored",
			originalNode: []byte(`{
						"ID": "test8",
						"Metadata": {
							"TID": "test8",
							"Name": "name8",
							"Type": "type8"
						},
						"Host": "host8",
						"Origin": "origin8",
						"CreatedAt": 0,
						"UpdatedAt": 0,
						"Revision":  0
					}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Host",
					Value: "foo",
				},
			},
			expectedNode: []byte(`{
						"ID": "test8",
						"Metadata": {
							"TID":  "test8",
							"Name": "name8",
							"Type": "type8"
						},
						"Host": "host8",
						"Origin": "origin8",
						"CreatedAt": 0,
						"UpdatedAt": 0,
						"Revision":  0
					}`),
			updated: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := getCrudClient()
			if err != nil {
				t.Fatal(err)
			}

			// Add originalNode to the graph
			originalNode := types.Node{}
			expectedNode := types.Node{}
			patchedNode := types.Node{}
			getPatchedNode := types.Node{}

			if err := json.Unmarshal(test.originalNode, &originalNode); err != nil {
				t.Fatalf("error unmarshal originalNode: %v", err)
			}

			if err := json.Unmarshal(test.expectedNode, &expectedNode); err != nil {
				t.Fatalf("error unmarshal expectedNode: %v", err)
			}

			// Create original node
			if err := client.Create("node", &originalNode, nil); err != nil {
				t.Fatalf("Failed to create originalNode: %s", err.Error())
			}
			defer func() {
				if err := client.Delete("node", originalNode.GetID()); err != nil {
					t.Fatalf("Failed to clean nodeA: %s", err.Error())
				}
			}()

			// Patch node
			updated, err := client.Update("node", originalNode.GetID(), test.patch, &patchedNode)
			if err != nil {
				t.Fatalf("Failed to apply patch: %s", err.Error())
			}

			err = client.Get("node", originalNode.GetID(), &getPatchedNode)
			if err != nil {
				t.Fatalf("Failed to get node after patching: %s", err.Error())
			}

			// Compare nodes
			if !reflect.DeepEqual(expectedNode.Metadata, getPatchedNode.Metadata) {
				t.Errorf("JSON Patch was not applied.\nMetadata expected: %+v\nMetadata patched:  %+v", expectedNode.Metadata, getPatchedNode.Metadata)
			}

			if test.updated != updated {
				t.Errorf("Wrong resource update status. Expected: %v. Received:  %v", test.updated, updated)
			}

			// If server returns 304 (Not modified), it does not return the JSON with the patched node.
			// In that case, ignore comparasion between PATCH response and node stored
			if updated {
				if !reflect.DeepEqual(patchedNode, getPatchedNode) {
					t.Errorf("Response from PATCH method and node stored are not the same.\nPATCH response: %+v\nNode stored:  %+v", patchedNode, getPatchedNode)
				}
			}

			if expectedNode.Revision != getPatchedNode.Revision {
				t.Errorf("Revision version is not being updated by PATCH")
			}

			// If expectedNode.UpdatedAt is "1", we expect a change in the UpdatedAt field
			if expectedNode.UpdatedAt.UnixMilli() == 1 && getPatchedNode.UpdatedAt.IsZero() {
				t.Error("UpdatedAt is not being updated by PATCH")
			}
		})
	}
}

// Try to patch a non existing node
func TestAPIPatchNodeNonExistingNode(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	patchedNode := types.Node{}

	patch := []JSONPatch{
		{
			Op:    "add",
			Path:  "/Metadata/Foo",
			Value: "bar",
		},
	}

	expectedError := fmt.Errorf("Failed to update node, 404 Not Found: ")

	// Patch node
	_, err = client.Update("node", "foo", patch, &patchedNode)
	if err == nil {
		t.Error("Error should be returned because trying to modify a node that does not exists")
	} else if err.Error() != expectedError.Error() {
		t.Errorf("Returned error is incorrect.\nExpected: %v\nReturned: %v", expectedError, err)
	}
}

// Try to patch deleting Metadata.Name generates a validation error
func TestAPIPatchNodeNameDeleteValidationError(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	// Add originalNode to the graph
	originalNode := types.Node{}
	patchedNode := types.Node{}

	if err := json.Unmarshal([]byte(`{
                             "ID": "foo",
                             "Metadata": {
                                     "TID": "foo",
                                     "Name": "name1",
                                     "Type": "type1"
                             },
                             "Host": "host1",
                             "Origin": "origin1",
                             "CreatedAt": 0,
                             "UpdatedAt": 0,
                             "Revision":  0
                        }`), &originalNode); err != nil {
		t.Fatalf("error unmarshal originalNode: %v", err)
	}

	// Create original node
	if err := client.Create("node", &originalNode, nil); err != nil {
		t.Fatalf("Failed to create originalNode: %s", err.Error())
	}

	patch := []JSONPatch{
		{
			Op:   "remove",
			Path: "/Metadata/Name",
		},
	}

	expectedError := fmt.Errorf("Failed to update node, 400 Bad Request: validating patched resource: invalid schema: (root): Must validate all the schemas (allOf)\nMetadata: Must validate all the schemas (allOf)\nMetadata: Name is required\n(root): Must validate at least one schema (anyOf)\n")

	// Patch node
	_, err = client.Update("node", "foo", patch, &patchedNode)
	if err == nil {
		t.Errorf("Error should be returned because trying to remove Metadata.Name violates validation")
	} else if err.Error() != expectedError.Error() {
		t.Errorf("Returned error is incorrect.\nExpected: %v\nReturned: %v", expectedError, err)
	}

	if err := client.Delete("node", originalNode.GetID()); err != nil {
		t.Errorf("Failed to delete originalNode: %s", err.Error())
	}
}

// Erroneous node patch
func TestAPIPatchNodeErroneousPatch(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	// Add originalNode to the graph
	patchedNode := types.Node{}

	patch := []JSONPatch{
		{
			Op:    "foobar",
			Path:  "/Metadata/Foo",
			Value: "bar",
		},
	}

	// Patch node
	_, err = client.Update("node", "foo", patch, &patchedNode)
	if err == nil {
		t.Fatal("Error should be returned if patch is incorrect")
	}
}

// Create two nodes, an edge, patch edge, check modification has been made and updatedAt/revision updated
func TestAPIPatchEdgeTable(t *testing.T) {
	tests := []struct {
		name         string
		originalEdge []byte
		patch        []JSONPatch
		expectedEdge []byte
		updated      bool
	}{
		{
			name: "PATCH edge, add new attribute",
			originalEdge: []byte(`{
							"ID": "test1",
							"Parent": "nodeA",
							"Child": "nodeB",
							"Metadata": {
								"RelationType": "type1"
							},
							"Host": "host1",
							"Origin": "origin1",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:    "add",
					Path:  "/Metadata/foo",
					Value: "bar",
				},
			},
			expectedEdge: []byte(`{
							"ID": "test1",
							"Parent": "nodeA",
							"Child": "nodeB",
							"Metadata": {
								"RelationType": "type1",
								"foo": "bar"
							},
							"Host": "host1",
							"Origin": "origin1",
							"CreatedAt": 0,
							"UpdatedAt": 1,
							"Revision":  1
						}`),
			updated: true,
		},
		{
			name: "PATCH edge, modify attribute",
			originalEdge: []byte(`{
							"ID": "test1",
							"Parent": "nodeA",
							"Child": "nodeB",
							"Metadata": {
								"RelationType": "type1",
								"foo": "bar"
							},
							"Host": "host1",
							"Origin": "origin1",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/foo",
					Value: "foo",
				},
			},
			expectedEdge: []byte(`{
							"ID": "test1",
							"Parent": "nodeA",
							"Child": "nodeB",
							"Metadata": {
								"RelationType": "type1",
								"foo": "foo"
							},
							"Host": "host1",
							"Origin": "origin1",
							"CreatedAt": 0,
							"UpdatedAt": 1,
							"Revision":  1
						}`),
			updated: true,
		},
		{
			name: "PATCH edge, delete attribute",
			originalEdge: []byte(`{
							"ID": "test1",
							"Parent": "nodeA",
							"Child": "nodeB",
							"Metadata": {
								"RelationType": "type1",
								"foo": "bar"
							},
							"Host": "host1",
							"Origin": "origin1",
							"CreatedAt": 0,
							"UpdatedAt": 0,
							"Revision":  0
						}`),
			patch: []JSONPatch{
				{
					Op:   "remove",
					Path: "/Metadata/foo",
				},
			},
			expectedEdge: []byte(`{
							"ID": "test1",
							"Parent": "nodeA",
							"Child": "nodeB",
							"Metadata": {
								"RelationType": "type1"
							},
							"Host": "host1",
							"Origin": "origin1",
							"CreatedAt": 0,
							"UpdatedAt": 1,
							"Revision":  1
						}`),
			updated: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := getCrudClient()
			if err != nil {
				t.Fatal(err)
			}

			nodeA := types.Node{}
			if err := json.Unmarshal([]byte(`{
                             "ID": "nodeA",
                             "Metadata": {
                                     "TID": "nodeA",
                                     "Name": "name1",
                                     "Type": "type1"
                             },
                             "Host": "host1",
                             "Origin": "origin1",
                             "CreatedAt": 0,
                             "UpdatedAt": 0,
                             "Revision":  0
                        }`), &nodeA); err != nil {
				t.Fatalf("error unmarshal nodeA: %v", err)
			}

			if err := client.Create("node", &nodeA, nil); err != nil {
				t.Fatalf("Failed to create nodeA: %s", err.Error())
			}
			defer func() {
				if err := client.Delete("node", nodeA.GetID()); err != nil {
					t.Fatalf("Failed to clean nodeA: %s", err.Error())
				}
			}()

			nodeB := types.Node{}
			if err := json.Unmarshal([]byte(`{
                             "ID": "nodeB",
                             "Metadata": {
                                     "TID": "nodeB",
                                     "Name": "name1",
                                     "Type": "type1"
                             },
                             "Host": "host1",
                             "Origin": "origin1",
                             "CreatedAt": 0,
                             "UpdatedAt": 0,
                             "Revision":  0
                        }`), &nodeB); err != nil {
				t.Fatalf("error unmarshal nodeB: %v", err)
			}

			if err := client.Create("node", &nodeB, nil); err != nil {
				t.Fatalf("Failed to create nodeB: %s", err.Error())
			}

			defer func() {
				if err := client.Delete("node", nodeB.GetID()); err != nil {
					t.Fatalf("Failed to clean nodeB: %s", err.Error())
				}
			}()

			// Add originalNode to the graph
			originalEdge := types.Edge{}
			expectedEdge := types.Edge{}
			patchedEdge := types.Edge{}
			getPatchedEdge := types.Edge{}

			if err := json.Unmarshal(test.originalEdge, &originalEdge); err != nil {
				t.Fatalf("error unmarshal originalEdge: %v", err)
			}

			if err := json.Unmarshal(test.expectedEdge, &expectedEdge); err != nil {
				t.Fatalf("error unmarshal expectedEdge: %v", err)
			}

			// Create original node
			if err := client.Create("edge", &originalEdge, nil); err != nil {
				t.Fatalf("Failed to create originalEdge: %s", err.Error())
			}

			// Patch node
			updated, err := client.Update("edge", originalEdge.GetID(), test.patch, &patchedEdge)
			if err != nil {
				t.Fatalf("Failed to apply patch: %s", err.Error())
			}

			err = client.Get("edge", originalEdge.GetID(), &getPatchedEdge)
			if err != nil {
				t.Fatalf("Failed to get edge after patching: %s", err.Error())
			}

			// Compare edges
			if !reflect.DeepEqual(expectedEdge.Metadata, getPatchedEdge.Metadata) {
				t.Errorf("JSON Patch was not applied.\nMetadata expected: %+v\nMetadata patched:  %+v", expectedEdge.Metadata, getPatchedEdge.Metadata)
			}

			if test.updated != updated {
				t.Errorf("Wrong resource update status. Expected: %v. Received:  %v", test.updated, updated)
			}

			// If server returns 304 (Not modified), it does not return the JSON with the patched node.
			// In that case, ignore comparasion between PATCH response and node stored
			if updated {
				if !reflect.DeepEqual(patchedEdge, getPatchedEdge) {
					t.Errorf("Response from PATCH method and node stored are not the same.\nPATCH response: %+v\nEdge stored:  %+v", patchedEdge, getPatchedEdge)
				}
			}

			if expectedEdge.Revision != getPatchedEdge.Revision {
				t.Errorf("Revision version is not being updated by PATCH")
			}

			// If expectedEdge.UpdatedAt is "1", we expect a change in the UpdatedAt field
			if expectedEdge.UpdatedAt.UnixMilli() == 1 && getPatchedEdge.UpdatedAt.IsZero() {
				t.Error("UpdatedAt is not being updated by PATCH")
			}
		})
	}
}

// Try to patch a non existing node
func TestAPIPatchEdgeNonExistingEdge(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	patchedEdge := types.Edge{}

	patch := []JSONPatch{
		{
			Op:    "add",
			Path:  "/Metadata/Foo",
			Value: "bar",
		},
	}

	expectedError := fmt.Errorf("Failed to update edge, 404 Not Found: ")

	// Patch edge
	_, err = client.Update("edge", "foo", patch, &patchedEdge)
	if err == nil {
		t.Error("Error should be returned because trying to modify a edge that does not exists")
	} else if err.Error() != expectedError.Error() {
		t.Errorf("Returned error is incorrect.\nExpected: %v\nReturned: %v", expectedError, err)
	}
}

// Erroneous edge patch
func TestAPIPatchEdgeErroneousPatch(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	// Add originalEdge to the graph
	patchedEdge := types.Edge{}

	patch := []JSONPatch{
		{
			Op:    "foobar",
			Path:  "/Metadata/Foo",
			Value: "bar",
		},
	}

	// Patch edge
	_, err = client.Update("edge", "foo", patch, &patchedEdge)
	if err == nil {
		t.Fatal("Error should be returned if patch is incorrect")
	}
}
