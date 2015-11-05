/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package topology

import (
	"encoding/json"
	"testing"
)

func TestSimpleTopology(t *testing.T) {
	topo := NewTopology()

	container1 := topo.NewContainer("C1", Root)
	container1.NewNode("node-1")
	container1.NewNode("node-2")
	node := container1.NewNode("node-3")
	node.Metadatas["mac"] = "1.1.1.1.1.1"

	container2 := topo.NewContainer("C2", NetNs)
	container2.NewNode("node-4")
	container2.NewNode("node-5")

	expected := `{"Containers":{"C1":{"Type":"root","Nodes":{"node-1":{},"node-2":{},"node-3":{"Metadatas":{"mac":"1.1.1.1.1.1"}}}},`
	expected += `"C2":{"Type":"netns","Nodes":{"node-4":{},"node-5":{}}}}}`

	j, _ := json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}
}

func TestDeleteOperation(t *testing.T) {
	topo := NewTopology()

	container1 := topo.NewContainer("C1", Root)
	container1.NewNode("node-1")
	container1.NewNode("node-2")
	node := container1.NewNode("node-3")
	node.Metadatas["mac"] = "1.1.1.1.1.1"

	container2 := topo.NewContainer("C2", NetNs)
	container2.NewNode("node-4")
	container2.NewNode("node-5")

	expected := `{"Containers":{"C1":{"Type":"root","Nodes":{"node-1":{},"node-2":{},"node-3":{"Metadatas":{"mac":"1.1.1.1.1.1"}}}},`
	expected += `"C2":{"Type":"netns","Nodes":{"node-4":{},"node-5":{}}}}}`

	j, _ := json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}

	container1.DelNode("node-2")
	topo.DelContainer("C2")

	expected = `{"Containers":{"C1":{"Type":"root","Nodes":{"node-1":{},"node-3":{"Metadatas":{"mac":"1.1.1.1.1.1"}}}}}}`

	j, _ = json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}
}
