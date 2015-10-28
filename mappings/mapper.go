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
	//"fmt"

	"github.com/redhat-cip/skydive/flow"
)

type InterfaceMappingDriver interface {
	Enhance(mac string, attrs *flow.InterfaceAttributes)
}

type InterfaceMapper struct {
	Drivers []InterfaceMappingDriver
}

type FlowMapper struct {
	InterfaceMapper *InterfaceMapper
}

func (im *InterfaceMapper) AddDriver(driver InterfaceMappingDriver) {
	im.Drivers = append(im.Drivers, driver)
}

func (im *InterfaceMapper) Enhance(mac string, attrs *flow.InterfaceAttributes) {
	/* enhance interface attributes pipeline */
	for _, driver := range im.Drivers {
		driver.Enhance(mac, attrs)
	}
}

func NewInterfaceMapper(drivers []InterfaceMappingDriver) *InterfaceMapper {
	im := &InterfaceMapper{Drivers: drivers}
	return im
}

func (fm *FlowMapper) EnhanceInterfaces(flow *flow.Flow) {
	fm.InterfaceMapper.Enhance(flow.EtherSrc, &flow.Attributes.IntfAttrSrc)
	fm.InterfaceMapper.Enhance(flow.EtherDst, &flow.Attributes.IntfAttrDst)
}

func (fm *FlowMapper) Enhance(flow *flow.Flow) {
	fm.EnhanceInterfaces(flow)
}

func (fm *FlowMapper) SetInterfaceMapper(mapper *InterfaceMapper) {
	fm.InterfaceMapper = mapper
}

func (fm *FlowMapper) SetDefaultInterfaceMappingDrivers() error {
	drivers, err := GetDefaultInterfaceMappingDrivers()
	if err != nil {
		return err
	}
	im := NewInterfaceMapper(drivers)
	fm.SetInterfaceMapper(im)

	return nil
}

func NewFlowMapper() *FlowMapper {
	fm := &FlowMapper{}
	return fm
}

func GetDefaultInterfaceMappingDrivers() ([]InterfaceMappingDriver, error) {
	drivers := []InterfaceMappingDriver{}

	neutron, err := NewNeutronMapper()
	if err != nil {
		return drivers, err
	}
	drivers = append(drivers, neutron)

	netlink, err := NewNetLinkMapper()
	if err != nil {
		return drivers, err
	}
	drivers = append(drivers, netlink)

	return drivers, nil
}
