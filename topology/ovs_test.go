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
	"testing"

	"github.com/socketplane/libovsdb"

	"github.com/redhat-cip/skydive/ovs"
)

func TestOvsTopology(t *testing.T) {
	ovsmon := ovsdb.NewOvsMonitor("127.0.0.1", 8888)
	topo := NewTopology()

	updater := NewOvsTopoUpdater(topo, ovsmon)

	/* add interface */
	rowFields := make(map[string]interface{})
	rowFields["name"] = "eth0.1"
	row := libovsdb.Row{Fields: rowFields}
	rowUpdate := libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "intf-uuid"}, New: row}

	updater.OnOvsInterfaceAdd(nil, "intf-uuid", &rowUpdate)

	/* add port */
	rowFields = make(map[string]interface{})
	rowFields["name"] = "eth0"
	uuid := libovsdb.UUID{GoUuid: "intf-uuid"}
	rowFields["interfaces"] = libovsdb.OvsSet{GoSet: []interface{}{uuid}}
	row = libovsdb.Row{Fields: rowFields}
	rowUpdate = libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "port-uuid"}, New: row}

	updater.OnOvsPortAdd(nil, "port-uuid", &rowUpdate)

	/* add bridge with already ports, simulate a initialisation */
	rowFields = make(map[string]interface{})
	rowFields["name"] = "br0"
	uuid = libovsdb.UUID{GoUuid: "port-uuid"}
	rowFields["ports"] = libovsdb.OvsSet{GoSet: []interface{}{uuid}}
	row = libovsdb.Row{Fields: rowFields}
	rowUpdate = libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "br0-uuid"}, New: row}

	updater.OnOvsBridgeAdd(nil, "br0-uuid", &rowUpdate)

	container := topo.GetContainer("br0")
	if container == nil {
		t.Error("Unable to find a container in the topo for the ovs bridge br0")
	}

	if container.Type != OvsBridge {
		t.Error("The container for the bridge br0 should be of type OvsBridge")
	}

	if container.GetPort("eth0") == nil {
		t.Error("Unable to find the port eth0 in the container br0 as expected")
	}

	updater.OnOvsInterfaceAdd(nil, "intf-uuid", &rowUpdate)

	if container.GetPort("eth0").GetInterface("eth0.1") == nil {
		t.Error("Unable to find the interface eth0 in the port eth0 as expected")
	}
}

func TestOvsOnBridgeAdd(t *testing.T) {
	ovsmon := ovsdb.NewOvsMonitor("127.0.0.1", 8888)
	topo := NewTopology()

	updater := NewOvsTopoUpdater(topo, ovsmon)

	/* add port */
	rowFields := make(map[string]interface{})
	rowFields["name"] = "br0"
	row := libovsdb.Row{Fields: rowFields}
	rowUpdate := libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "br0-uuid"}, New: row}

	updater.OnOvsPortAdd(nil, "br0-uuid", &rowUpdate)

	/* add new bridge */
	rowFields = make(map[string]interface{})
	rowFields["name"] = "br0"
	uuid := libovsdb.UUID{GoUuid: "br0-uuid"}
	rowFields["ports"] = uuid
	row = libovsdb.Row{Fields: rowFields}
	rowUpdate = libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "br0-uuid"}, New: row}

	updater.OnOvsBridgeAdd(nil, "br0-uuid", &rowUpdate)

	if topo.GetContainer("br0") == nil {
		t.Error("Unable to find a container in the topo for the ovs bridge br0")
	}
}

func TestOvsOnBridgeDel(t *testing.T) {
	ovsmon := ovsdb.NewOvsMonitor("127.0.0.1", 8888)
	topo := NewTopology()

	updater := NewOvsTopoUpdater(topo, ovsmon)

	/* add port */
	rowFields := make(map[string]interface{})
	rowFields["name"] = "br0"
	row := libovsdb.Row{Fields: rowFields}
	rowUpdate := libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "br0-uuid"}, New: row}

	updater.OnOvsPortAdd(nil, "br0-uuid", &rowUpdate)

	/* add new bridge */
	rowFields = make(map[string]interface{})
	rowFields["name"] = "br0"
	uuid := libovsdb.UUID{GoUuid: "br0-uuid"}
	rowFields["ports"] = uuid
	row = libovsdb.Row{Fields: rowFields}
	rowUpdate = libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "br0-uuid"}, New: row}

	updater.OnOvsBridgeAdd(nil, "br0-uuid", &rowUpdate)

	container := topo.GetContainer("br0")
	if container == nil {
		t.Error("Unable to find a container in the topo for the ovs bridge br0")
	}

	if container.Type != OvsBridge {
		t.Error("The container for the bridge br0 should be of type OvsBridge")
	}

	rowUpdate = libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "br0-uuid"}, Old: row}

	updater.OnOvsBridgeDel(nil, "br0-uuid", &rowUpdate)

	if topo.GetContainer("br0") != nil {
		t.Error("Container for the bridge br0 should exist anymore since the bridge has been deleted")
	}
}
