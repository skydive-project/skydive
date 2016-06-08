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

package ovsdb

import (
	"testing"

	"github.com/socketplane/libovsdb"
)

type FakeBridgeHandler struct {
	BridgeUUID string
	Added      bool
	Deleted    bool
}

func (b *FakeBridgeHandler) OnOvsBridgeUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (b *FakeBridgeHandler) OnOvsBridgeAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	b.BridgeUUID = uuid
	b.Added = true
}

func (b *FakeBridgeHandler) OnOvsBridgeDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	b.BridgeUUID = uuid
	b.Deleted = true
}

func (b *FakeBridgeHandler) OnOvsInterfaceUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (b *FakeBridgeHandler) OnOvsInterfaceAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (b *FakeBridgeHandler) OnOvsInterfaceDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (b *FakeBridgeHandler) OnOvsPortUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (b *FakeBridgeHandler) OnOvsPortAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (b *FakeBridgeHandler) OnOvsPortDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func NewFakeBridgeHandler() FakeBridgeHandler {
	return FakeBridgeHandler{Added: false, Deleted: false}
}

func getTableUpdates(bridgeName string, op string) *libovsdb.TableUpdates {
	tableUpdates := &libovsdb.TableUpdates{}
	tableUpdates.Updates = make(map[string]libovsdb.TableUpdate)

	rows := make(map[string]libovsdb.RowUpdate)
	tableUpdates.Updates["Bridge"] = libovsdb.TableUpdate{Rows: rows}

	rowFields := make(map[string]interface{})
	rowFields["name"] = bridgeName + "-name"
	row := libovsdb.Row{Fields: rowFields}

	BridgeUUID := bridgeName + "-uuid"

	var rowUpdate libovsdb.RowUpdate

	switch op {
	case "add":
		rowUpdate = libovsdb.RowUpdate{UUID: libovsdb.UUID{GoUUID: BridgeUUID}, New: row}
	case "del":
		rowUpdate = libovsdb.RowUpdate{UUID: libovsdb.UUID{GoUUID: BridgeUUID}, Old: row}
	}

	rows[BridgeUUID] = rowUpdate

	return tableUpdates
}

func TestBridgeAdded(t *testing.T) {
	monitor := NewOvsMonitor("tcp", "127.0.0.1:8888")

	handler := NewFakeBridgeHandler()
	monitor.AddMonitorHandler(&handler)

	tableUpdates := getTableUpdates("bridge1", "add")
	monitor.updateHandler(tableUpdates)

	if handler.BridgeUUID != "bridge1-uuid" || handler.Added == false {
		t.Error("Bridge handler not called")
	}
}

func TestBridgeDeleted(t *testing.T) {
	monitor := NewOvsMonitor("tcp", "127.0.0.1:8888")

	handler := NewFakeBridgeHandler()
	monitor.AddMonitorHandler(&handler)

	tableUpdates := getTableUpdates("bridge1", "add")
	monitor.updateHandler(tableUpdates)

	tableUpdates = getTableUpdates("bridge1", "del")
	monitor.updateHandler(tableUpdates)

	if handler.BridgeUUID != "bridge1-uuid" || handler.Deleted == false {
		t.Error("Bridge handler not called")
	}
}

/* TODO(safchain) Add UT for interface adding */
