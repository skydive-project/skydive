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
	"bytes"
	"encoding/json"
	"testing"
	"text/template"
)

func TestSimpleTopology(t *testing.T) {
	topo := NewTopology("host-a")

	container1 := topo.NewNetNs("C1")
	container1.NewInterface("node-1", 0)
	container1.NewInterface("node-2", 0)
	intf3 := topo.NewInterfaceWithUUID("node-3", "node-3-uuid")
	container1.AddInterface(intf3)

	container2 := topo.NewOvsBridge("C2")
	container2.NewPort("node-4")
	port5 := topo.NewPort("node-5")
	intf := port5.NewInterface("intf-1", 0)
	intf.SetMac("1.1.1.1.1.1")
	container2.AddPort(port5)

	expected := `{"Host":"host-a","OvsBridges":{"C2":{"Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"UUID":"{{.UUID_INTF_1}}","Mac":"1.1.1.1.1.1"}}}}}},`
	expected += `"NetNss":{"C1":{"Interfaces":{"node-1":{"UUID":"{{.UUID_NODE_1}}"},"node-2":{"UUID":"{{.UUID_NODE_2}}"},"node-3":{"UUID":"node-3-uuid"}}}}}`

	var data = &struct {
		UUID_INTF_1 string
		UUID_NODE_1 string
		UUID_NODE_2 string
	}{
		UUID_INTF_1: intf.UUID,
		UUID_NODE_1: topo.LookupInterface(LookupByID("node-1"), OvsScope|NetNSScope).UUID,
		UUID_NODE_2: topo.LookupInterface(LookupByID("node-2"), OvsScope|NetNSScope).UUID,
	}

	tmpl, _ := template.New("str").Parse(expected)

	var str bytes.Buffer
	tmpl.Execute(&str, data)

	j, _ := json.Marshal(topo)
	if string(j) != str.String() {
		t.Error("Expected: ", str.String(), " Got: ", string(j))
	}
}

func TestDeleteOvsBridge(t *testing.T) {
	topo := NewTopology("host-a")

	container1 := topo.NewNetNs("C1")
	container1.NewInterface("node-1", 0)
	container1.NewInterface("node-2", 0)
	intf3 := topo.NewInterfaceWithUUID("node-3", "node-3-uuid")
	container1.AddInterface(intf3)

	container2 := topo.NewOvsBridge("C2")
	container2.NewPort("node-4")
	port5 := topo.NewPort("node-5")
	intf := port5.NewInterface("intf-1", 0)
	intf.SetMac("1.1.1.1.1.1")
	container2.AddPort(port5)

	expected := `{"Host":"host-a","OvsBridges":{"C2":{"Ports":{"node-4":{},"node-5":{"Interfaces":{"intf-1":{"UUID":"{{.UUID_INTF_1}}","Mac":"1.1.1.1.1.1"}}}}}},`
	expected += `"NetNss":{"C1":{"Interfaces":{"node-1":{"UUID":"{{.UUID_NODE_1}}"},"node-2":{"UUID":"{{.UUID_NODE_2}}"},"node-3":{"UUID":"node-3-uuid"}}}}}`

	var data = &struct {
		UUID_INTF_1 string
		UUID_NODE_1 string
		UUID_NODE_2 string
	}{
		UUID_INTF_1: intf.UUID,
		UUID_NODE_1: topo.LookupInterface(LookupByID("node-1"), OvsScope|NetNSScope).UUID,
		UUID_NODE_2: topo.LookupInterface(LookupByID("node-2"), OvsScope|NetNSScope).UUID,
	}

	tmpl, _ := template.New("str").Parse(expected)

	var str bytes.Buffer
	tmpl.Execute(&str, data)

	j, _ := json.Marshal(topo)
	if string(j) != str.String() {
		t.Error("Expected: ", str.String(), " Got: ", string(j))
	}

	topo.DelOvsBridge("C2")

	expected = `{"Host":"host-a","OvsBridges":{},`
	expected += `"NetNss":{"C1":{"Interfaces":{"node-1":{"UUID":"{{.UUID_NODE_1}}"},"node-2":{"UUID":"{{.UUID_NODE_2}}"},"node-3":{"UUID":"node-3-uuid"}}}}}`

	str.Reset()
	tmpl, _ = template.New("str").Parse(expected)
	tmpl.Execute(&str, data)

	j, _ = json.Marshal(topo)
	if string(j) != str.String() {
		t.Error("Expected: ", str.String(), " Got: ", string(j))
	}
}

func TestSameInterfaceNameTwoPlaces(t *testing.T) {
	topo := NewTopology("host-a")

	container1 := topo.NewOvsBridge("C1")
	port1 := topo.NewPort("port")
	intf1 := port1.NewInterface("intf", 1)
	intf1.SetMac("1.1.1.1.1.1")
	container1.AddPort(port1)

	container2 := topo.NewNetNs("C2")
	intf2 := container2.NewInterface("intf", 2)
	intf2.SetMac("2.2.2.2.2.2")
	container2.AddInterface(intf2)

	if topo.LookupInterface(LookupByIfIndex(1), NetNSScope|OvsScope) == topo.LookupInterface(LookupByIfIndex(2), NetNSScope|OvsScope) {
		t.Error("The first interface has been overwritten by the second one")
	}
}

func TestSameInterfaceTwoPlaces(t *testing.T) {
	topo := NewTopology("host-a")

	container1 := topo.NewOvsBridge("C1")
	port1 := topo.NewPort("port")
	intf1 := port1.NewInterface("intf", 1)
	intf1.SetMac("1.1.1.1.1.1")
	container1.AddPort(port1)

	container2 := topo.NewNetNs("C2")
	intf2 := topo.LookupInterface(LookupByMac("intf", "1.1.1.1.1.1"), OvsScope)
	container2.AddInterface(intf2)

	if container1.GetPort("port").GetInterface("intf") != container2.GetInterface("intf") {
		t.Error("Unable to find the same interface in both container")
	}
}
