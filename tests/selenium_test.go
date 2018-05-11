// +build selenium

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
	"testing"
	"time"

	"github.com/tebeka/selenium"

	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestPacketInjectionCapture(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	topology := gopath + "/src/github.com/skydive-project/skydive/scripts/simple.sh"

	setupCmds := []helper.Cmd{
		{fmt.Sprintf("%s start 124.65.54.42/24 124.65.54.43/24", topology), true},
	}

	tearDownCmds := []helper.Cmd{
		{fmt.Sprintf("%s stop", topology), true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	ipaddr, err := getFirstAvailableIPv4Addr()
	if err != nil {
		t.Fatalf("Not able to find Analayzer addr: %v", err)
	}

	sh, err := newSeleniumHelper(t, ipaddr, 8082)
	if err != nil {
		t.Fatal(err)
	}
	defer sh.quit()

	if err = sh.connect(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Second)

	verifyFlows := func() error {
		time.Sleep(3 * time.Second)

		// do not check the direction as first packet could have been not seen
		if err = sh.flowQuery(g.G.Flows().Has("Network", "124.65.54.42", "Network", "124.65.54.43")); err != nil {
			return err
		}

		time.Sleep(2 * time.Second)

		flowRow, err := sh.findElement(selenium.ByClassName, "flow-row")
		if err != nil {
			return err
		}
		rowData, err := flowRow.FindElements(selenium.ByTagName, "td")
		if err != nil {
			return err
		}
		const expectedRowCount = 8
		if len(rowData) != expectedRowCount {
			return fmt.Errorf("By default %d rows should be return, but got: %d", expectedRowCount, len(rowData))
		}
		txt, err := rowData[1].Text()
		if err != nil {
			return err
		}
		if txt != "124.65.54.42" {
			return fmt.Errorf("Network.A should be '124.65.54.42' but got: %s", txt)
		}
		return nil
	}

	if err = sh.expand(); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	if err = sh.zoomFit(); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	if err = sh.expandGroup(g.G.V().Has("Name", "vm1", "Type", "netns")); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Error(err)
		return
	}
	time.Sleep(2 * time.Second)

	if err = sh.zoomFit(); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Error(err)
		return
	}
	time.Sleep(2 * time.Second)

	if err = sh.expandGroup(g.G.V().Has("Name", "vm2", "Type", "netns")); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Error(err)
		return
	}
	time.Sleep(2 * time.Second)

	if err = sh.zoomFit(); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Error(err)
		return
	}
	time.Sleep(2 * time.Second)

	if err = sh.startGremlinCapture(g.G.V().Has("Name", "br-int", "Type", "ovsbridge")); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Error(err)
		return
	}

	node1 := g.G.V().Has("Name", "eth0", "IPV4", "124.65.54.42/24")
	node2 := g.G.V().Has("Name", "eth0", "IPV4", "124.65.54.43/24")

	if err = sh.injectPacket(node1, node2, 0); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Error(err)
		return
	}

	if err = verifyFlows(); err != nil {
		if err := sh.screenshot("postmortem.png"); err != nil {
			t.Log(err)
		}

		t.Error(err)
		return
	}
}
