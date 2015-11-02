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

package mappings

import (
	"testing"

	"github.com/socketplane/libovsdb"

	"github.com/redhat-cip/skydive/flow"
)

func TestOvsEnhance(t *testing.T) {

	mapper, err := NewOvsMapper()
	if err != nil {
		t.Fatal(err)
	}

	/* add port */
	rowFields := make(map[string]interface{})
	rowFields["name"] = "eth0"
	row := libovsdb.Row{Fields: rowFields}
	rowUpdate := libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "port-uuid"}, New: row}

	mapper.OnOvsPortAdd(nil, "port-uuid", &rowUpdate)

	/* add bridge */
	rowFields = make(map[string]interface{})
	rowFields["name"] = "br0"
	uuid := libovsdb.UUID{GoUuid: "port-uuid"}
	rowFields["ports"] = libovsdb.OvsSet{GoSet: []interface{}{uuid}}
	row = libovsdb.Row{Fields: rowFields}
	rowUpdate = libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: "br0-uuid"}, New: row}

	mapper.OnOvsBridgeAdd(nil, "br0-uuid", &rowUpdate)

	attrs := flow.InterfaceAttributes{IfName: "eth0"}
	mapper.Enhance("", &attrs)

	if attrs.BridgeName != "br0" {
		t.Error("Bridge name not found, expected br0")
	}
}
