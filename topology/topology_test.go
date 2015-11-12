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

	container1 := topo.NewNetNs("C1")
	container1.NewInterface("node-1", 0)
	container1.NewInterface("node-2", 0)
	intf3 := topo.NewInterface("node-3", 0)
	container1.AddInterface(intf3)

	container2 := topo.NewOvsBridge("C2")
	container2.NewPort("node-4")
	port5 := topo.NewPort("node-5")
	intf := port5.NewInterface("intf-1", 0)
	intf.SetMac("1.1.1.1.1.1")
	container2.AddPort(port5)

	expected := `{"OvsBridges":{"C2":{"Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"Mac":"1.1.1.1.1.1"}}}}}},`
	expected += `"NetNss":{"C1":{"Interfaces":{"node-1":{},"node-2":{},"node-3":{}}}}}`

	j, _ := json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}
}

func TestDeleteOvsBridge(t *testing.T) {
	topo := NewTopology()

	container1 := topo.NewNetNs("C1")
	container1.NewInterface("node-1", 0)
	container1.NewInterface("node-2", 0)
	intf3 := topo.NewInterface("node-3", 0)
	container1.AddInterface(intf3)

	container2 := topo.NewOvsBridge("C2")
	container2.NewPort("node-4")
	port5 := topo.NewPort("node-5")
	intf := port5.NewInterface("intf-1", 0)
	intf.SetMac("1.1.1.1.1.1")
	container2.AddPort(port5)

	expected := `{"OvsBridges":{"C2":{"Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"Mac":"1.1.1.1.1.1"}}}}}},`
	expected += `"NetNss":{"C1":{"Interfaces":{"node-1":{},"node-2":{},"node-3":{}}}}}`

	j, _ := json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}

	topo.DelOvsBridge("C2")

	expected = `{"OvsBridges":{},"NetNss":{"C1":{"Interfaces":{"node-1":{},"node-2":{},"node-3":{}}}}}`

	j, _ = json.Marshal(topo)
	if string(j) != expected {
		t.Error("Expected: ", expected, " Got: ", string(j))
	}
}

func TestSameInterfaceNameTwoPlaces(t *testing.T) {
	topo := NewTopology()

	container1 := topo.NewOvsBridge("C1")
	port1 := topo.NewPort("port")
	intf1 := port1.NewInterface("intf", 1)
	intf1.SetMac("1.1.1.1.1.1")
	container1.AddPort(port1)

	container2 := topo.NewNetNs("C2")
	intf2 := container2.NewInterface("intf", 2)
	intf2.SetMac("2.2.2.2.2.2")
	container2.AddInterface(intf2)

	if topo.LookupInterfaceByIndex(1) == topo.LookupInterfaceByIndex(2) {
		t.Error("The first interface has been overwritten by the second one")
	}
}

func TestSameInterfaceTwoPlaces(t *testing.T) {
	topo := NewTopology()

	container1 := topo.NewOvsBridge("C1")
	port1 := topo.NewPort("port")
	intf1 := port1.NewInterface("intf", 1)
	intf1.SetMac("1.1.1.1.1.1")
	container1.AddPort(port1)

	container2 := topo.NewNetNs("C2")
	intf2 := topo.LookupInterfaceByMac("1.1.1.1.1.1")
	container2.AddInterface(intf2)

	if container1.GetPort("port").GetInterface("intf") != container2.GetInterface("intf") {
		t.Error("Unable to find the same interface in both container")
	}
}
