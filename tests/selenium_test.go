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
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/tebeka/selenium"

	gclient "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestSelenium(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	topology := gopath + "/src/github.com/skydive-project/skydive/scripts/simple.sh"

	setupCmds := []helper.Cmd{
		{fmt.Sprintf("%s start 124.65.54.42/24 124.65.54.43/24", topology), true},
		{"docker pull elgalu/selenium", true},
		{"docker run -d --name=grid -p 4444:24444 -p 5900:25900 -e --shm-size=1g -p 6080:26080 -e SCREEN_WIDTH=1600 -e SCREEN_HEIGHT=1400 -e NOVNC=true elgalu/selenium", true},
		{"docker exec grid wait_all_done 30s", true},
	}

	tearDownCmds := []helper.Cmd{
		{fmt.Sprintf("%s stop", topology), true},
		{"docker exec grid stop-video", true},
		{"docker cp grid:/videos/. .", true},
		{"docker stop grid", true},
		{"docker rm -f grid", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	caps := selenium.Capabilities{"browserName": "chrome"}
	webdriver, err := selenium.NewRemote(caps, "http://127.0.0.1:4444/wd/hub")
	if err != nil {
		t.Fatal(err)
	}
	defer webdriver.Quit()

	ipaddr, err := getIPv4Addr()
	if err != nil {
		t.Fatalf("Not able to find Analayzer addr: %v", err)
	}

	if err := webdriver.Get("http://" + ipaddr + ":8082"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Second)

	authOptions := &shttp.AuthenticationOpts{}
	gh := gclient.NewGremlinQueryHelper(authOptions)

	findElement := func(selection, xpath string) (el selenium.WebElement, err error) {
		common.Retry(func() error {
			el, err = webdriver.FindElement(selection, xpath)
			if err != nil || el == nil {
				return fmt.Errorf("Failed to find element for %s (error: %+v)", xpath, err)
			}
			return nil
		}, 10, time.Second)
		return
	}

	zoomOut := func() error {
		for i := 0; i != 5; i++ {
			zo, err := findElement(selenium.ByID, "zoom-out")
			if err != nil {
				return err
			}
			if err = zo.Click(); err != nil {
				return err
			}
		}
		return nil
	}

	expandGroup := func(gremlin string) error {
		node, err := gh.GetNode(gremlin)
		if err != nil {
			return err
		}
		if err = webdriver.KeyDown(selenium.AltKey); err != nil {
			return err
		}

		err = common.Retry(func() error {
			el, err := findElement(selenium.ByXPATH, ".//*[@id='node-"+string(node.ID)+"']")
			if err != nil {
				return err
			}

			if err = el.Click(); err != nil {
				zoomOut()
				return err
			}

			if collapsed, err := el.GetAttribute("collapsed"); err != nil || collapsed != "false" {
				return errors.New("group still collapsed")
			}

			return nil
		}, 10, time.Second)

		if err != nil {
			return err
		}

		if err = webdriver.KeyUp(selenium.AltKey); err != nil {
			return err
		}

		return nil
	}

	selectNode := func(gremlin string) error {
		node, err := gh.GetNode(gremlin)
		if err != nil {
			return err
		}

		el, err := findElement(selenium.ByXPATH, ".//*[@id='node-"+string(node.ID)+"']")
		if err != nil {
			return err
		}
		return common.Retry(func() error {
			if err := el.Click(); err != nil {
				zoomOut()
				return fmt.Errorf("Failed to click on source node: %s", err.Error())
			}
			return nil
		}, 10, time.Second)
	}

	startCapture := func() error {
		captureTab, err := webdriver.FindElement(selenium.ByXPATH, ".//*[@id='Captures']")
		if err != nil || captureTab == nil {
			return fmt.Errorf("Not found capture tab: %v", err)
		}
		if err := captureTab.Click(); err != nil {
			return fmt.Errorf("%v", err)
		}
		createBtn, err := webdriver.FindElement(selenium.ByXPATH, ".//*[@id='create-capture']")
		if err != nil || createBtn == nil {
			return fmt.Errorf("Not found create button : %v", err)
		}
		if err := createBtn.Click(); err != nil {
			return fmt.Errorf("%v", err)
		}
		time.Sleep(2 * time.Second)

		gremlinRdoBtn, err := webdriver.FindElement(selenium.ByXPATH, ".//*[@id='by-gremlin']")
		if err != nil || gremlinRdoBtn == nil {
			return fmt.Errorf("Not found gremlin expression radio button: %v", err)
		}
		if err := gremlinRdoBtn.Click(); err != nil {
			return err
		}

		queryTxtBox, err := webdriver.FindElement(selenium.ByXPATH, ".//*[@id='capture-query']")
		if err != nil || queryTxtBox == nil {
			return fmt.Errorf("Not found Query text box: %v", err)
		}
		if err := queryTxtBox.Clear(); err != nil {
			return err
		}
		if err := queryTxtBox.SendKeys("G.V().Has('Name', 'br-int', 'Type', 'ovsbridge')"); err != nil {
			return err
		}

		startBtn, err := webdriver.FindElement(selenium.ByXPATH, ".//*[@id='start-capture']")
		if err != nil || startBtn == nil {
			return fmt.Errorf("Not found start button: %v", err)
		}
		if err := startBtn.Click(); err != nil {
			return err
		}
		time.Sleep(3 * time.Second)

		//check capture created with the given query
		captures, err := webdriver.FindElements(selenium.ByClassName, "query")
		if err != nil {
			return err
		}
		var foundCapture bool
		for _, capture := range captures {
			if txt, _ := capture.Text(); txt == "G.V().Has('Name', 'br-int', 'Type', 'ovsbridge')" {
				foundCapture = true
				break
			}

		}
		if !foundCapture {
			return fmt.Errorf("Capture not found in the list")
		}
		return nil
	}

	injectPacket := func() error {
		generatorTab, err := findElement(selenium.ByXPATH, ".//*[@id='Generator']")
		if err != nil {
			return err
		}

		err = common.Retry(func() error {
			return generatorTab.Click()
		}, 10, time.Second)
		if err != nil {
			return fmt.Errorf("Could not click on generator tab: %s", err.Error())
		}

		injectSrc, err := findElement(selenium.ByXPATH, ".//*[@id='inject-src']/input")
		if err != nil {
			return err
		}

		if err := injectSrc.Click(); err != nil {
			return fmt.Errorf("Failed to click on inject input: %s", err.Error())
		}

		if err = selectNode("G.V().Has('Name', 'eth0', 'IPV4', Contains('124.65.54.42/24'))"); err != nil {
			return err
		}

		injectDst, err := findElement(selenium.ByXPATH, ".//*[@id='inject-dst']/input")
		if err != nil {
			return err
		}
		if err := injectDst.Click(); err != nil {
			return fmt.Errorf("Failed to click on destination input: %s", err.Error())
		}

		if err = selectNode("G.V().Has('Name', 'eth0', 'IPV4', Contains('124.65.54.43/24'))"); err != nil {
			return err
		}

		injectBtn, err := findElement(selenium.ByXPATH, ".//*[@id='inject']")
		if err != nil {
			return err
		}
		if err := injectBtn.Click(); err != nil {
			return fmt.Errorf("Failed to click on inject button: %s", err.Error())
		}

		var alertMsg selenium.WebElement
		err = common.Retry(func() error {
			alertMsg, err = findElement(selenium.ByClassName, "alert-success")
			return err
		}, 10, time.Second)
		if err != nil {
			return err
		}

		closeBtn, _ := alertMsg.FindElement(selenium.ByClassName, "close")
		if closeBtn != nil {
			closeBtn.Click()
		}

		return nil
	}

	verifyFlows := func() error {
		time.Sleep(3 * time.Second)

		flowsTab, err := findElement(selenium.ByXPATH, ".//*[@id='Flows']")
		if err != nil {
			return fmt.Errorf("Flows tab not found: %v", err)
		}
		if err := flowsTab.Click(); err != nil {
			return err
		}

		flowQuery, err := findElement(selenium.ByXPATH, ".//*[@id='flow-table-query']")
		if err != nil {
			return err
		}
		if err := flowQuery.Clear(); err != nil {
			return err
		}
		query := "G.Flows().Has('Network.A', '124.65.54.42', 'Network.B', '124.65.54.43')"
		if err := flowQuery.SendKeys(query); err != nil {
			return err
		}

		time.Sleep(2 * time.Second)

		flowRow, err := findElement(selenium.ByClassName, "flow-row")
		if err != nil {
			return err
		}
		rowData, err := flowRow.FindElements(selenium.ByTagName, "td")
		if err != nil {
			return err
		}
		if len(rowData) != 7 {
			return fmt.Errorf("By default 7 rows should be return")
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

	takeScreenshot := func(path string) error {
		t.Logf("Taking screenshot %s", path)
		content, err := webdriver.Screenshot()
		if err != nil {
			return err
		}

		f, err := os.Create(path)
		if err != nil {
			return err
		}

		if _, err = f.Write(content); err != nil {
			return err
		}

		return f.Close()
	}

	if helper.RecordVideo {
		helper.ExecCmds(t, helper.Cmd{"docker exec grid start-video", true})
	}

	// expand the topology to be sure to find nodes
	expand, err := findElement(selenium.ByID, "expand-collapse")
	if err != nil {
		if err := takeScreenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}
	expand.Click()

	fit, err := findElement(selenium.ByID, "zoom-fit")
	if err != nil {
		if err := takeScreenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}
	fit.Click()

	if err = expandGroup("G.V().Has('Name', 'vm1', 'Type', 'netns')"); err != nil {
		if err := takeScreenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)
	fit.Click()

	if err = expandGroup("G.V().Has('Name', 'vm2', 'Type', 'netns')"); err != nil {
		if err := takeScreenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)
	fit.Click()

	if err := startCapture(); err != nil {
		if err := takeScreenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}

	if err := injectPacket(); err != nil {
		if err := takeScreenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}

	if err := verifyFlows(); err != nil {
		if err := takeScreenshot("postmortem.png"); err != nil {
			t.Log(err)
		}
		t.Fatal(err)
	}

	if helper.RecordVideo {
		helper.ExecCmds(t, helper.Cmd{"docker exec grid stop-video", true})
	}
}

func getIPv4Addr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		//neglect interfaces which are down
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		//neglect loopback interface
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch t := addr.(type) {
			case *net.IPNet:
				ip = t.IP
			case *net.IPAddr:
				ip = t.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip != nil {
				return ip.String(), nil
			}
		}
	}
	return "", fmt.Errorf("No IP found")
}
