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
	container1.NewPort("node-1")
	container1.NewPort("node-2")
	port3 := topo.NewPort("node-3")
	container1.AddPort(port3)

	container2 := topo.NewContainer("C2", NetNs)
	container2.NewPort("node-4")
	port5 := topo.NewPort("node-5")
	intf := port5.NewInterface("intf-1")
	intf.SetMac("1.1.1.1.1.1")
	container2.AddPort(port5)

	expected := `{"Containers":{"C1":{"Type":"root","Ports":{"node-1":{},"node-2":{},"node-3":{}}},`
	expected += `"C2":{"Type":"netns","Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"Type":"","Mac":"1.1.1.1.1.1"}}}}}}}`

	j, _ := json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}
}

func TestDeleteOperation(t *testing.T) {
	topo := NewTopology()

	container1 := topo.NewContainer("C1", Root)
	container1.NewPort("node-1")
	container1.NewPort("node-2")
	port3 := topo.NewPort("node-3")
	container1.AddPort(port3)

	container2 := topo.NewContainer("C2", NetNs)
	container2.NewPort("node-4")
	port5 := topo.NewPort("node-5")
	intf := port5.NewInterface("intf-1")
	intf.SetMac("1.1.1.1.1.1")
	container2.AddPort(port5)

	expected := `{"Containers":{"C1":{"Type":"root","Ports":{"node-1":{},"node-2":{},"node-3":{}}},`
	expected += `"C2":{"Type":"netns","Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"Type":"","Mac":"1.1.1.1.1.1"}}}}}}}`

	j, _ := json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}

	topo.DelContainer("C1")

	expected = `{"Containers":{"C2":{"Type":"netns","Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"Type":"","Mac":"1.1.1.1.1.1"}}}}}}}`

	j, _ = json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}
}

func TestSamePortInterfacNameDifferentContainer(t *testing.T) {
	topo := NewTopology()

	container1 := topo.NewContainer("C1", NetNs)
	port1 := topo.NewPort("port")
	intf1 := port1.NewInterfaceWithIndex("intf", 1)
	intf1.SetMac("1.1.1.1.1.1")
	container1.AddPort(port1)

	container2 := topo.NewContainer("C2", NetNs)
	port2 := topo.NewPort("port")
	intf2 := port2.NewInterfaceWithIndex("intf", 2)
	intf2.SetMac("2.2.2.2.2.2")
	container2.AddPort(port2)

	if topo.LookupInterfaceByIndex(1) == topo.LookupInterfaceByIndex(2) {
		t.Error("The first interface has been overwritten by the second one")
	}
}
