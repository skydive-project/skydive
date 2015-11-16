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
	//"github.com/golang/protobuf/proto"

	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/topology"
)

type OvsMapper struct {
	Topology *topology.Topology
}

func (mapper *OvsMapper) Enhance(mac string, attrs *flow.Flow_InterfaceAttributes) {
	/*if mapper.Topology != nil {
		f := func(intf *topology.Interface) bool {
			if intf.Mac == mac {
				return true
			}
			return false
		}

		bridge := u.NetNs.Topology.LookupInterface(f, topology.NetNSScope|topology.OvsScope)
		if bridge != nil {
			attrs.BridgeName = proto.String(bridge.ID)
			return
		}
	}
	attrs.BridgeName = proto.String("")*/
}

func NewOvsMapper(o *topology.Topology) (*OvsMapper, error) {
	mapper := &OvsMapper{
		Topology: o,
	}

	return mapper, nil
}
