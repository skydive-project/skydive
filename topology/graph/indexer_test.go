/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"testing"

	"github.com/skydive-project/skydive/filters"
)

func TestMetadataIndexer(t *testing.T) {
	b, err := NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	g := NewGraphFromConfig(b)

	nodeFilter := NewGraphElementFilter(filters.NewAndFilter(
		filters.NewNotNullFilter("TID"),
		filters.NewNotNullFilter("MAC"),
	))

	tidCache := NewMetadataIndexer(g, nodeFilter, "MAC")
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
	m := NewGraphElementFilter(filter)

	dockerCache := NewMetadataIndexer(g, m, "Docker.Labels.io.kubernetes.pod.namespace", "Docker.Labels.io.kubernetes.pod.name", "Docker.Labels.io.kubernetes.container.name")
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
