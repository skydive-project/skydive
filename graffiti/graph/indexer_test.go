/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"testing"

	"github.com/skydive-project/skydive/filters"
)

func TestMetadataIndexer(t *testing.T) {
	g := newGraph(t)

	nodeFilter := NewElementFilter(filters.NewAndFilter(
		filters.NewNotNullFilter("TID"),
		filters.NewNotNullFilter("MAC"),
	))

	tidCache := NewMetadataIndexer(g, g, nodeFilter, "MAC")
	tidCache.Start()

	m1 := Metadata{
		"Type": "host",
		"MAC":  "00:11:22:33:44:55",
		"TID":  "123",
	}

	m2 := Metadata{
		"Type": "host",
		"MAC":  "55:44:33:22:11",
		"TID":  "321",
	}

	m3 := Metadata{
		"Type": "host",
		"MAC":  "01:23:45:67:89:00",
		"TID":  "456",
	}

	m4 := Metadata{
		"Type": "host",
		"MAC":  "55:44:33:22:11",
		"TID":  "456",
	}

	g.NewNode(GenID(), m1, "host")
	g.NewNode(GenID(), m2, "host")
	g.NewNode(GenID(), m3, "host")
	g.NewNode(GenID(), m4, "host")

	nodes, values := tidCache.Get("00:11:22:33:44:55")
	if len(nodes) != 1 || len(values) != 1 {
		t.Errorf("Expected one node and one value: got %+v, %+v", nodes, values)
		return
	}

	if tid, _ := nodes[0].GetFieldString("TID"); tid != "123" {
		t.Errorf("Expected one node with TID 123: got %+v", nodes[0])
	}

	nodes, values = tidCache.FromHash("55:44:33:22:11")
	if len(nodes) != 2 || len(values) != 2 {
		t.Errorf("Expected two nodes and two values: got %+v, %+v", nodes, values)
	}

	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", "docker"),
		filters.NewTermStringFilter("Type", "container"),
		filters.NewNotNullFilter("Docker.Labels.io.kubernetes.pod.name"),
		filters.NewNotNullFilter("Docker.Labels.io.kubernetes.pod.namespace"),
		filters.NewNotNullFilter("Docker.Labels.io.kubernetes.container.name"))
	m := NewElementFilter(filter)

	dockerCache := NewMetadataIndexer(g, g, m, "Docker.Labels.io.kubernetes.pod.namespace", "Docker.Labels.io.kubernetes.pod.name", "Docker.Labels.io.kubernetes.container.name")
	dockerCache.Start()

	m5 := Metadata{
		"Type":    "container",
		"Manager": "docker",
		"Docker": map[string]interface{}{
			"Labels": map[string]interface{}{
				"io": map[string]interface{}{
					"kubernetes": map[string]interface{}{
						"container": map[string]interface{}{
							"name": "mycontainer",
						},
						"pod": map[string]interface{}{
							"name":      "mypod",
							"namespace": "kube-system",
						},
					},
				},
			},
		},
	}

	g.NewNode(GenID(), m5, "host")

	nodes, _ = dockerCache.Get("kube-system", "mypod", "mycontainer")
	if len(nodes) != 1 {
		t.Errorf("Expected 1 one, got %+v", nodes)
	}
}

func TestMetadataIndexerWithArray(t *testing.T) {
	g := newGraph(t)

	vfCache := NewMetadataIndexer(g, g, nil, "VFS.MAC")
	vfCache.Start()

	macCache := NewMetadataIndexer(g, g, nil, "MAC")
	macCache.Start()

	vf1 := Metadata{
		"Name": "vf1",
		"VFS": []interface{}{
			map[string]interface{}{
				"MAC": "00:11:22:33:44:55",
			},
			map[string]interface{}{
				"MAC": "01:23:45:67:89:ab",
			},
		},
	}

	m1 := Metadata{
		"Name": "m1",
		"MAC":  "00:11:22:33:44:55",
	}

	m2 := Metadata{
		"Name": "m2",
		"MAC":  "01:23:45:67:89:ab",
	}

	n1, _ := g.NewNode(GenID(), vf1, "host")
	g.NewNode(GenID(), m1, "host")
	g.NewNode(GenID(), m2, "host")

	nodes, values := vfCache.Get("00:11:22:33:44:55")
	if len(nodes) != 1 || len(values) != 1 {
		t.Errorf("Expected one node and 1 value: got %+v, %+v", nodes, values)
	}

	nodes, values = vfCache.Get("01:23:45:67:89:ab")
	if len(nodes) != 1 || len(values) != 1 {
		t.Errorf("Expected one node and 1 value: got %+v, %+v", nodes, values)
	}

	g.SetMetadata(n1, Metadata{
		"Name": "vf1",
		"VFS": []interface{}{
			map[string]interface{}{
				"MAC": "00:11:22:33:44:55",
			},
		},
	})

	nodes, values = vfCache.Get("00:11:22:33:44:55")
	if len(nodes) != 1 || len(values) != 1 {
		t.Errorf("Expected one node and 1 value: got %+v, %+v", nodes, values)
	}

	nodes, values = vfCache.Get("01:23:45:67:89:ab")
	if len(nodes) != 0 || len(values) != 0 {
		t.Errorf("Expected zero node and zero value: got %+v, %+v", nodes, values)
	}

	g.SetMetadata(n1, Metadata{
		"Name": "vf1",
	})

	nodes, values = vfCache.Get("00:11:22:33:44:55")
	if len(nodes) != 0 || len(values) != 0 {
		t.Errorf("Expected zero node and zero value: got %+v, %+v", nodes, values)
	}

	nodes, values = vfCache.Get("01:23:45:67:89:ab")
	if len(nodes) != 0 || len(values) != 0 {
		t.Errorf("Expected zero node and zero value: got %+v, %+v", nodes, values)
	}
}
