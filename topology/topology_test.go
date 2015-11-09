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
	topo.NewPort("node-1", container1)
	topo.NewPort("node-2", container1)
	port := topo.NewPort("node-3", container1)
	port.Metadatas["mac"] = "1.1.1.1.1.1"

	container2 := topo.NewContainer("C2", NetNs)
	topo.NewPort("node-4", container2)
	port5 := topo.NewPort("node-5", container2)
	topo.NewInterface("intf-1", port5)

	expected := `{"Containers":{"C1":{"Type":"root","Ports":{"node-1":{},"node-2":{},"node-3":{"Metadatas":{"mac":"1.1.1.1.1.1"}}}},`
	expected += `"C2":{"Type":"netns","Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"Metadatas":{}}}}}}}}`

	j, _ := json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}
}

func TestDeleteOperation(t *testing.T) {
	topo := NewTopology()

	container1 := topo.NewContainer("C1", Root)
	topo.NewPort("node-1", container1)
	topo.NewPort("node-2", container1)
	port := topo.NewPort("node-3", container1)
	port.Metadatas["mac"] = "1.1.1.1.1.1"

	container2 := topo.NewContainer("C2", NetNs)
	topo.NewPort("node-4", container2)
	port5 := topo.NewPort("node-5", container2)
	topo.NewInterface("intf-1", port5)

	expected := `{"Containers":{"C1":{"Type":"root","Ports":{"node-1":{},"node-2":{},"node-3":{"Metadatas":{"mac":"1.1.1.1.1.1"}}}},`
	expected += `"C2":{"Type":"netns","Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"Metadatas":{}}}}}}}}`

	j, _ := json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}

	topo.DelContainer("C1")

	expected = `{"Containers":{"C2":{"Type":"netns","Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"Metadatas":{}}}}}}}}`

	j, _ = json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}
}
