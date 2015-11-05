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
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/socketplane/libovsdb"

	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/ovs"
)

type OvsMapper struct {
	sync.Mutex
	IfNameToPort map[string]string
	PortToBridge map[string]string
}

func (mapper *OvsMapper) OnOvsBridgeAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	name, ok := row.New.Fields["name"]
	if !ok {
		return
	}

	mapper.Lock()
	defer mapper.Unlock()

	switch row.New.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid
			mapper.PortToBridge[u] = name.(string)
		}

	case libovsdb.UUID:
		u := row.New.Fields["ports"].(libovsdb.UUID).GoUuid
		mapper.PortToBridge[u] = name.(string)
	}
}

func (mapper *OvsMapper) OnOvsBridgeDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	mapper.Lock()
	defer mapper.Unlock()

	switch row.Old.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.Old.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid
			delete(mapper.PortToBridge, u)
		}
	case libovsdb.UUID:
		u := row.Old.Fields["ports"].(libovsdb.UUID).GoUuid

		delete(mapper.PortToBridge, u)
	}
}

func (mapper *OvsMapper) OnOvsInterfaceAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (mapper *OvsMapper) OnOvsInterfaceDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (mapper *OvsMapper) OnOvsPortAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	name := row.New.Fields["name"].(string)

	mapper.Lock()
	defer mapper.Unlock()

	mapper.IfNameToPort[name] = uuid
}

func (mapper *OvsMapper) OnOvsPortDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	name := row.Old.Fields["name"].(string)

	mapper.Lock()
	defer mapper.Unlock()

	delete(mapper.IfNameToPort, name)
}

func (mapper *OvsMapper) Enhance(mac string, attrs *flow.Flow_InterfaceAttributes) {
	mapper.Lock()
	defer mapper.Unlock()

	attrs.BridgeName = proto.String("")

	port, ok := mapper.IfNameToPort[attrs.GetIfName()]
	if !ok {
		return
	}

	bridge, ok := mapper.PortToBridge[port]
	if !ok {
		return
	}
	attrs.BridgeName = proto.String(bridge)
}

func NewOvsMapper() (*OvsMapper, error) {
	mapper := &OvsMapper{
		IfNameToPort: make(map[string]string),
		PortToBridge: make(map[string]string),
	}

	return mapper, nil
}
