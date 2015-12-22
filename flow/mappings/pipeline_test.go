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

	"github.com/golang/protobuf/proto"

	"github.com/redhat-cip/skydive/flow"
)

type FakeInterfaceDriver struct {
	FakeTenantID string
	FakeVNI      uint64
}

func (d *FakeInterfaceDriver) Enhance(mac string, attrs *flow.Flow_InterfaceAttributes) {
	attrs.TenantID = proto.String(d.FakeTenantID)
	attrs.VNI = proto.Uint64(d.FakeVNI)
}

type FakeIfNameDriver struct {
	FakeIfName string
}

func (d *FakeIfNameDriver) Enhance(mac string, attrs *flow.Flow_InterfaceAttributes) {
	attrs.IfName = proto.String(d.FakeIfName)
}

func TestInterfacePipeline(t *testing.T) {
	fm := NewFlowMapper()

	tenantIDExpected := "tenant-id"
	VNIExpected := uint64(1008)

	fd := &FakeInterfaceDriver{FakeTenantID: tenantIDExpected, FakeVNI: VNIExpected}
	im := NewInterfaceMapper([]InterfaceMappingDriver{fd})

	fm.SetInterfaceMapper(im)

	f := flow.New("127.0.0.1", 1, 2, nil)

	fm.Enhance([]*flow.Flow{f})
	if f.GetAttributes().GetIntfAttrSrc().GetIfIndex() != 1 || f.GetAttributes().GetIntfAttrDst().GetIfIndex() != 2 {
		t.Error("Original flow attributes ovverided")
	}
	if f.GetAttributes().GetIntfAttrSrc().GetTenantID() != tenantIDExpected || f.GetAttributes().GetIntfAttrSrc().GetVNI() != VNIExpected {
		t.Error("Flow src interface attrs not updated: ",
			f.GetAttributes().GetIntfAttrSrc(), " expected ", tenantIDExpected, ", ", VNIExpected)
	}
	if f.GetAttributes().GetIntfAttrDst().GetTenantID() != tenantIDExpected || f.GetAttributes().GetIntfAttrDst().GetVNI() != VNIExpected {
		t.Error("Flow dst interface attrs not updated: ",
			f.GetAttributes().GetIntfAttrDst(), " expected ", tenantIDExpected, ", ", VNIExpected)
	}
}

func TestInterfacePipelineAddingIfNameDriver(t *testing.T) {
	fm := NewFlowMapper()

	tenantIDExpected := "tenant-id"
	VNIExpected := uint64(1008)

	fd := &FakeInterfaceDriver{FakeTenantID: tenantIDExpected, FakeVNI: VNIExpected}
	im := NewInterfaceMapper([]InterfaceMappingDriver{fd})

	fm.SetInterfaceMapper(im)

	f := flow.New("127.0.0.1", 1, 2, nil)

	fm.Enhance([]*flow.Flow{f})
	if f.GetAttributes().GetIntfAttrSrc().GetIfIndex() != 1 || f.GetAttributes().GetIntfAttrDst().GetIfIndex() != 2 {
		t.Error("Original flow attributes ovverided")
	}
	if f.GetAttributes().GetIntfAttrSrc().GetTenantID() != tenantIDExpected || f.GetAttributes().GetIntfAttrSrc().GetVNI() != VNIExpected {
		t.Error("Flow src interface attrs not updated: ",
			f.GetAttributes().GetIntfAttrSrc(), " expected ", tenantIDExpected, ", ", VNIExpected)
	}
	if f.GetAttributes().GetIntfAttrDst().GetTenantID() != tenantIDExpected || f.GetAttributes().GetIntfAttrDst().GetVNI() != VNIExpected {
		t.Error("Flow dst interface attrs not updated: ",
			f.GetAttributes().GetIntfAttrDst(), " expected ", tenantIDExpected, ", ", VNIExpected)
	}

	/* add a driver that will handles the IfName attribute */
	intfExpected := "eth0"
	id := &FakeIfNameDriver{FakeIfName: intfExpected}
	im.AddDriver(id)

	/* update the previous attributes */
	tenantIDExpected = "tenant-id2"
	VNIExpected = uint64(1009)

	fd.FakeTenantID = tenantIDExpected
	fd.FakeVNI = VNIExpected

	fm.Enhance([]*flow.Flow{f})
	if f.GetAttributes().GetIntfAttrSrc().GetTenantID() != tenantIDExpected || f.GetAttributes().GetIntfAttrSrc().GetVNI() != VNIExpected {
		t.Error("Flow src interface attrs updated: ",
			f.GetAttributes().GetIntfAttrSrc(), " expected ", tenantIDExpected, ", ", VNIExpected)
	}
	if f.GetAttributes().GetIntfAttrDst().GetTenantID() != tenantIDExpected || f.GetAttributes().GetIntfAttrDst().GetVNI() != VNIExpected {
		t.Error("Flow dst interface attrs updated: ",
			f.GetAttributes().GetIntfAttrDst(), " expected ", tenantIDExpected, ", ", VNIExpected)
	}
	if f.GetAttributes().GetIntfAttrSrc().GetIfName() != intfExpected {
		t.Error("Flow src interface name not updated: ",
			f.GetAttributes().GetIntfAttrSrc().GetIfName(), " expected ", intfExpected)
	}
	if f.GetAttributes().GetIntfAttrDst().GetIfName() != intfExpected {
		t.Error("Flow src interface name not updated: ",
			f.GetAttributes().GetIntfAttrDst().GetIfName(), " expected ", intfExpected)
	}
}
