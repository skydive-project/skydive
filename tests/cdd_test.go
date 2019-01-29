// +build cdd

/*
 * Copyright (C) 2017 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package tests

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	g "github.com/skydive-project/skydive/gremlin"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/tebeka/selenium"
)

func TestOverview(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	scale := gopath + "/src/github.com/skydive-project/skydive/scripts/scale.sh"
	port := strings.Split(analyzerListen, ":")[1]

	os.Setenv("PATH", fmt.Sprintf("%s/bin:%s", gopath, os.Getenv("PATH")))
	os.Setenv("ANALYZER_PORT", port)

	setupCmds := []Cmd{
		{fmt.Sprintf("%s start 1 4 2", scale), true},
	}

	tearDownCmds := []Cmd{
		{fmt.Sprintf("%s stop 1 4 2", scale), false},
	}

	execCmds(t, setupCmds...)
	defer execCmds(t, tearDownCmds...)

	// IP prefix set in the scale.sh script
	sa, err := common.ServiceAddressFromString(fmt.Sprintf("192.168.50.254:%s", port))
	if err != nil {
		t.Error(err)
		return
	}

	authOptions := &shttp.AuthenticationOpts{Username: "admin", Password: "password"}
	sh, err := newSeleniumHelper(t, sa.Addr, sa.Port, authOptions)
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

	if err = delaySec(5, sh.login()); err != nil {
		t.Error(err)
		return
	}

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

	verifyFlows := func() error {
		n1_eth0, err := sh.gh.GetNode(ng1_eth0.String())
		if err != nil {
			return err
		}
		n3_eth0, err := sh.gh.GetNode(ng3_eth0.String())
		if err != nil {
			return err
		}

		ipList1, err := n1_eth0.GetFieldStringList("IPV4")
		if err != nil {
			return err
		}
		ip1 := strings.Split(ipList1[0], "/")[0]

		ipList3, err := n3_eth0.GetFieldStringList("IPV4")
		if err != nil {
			return err
		}
		ip3 := strings.Split(ipList3[0], "/")[0]

		return common.Retry(func() error {
			// do not check the direction as first packet could have been not seen
			if err = sh.flowQuery(g.G.Flows().Has("Network", ip1, "Network", ip3)); err != nil {
				return err
			}

			time.Sleep(time.Second)

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

			if txt != ip1 && txt != ip3 {
				return fmt.Errorf("Network.A should be either '%s' or '%s' but got: %s", ip1, ip3, txt)
			}
			return nil
		}, 10, time.Second)
	}

	if err = verifyFlows(); err != nil {
		t.Error(err)
		return
	}
}
