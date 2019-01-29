/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package sflow

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	sm := &SFMetric{
		Last:               200,
		IfInOctets:         200,
		IfInUcastPkts:      200,
		IfInMulticastPkts:  200,
		IfInBroadcastPkts:  200,
		IfInDiscards:       200,
		IfInErrors:         200,
		IfInUnknownProtos:  200,
		IfOutOctets:        200,
		IfOutUcastPkts:     200,
		IfOutMulticastPkts: 200,
		IfOutBroadcastPkts: 200,
		IfOutDiscards:      200,
		IfOutErrors:        200,
		OvsdpNHit:          200,
		OvsdpNMissed:       200,
		OvsdpNLost:         200,
		OvsdpNMaskHit:      200,
		OvsdpNFlows:        200,
		OvsdpNMasks:        200,
		OvsAppFdOpen:       200,
		OvsAppFdMax:        200,
		OvsAppConnOpen:     200,
		OvsAppConnMax:      200,
		OvsAppMemUsed:      200,
		OvsAppMemMax:       200,
		VlanOctets:         200,
		VlanUcastPkts:      200,
		VlanMulticastPkts:  200,
		VlanBroadcastPkts:  200,
		VlanDiscards:       200,
	}

	sm1, sm2 := sm.Split(100)

	expected := &SFMetric{
		Start:              0,
		Last:               100,
		IfInOctets:         100,
		IfInUcastPkts:      100,
		IfInMulticastPkts:  100,
		IfInBroadcastPkts:  100,
		IfInDiscards:       100,
		IfInErrors:         100,
		IfInUnknownProtos:  100,
		IfOutOctets:        100,
		IfOutUcastPkts:     100,
		IfOutMulticastPkts: 100,
		IfOutBroadcastPkts: 100,
		IfOutDiscards:      100,
		IfOutErrors:        100,
		OvsdpNHit:          100,
		OvsdpNMissed:       100,
		OvsdpNLost:         100,
		OvsdpNMaskHit:      100,
		OvsdpNFlows:        100,
		OvsdpNMasks:        100,
		OvsAppFdOpen:       100,
		OvsAppFdMax:        100,
		OvsAppConnOpen:     100,
		OvsAppConnMax:      100,
		OvsAppMemUsed:      100,
		OvsAppMemMax:       100,
		VlanOctets:         100,
		VlanUcastPkts:      100,
		VlanMulticastPkts:  100,
		VlanBroadcastPkts:  100,
		VlanDiscards:       100,
	}

	if !reflect.DeepEqual(expected, sm1) {
		t.Errorf("Slice 1 error, expected %+v, got %+v", expected, sm1)
	}

	expected = &SFMetric{
		Start:              100,
		Last:               200,
		IfInOctets:         100,
		IfInUcastPkts:      100,
		IfInMulticastPkts:  100,
		IfInBroadcastPkts:  100,
		IfInDiscards:       100,
		IfInErrors:         100,
		IfInUnknownProtos:  100,
		IfOutOctets:        100,
		IfOutUcastPkts:     100,
		IfOutMulticastPkts: 100,
		IfOutBroadcastPkts: 100,
		IfOutDiscards:      100,
		IfOutErrors:        100,
		OvsdpNHit:          100,
		OvsdpNMissed:       100,
		OvsdpNLost:         100,
		OvsdpNMaskHit:      100,
		OvsdpNFlows:        100,
		OvsdpNMasks:        100,
		OvsAppFdOpen:       100,
		OvsAppFdMax:        100,
		OvsAppConnOpen:     100,
		OvsAppConnMax:      100,
		OvsAppMemUsed:      100,
		OvsAppMemMax:       100,
		VlanOctets:         100,
		VlanUcastPkts:      100,
		VlanMulticastPkts:  100,
		VlanBroadcastPkts:  100,
		VlanDiscards:       100,
	}

	if !reflect.DeepEqual(expected, sm2) {
		t.Errorf("Slice 2 error, expected %+v, got %+v", expected, sm2)
	}
}
