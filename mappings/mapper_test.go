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

	"github.com/redhat-cip/skydive/flow"
)

type FakeInterfaceDriver struct {
	FakeTenantID string
	FakeVNI      string
}

func (d *FakeInterfaceDriver) Enhance(mac string, attrs *flow.InterfaceAttributes) {
	attrs.TenantID = d.FakeTenantID
	attrs.VNI = d.FakeVNI
}

type FakeIfNameDriver struct {
	FakeIfName string
}

func (d *FakeIfNameDriver) Enhance(mac string, attrs *flow.InterfaceAttributes) {
	attrs.IfName = d.FakeIfName
}

func TestInterfacePipeline(t *testing.T) {
	fm := NewFlowMapper()

	tenantIDExpected := "tenant-id"
	VNIExpected := "vni"

	fd := &FakeInterfaceDriver{FakeTenantID: tenantIDExpected, FakeVNI: VNIExpected}
	im := NewInterfaceMapper([]InterfaceMappingDriver{fd})

	fm.SetInterfaceMapper(im)

	f := flow.New("127.0.0.1", 1, 2, nil)

	fm.Enhance([]*flow.Flow{f})
	if f.Attributes.IntfAttrSrc.IfIndex != 1 || f.Attributes.IntfAttrDst.IfIndex != 2 {
		t.Error("Original flow attributes ovverided")
	}
	if f.Attributes.IntfAttrSrc.TenantID != tenantIDExpected || f.Attributes.IntfAttrSrc.VNI != VNIExpected {
		t.Error("Flow src interface attrs not updated: ",
			f.Attributes.IntfAttrSrc, " expected ", tenantIDExpected, ", ", VNIExpected)
	}
	if f.Attributes.IntfAttrDst.TenantID != tenantIDExpected || f.Attributes.IntfAttrDst.VNI != VNIExpected {
		t.Error("Flow dst interface attrs not updated: ",
			f.Attributes.IntfAttrSrc, " expected ", tenantIDExpected, ", ", VNIExpected)
	}
}

func TestInterfacePipelineAddingIfNameDriver(t *testing.T) {
	fm := NewFlowMapper()

	tenantIDExpected := "tenant-id"
	VNIExpected := "vni"

	fd := &FakeInterfaceDriver{FakeTenantID: tenantIDExpected, FakeVNI: VNIExpected}
	im := NewInterfaceMapper([]InterfaceMappingDriver{fd})

	fm.SetInterfaceMapper(im)

	f := flow.New("127.0.0.1", 1, 2, nil)

	fm.Enhance([]*flow.Flow{f})
	if f.Attributes.IntfAttrSrc.IfIndex != 1 || f.Attributes.IntfAttrDst.IfIndex != 2 {
		t.Error("Original flow attributes ovverided")
	}
	if f.Attributes.IntfAttrSrc.TenantID != tenantIDExpected || f.Attributes.IntfAttrSrc.VNI != VNIExpected {
		t.Error("Flow src interface attrs not updated: ",
			f.Attributes.IntfAttrSrc, " expected ", tenantIDExpected, ", ", VNIExpected)
	}
	if f.Attributes.IntfAttrDst.TenantID != tenantIDExpected || f.Attributes.IntfAttrDst.VNI != VNIExpected {
		t.Error("Flow dst interface attrs not updated: ",
			f.Attributes.IntfAttrSrc, " expected ", tenantIDExpected, ", ", VNIExpected)
	}

	/* add a driver that will handles the IfName attribute */
	intfExpected := "eth0"
	id := &FakeIfNameDriver{FakeIfName: intfExpected}
	im.AddDriver(id)

	/* update the previous attributes */
	tenantIDExpected = "tenant-id2"
	VNIExpected = "vni2"

	fd.FakeTenantID = tenantIDExpected
	fd.FakeVNI = VNIExpected

	fm.Enhance([]*flow.Flow{f})
	if f.Attributes.IntfAttrSrc.TenantID != tenantIDExpected || f.Attributes.IntfAttrSrc.VNI != VNIExpected {
		t.Error("Flow src interface attrs updated: ",
			f.Attributes.IntfAttrSrc, " expected ", tenantIDExpected, ", ", VNIExpected)
	}
	if f.Attributes.IntfAttrDst.TenantID != tenantIDExpected || f.Attributes.IntfAttrDst.VNI != VNIExpected {
		t.Error("Flow dst interface attrs updated: ",
			f.Attributes.IntfAttrSrc, " expected ", tenantIDExpected, ", ", VNIExpected)
	}
	if f.Attributes.IntfAttrSrc.IfName != intfExpected {
		t.Error("Flow src interface name not updated: ",
			f.Attributes.IntfAttrSrc.IfName, " expected ", intfExpected)
	}
	if f.Attributes.IntfAttrDst.IfName != intfExpected {
		t.Error("Flow src interface name not updated: ",
			f.Attributes.IntfAttrDst.IfName, " expected ", intfExpected)
	}
}
