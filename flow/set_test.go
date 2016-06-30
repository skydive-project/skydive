/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package flow

import "testing"

func TestWindowBandwidth(t *testing.T) {
	now := int64(1462962423)

	ft := NewTable(nil, nil)
	flows := GenerateTestFlows(t, ft, 0x4567, "probequery0")
	for i, f := range flows {
		randomizeLayerStats(t, int64(0x785612+i), now, f, FlowEndpointType_ETHERNET)
	}

	fbw := ft.Window(now-100, now+100).Bandwidth()
	fbwSeed0x4567 := FlowSetBandwidth{ABpackets: 392193, ABbytes: 225250394, BApackets: 278790, BAbytes: 238466148, Duration: 200, NBFlow: 10}
	if fbw != fbwSeed0x4567 {
		t.Fatal("flows Bandwidth didn't match\n", "fbw:", fbw, "fbwSeed0x4567:", fbwSeed0x4567)
	}

	fbw = ft.Window(0, now+100).Bandwidth()
	fbw10FlowZero := fbwSeed0x4567
	fbw10FlowZero.Duration = 1462962523
	if fbw != fbw10FlowZero {
		t.Fatal("flows Bandwidth should be zero for 10 flows", fbw, fbw10FlowZero)
	}

	fbw = ft.Window(0, 0).Bandwidth()
	fbwZero := FlowSetBandwidth{}
	if fbw != fbwZero {
		t.Fatal("flows Bandwidth should be zero", fbw, fbwZero)
	}

	fbw = ft.Window(now, now-1).Bandwidth()
	if fbw != fbwZero {
		t.Fatal("flows Bandwidth should be zero", fbw, fbwZero)
	}
	graphFlows(now, flows)

	fbw = ft.Window(now-89, now-89).Bandwidth()
	if fbw != fbwZero {
		t.Fatal("flows Bandwidth should be zero", fbw, fbwZero)
	}

	/* flow half window (2 sec) */
	winFlows := ft.Window(now-89-1, now-89+1)
	fbw = winFlows.Bandwidth()
	fbwFlow := FlowSetBandwidth{ABpackets: 239, ABbytes: 106266, BApackets: 551, BAbytes: 444983, Duration: 2, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow 2/3 window (3 sec) */
	winFlows = ft.Window(now-89-1, now-89+2)
	fbw = winFlows.Bandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 479, ABbytes: 212532, BApackets: 1102, BAbytes: 889966, Duration: 3, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow full window, 1 sec */
	winFlows = ft.Window(now-89, now-89+1)
	fbw = winFlows.Bandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 239, ABbytes: 106266, BApackets: 551, BAbytes: 444983, Duration: 1, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow full window shifted (+2), 2 sec */
	winFlows = ft.Window(now-89+2, now-89+4)
	fbw = winFlows.Bandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 479, ABbytes: 212532, BApackets: 1102, BAbytes: 889966, Duration: 2, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* 2 flows full window, 1 sec */
	winFlows = ft.Window(now-71, now-71+1)
	fbw = winFlows.Bandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 3956, ABbytes: 3154923, BApackets: 2052, BAbytes: 1879998, Duration: 1, NBFlow: 2}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 2 flows ", fbw, fbwFlow)
	}

	tags := make([]string, fbw.NBFlow)
	for i, fw := range winFlows.Flows {
		tags[i] = fw.GetLayerHash(FlowEndpointType_ETHERNET)
	}
	graphFlows(now, flows, tags...)

	winFlows = ft.Window(now-58, now-58+1)
	fbw = winFlows.Bandwidth()
	fbw4flows := FlowSetBandwidth{ABpackets: 33617, ABbytes: 31846830, BApackets: 7529, BAbytes: 9191349, Duration: 1, NBFlow: 4}
	if fbw != fbw4flows {
		t.Fatal("flows Bandwidth should be from 4 flows ", fbw, fbw4flows)
	}

	tags = make([]string, fbw.NBFlow)
	for i, fw := range winFlows.Flows {
		tags[i] = fw.GetLayerHash(FlowEndpointType_ETHERNET)
	}
	graphFlows(now, flows, tags...)
	t.Log(fbw)
}
