// +build cdd

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package tests

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/skydive-project/skydive/common"
	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestOverview(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	scale := gopath + "/src/github.com/skydive-project/skydive/scripts/scale.sh"
	port := strings.Split(helper.AnalyzerListen, ":")[1]

	os.Setenv("PATH", fmt.Sprintf("%s/bin:%s", gopath, os.Getenv("PATH")))
	os.Setenv("ANALYZER_PORT", port)

	setupCmds := []helper.Cmd{
		{fmt.Sprintf("%s start 1 4 2", scale), true},
	}

	tearDownCmds := []helper.Cmd{
		{fmt.Sprintf("%s stop 1 4 2", scale), false},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	// IP prefix set in the scale.sh script
	sa, err := common.ServiceAddressFromString(fmt.Sprintf("192.168.50.254:%s", port))
	if err != nil {
		t.Error(err)
		return
	}

	sh, err := newSeleniumHelper(t, sa.Addr, sa.Port)
	if err != nil {
		t.Error(err)
		return
	}
	defer sh.quit()

	if err = delaySec(5, sh.connect()); err != nil {
		t.Error(err)
		return
	}

	if err = delaySec(1, sh.enableFakeMousePointer()); err != nil {
		t.Error(err)
		return
	}

	// start recording
	sh.startVideoRecord("overview")
	defer sh.stopVideoRecord()

	if err = delaySec(5, sh.expand()); err != nil {
		t.Error(err)
		return
	}
	if err = delaySec(1, sh.zoomFit()); err != nil {
		t.Error(err)
		return
	}
	if err = delaySec(1, sh.expandGroup(g.G.V().Has("Name", "agent-1", "Type", "host").Out().Has("Name", "vm1", "Type", "netns"))); err != nil {
		t.Error(err)
		return
	}
	if err = delaySec(1, sh.zoomFit()); err != nil {
		t.Error(err)
		return
	}
	if err = delaySec(1, sh.expandGroup(g.G.V().Has("Name", "agent-3", "Type", "host").Out().Has("Name", "vm1", "Type", "netns"))); err != nil {
		t.Error(err)
		return
	}
	if err = delaySec(1, sh.zoomFit()); err != nil {
		t.Error(err)
		return
	}

	ng1_eth0 := g.G.V().Has("Name", "agent-1", "Type", "host").Out().Has("Name", "vm1", "Type", "netns").Out().Has("Name", "eth0")
	ng1_bridge := g.G.V().Has("Name", "agent-1", "Type", "host").Out().Has("Type", "ovsbridge")
	if err = sh.startShortestPathCapture(ng1_eth0, ng1_bridge, "icmp"); err != nil {
		t.Error(err)
		return
	}

	ng3_eth0 := g.G.V().Has("Name", "agent-3", "Type", "host").Out().Has("Name", "vm1", "Type", "netns").Out().Has("Name", "eth0")
	ng3_bridge := g.G.V().Has("Name", "agent-3", "Type", "host").Out().Has("Type", "ovsbridge")
	if err = sh.startShortestPathCapture(ng3_eth0, ng3_bridge, "icmp"); err != nil {
		t.Error(err)
		return
	}

	if err = delaySec(1, sh.injectPacket(ng1_eth0, ng3_eth0, 4)); err != nil {
		t.Error(err)
		return
	}

	if err = delaySec(1, sh.showNodeFlowTable(ng1_eth0)); err != nil {
		t.Error(err)
		return
	}
	if err = delaySec(1, sh.highlightFlow(ng1_eth0.Flows())); err != nil {
		t.Error(err)
		return
	}
	if err = delaySec(1, sh.clickOnFlow(ng1_eth0.Flows())); err != nil {
		t.Error(err)
		return
	}

	if err = delaySec(3, sh.scrollDownRightPanel()); err != nil {
		t.Error(err)
		return
	}
}
