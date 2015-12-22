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
	"github.com/redhat-cip/skydive/flow"
)

type InterfaceDriver interface {
	Enhance(mac string, attrs *flow.Flow_InterfaceAttributes)
}

type InterfacePipeline struct {
	Drivers []InterfaceDriver
}

type MappingPipeline struct {
	InterfacePipeline *InterfacePipeline
}

func (ip *InterfacePipeline) Enhance(mac string, attrs *flow.Flow_InterfaceAttributes) {
	for _, driver := range ip.Drivers {
		driver.Enhance(mac, attrs)
	}
}

func NewInterfacePipeline(drivers []InterfaceDriver) *InterfacePipeline {
	return &InterfacePipeline{
		Drivers: drivers,
	}
}

func (mp *MappingPipeline) EnhanceInterfaces(flow *flow.Flow) {
	mp.InterfacePipeline.Enhance(flow.GetEtherSrc(), flow.GetIfAttributes().GetIfAttrsSrc())
	mp.InterfacePipeline.Enhance(flow.GetEtherDst(), flow.GetIfAttributes().GetIfAttrsDst())
}

func (mp *MappingPipeline) Enhance(flows []*flow.Flow) {
	for _, flow := range flows {
		mp.EnhanceInterfaces(flow)
	}
}

func NewMappingPipeline(ip *InterfacePipeline) *MappingPipeline {
	return &MappingPipeline{
		InterfacePipeline: ip,
	}
}
