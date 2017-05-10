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
	"net"
	"os"
	"testing"
	"time"

	"github.com/tebeka/selenium"

	gclient "github.com/skydive-project/skydive/cmd/client"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestSelenium(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	topology := gopath + "/src/github.com/skydive-project/skydive/scripts/simple.sh"

	setupCmds := []helper.Cmd{
		{fmt.Sprintf("%s start 124.65.54.42/24 124.65.54.43/24", topology), true},
		{"sudo docker pull elgalu/selenium", true},
		{"sudo docker run -d --name=grid -p 4444:24444 -p 5900:25900 -e --shm-size=1g elgalu/selenium", true},
		{"docker exec grid wait_all_done 30s", true},
	}

	tearDownCmds := []helper.Cmd{
		{fmt.Sprintf("%s stop", topology), true},
		{"sudo docker exec grid stop", true},
		{"sudo docker stop grid", true},
		{"sudo docker rm grid", true},
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
		t.Fatal("Not able to find Analayzer addr: %v", err)
	}

	if err := webdriver.Get("http://" + ipaddr + ":8082"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)

	startCapture := func(wd selenium.WebDriver) error {
		captureTab, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='Captures']")
		if err != nil || captureTab == nil {
			return fmt.Errorf("Not found capture tab: %v", err)
		}
		if err := captureTab.Click(); err != nil {
			return fmt.Errorf("%v", err)
		}
		createBtn, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='create-capture']")
		if err != nil || createBtn == nil {
			return fmt.Errorf("Not found create button : %v", err)
		}
		if err := createBtn.Click(); err != nil {
			return fmt.Errorf("%v", err)
		}
		time.Sleep(2 * time.Second)

		gremlinRdoBtn, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='by-gremlin']")
		if err != nil || gremlinRdoBtn == nil {
			return fmt.Errorf("Not found gremlin expression radio button: %v", err)
		}
		if err := gremlinRdoBtn.Click(); err != nil {
			return err
		}

		queryTxtBox, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='capture-query']")
		if err != nil || queryTxtBox == nil {
			return fmt.Errorf("Not found Query text box: %v", err)
		}
		if err := queryTxtBox.Clear(); err != nil {
			return err
		}
		if err := queryTxtBox.SendKeys("G.V().Has('Name', 'br-int', 'Type', 'ovsbridge')"); err != nil {
			return err
		}

		startBtn, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='start-capture']")
		if err != nil || startBtn == nil {
			return fmt.Errorf("Not found start button: %v", err)
		}
		if err := startBtn.Click(); err != nil {
			return err
		}
		time.Sleep(3 * time.Second)

		//check capture created with the given query
		captures, err := wd.FindElements(selenium.ByClassName, "query")
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

	injectPacket := func(wd selenium.WebDriver) error {
		generatorTab, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='Generator']")
		if err != nil || generatorTab == nil {
			return fmt.Errorf("Generator tab not found: %v", err)
		}
		if err := generatorTab.Click(); err != nil {
			return err
		}

		injectSrc, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='inject-src']/input")
		if err != nil || injectSrc == nil {
			return err
		}
		if err := injectSrc.Click(); err != nil {
			return err
		}

		authOptions := &shttp.AuthenticationOpts{}
		gh := gclient.NewGremlinQueryHelper(authOptions)

		node1, err := gh.GetNode("G.V().Has('Name', 'eth0', 'IPV4', Contains('124.65.54.42/24')).HasKey('TID')")
		if err != nil {
			return err
		}

		node2, err := gh.GetNode("G.V().Has('Name', 'eth0', 'IPV4', Contains('124.65.54.43/24')).HasKey('TID')")
		if err != nil {
			return err
		}

		tid1, _ := node1.GetFieldString("TID")
		tid2, _ := node2.GetFieldString("TID")

		srcNode, err := wd.FindElement(selenium.ByXPATH, ".//*[@tid='"+tid1+"']")
		if err != nil || srcNode == nil {
			return err
		}
		if err := srcNode.Click(); err != nil {
			return err
		}

		injectDst, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='inject-dst']/input")
		if err != nil || injectDst == nil {
			return err
		}
		if err := injectDst.Click(); err != nil {
			return err
		}
		dstNode, err := wd.FindElement(selenium.ByXPATH, ".//*[@tid='"+tid2+"']")
		if err != nil || dstNode == nil {
			return err
		}
		if err := dstNode.Click(); err != nil {
			return err
		}

		injectBtn, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='inject']")
		if err != nil || injectBtn == nil {
			return nil
		}
		if err := injectBtn.Click(); err != nil {
			return err
		}

		var alertMsg selenium.WebElement
		for i := 1; i <= 10; i++ {
			alertMsg, err = wd.FindElement(selenium.ByClassName, "alert-success")
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			break
		}
		if alertMsg != nil {
			closeBtn, _ := alertMsg.FindElement(selenium.ByClassName, "close")
			if closeBtn != nil {
				closeBtn.Click()
			}
		} else {
			return fmt.Errorf("No success alert msg.")
		}
		return nil
	}

	verifyFlows := func(wd selenium.WebDriver) error {
		time.Sleep(3 * time.Second)

		flowsTab, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='Flows']")
		if err != nil || flowsTab == nil {
			return fmt.Errorf("Flows tab not found: %v", err)
		}
		if err := flowsTab.Click(); err != nil {
			return err
		}

		flowQuery, err := wd.FindElement(selenium.ByXPATH, ".//*[@id='flow-table-query']")
		if err != nil || flowQuery == nil {
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

		flowRow, err := wd.FindElement(selenium.ByClassName, "flow-row")
		if err != nil || flowRow == nil {
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
			fmt.Errorf("Network.A should be '124.65.54.42' but got: %s", txt)
		}
		return nil
	}

	if err := startCapture(webdriver); err != nil {
		t.Fatal(err)
	}

	if err := injectPacket(webdriver); err != nil {
		t.Fatal(err)
	}

	if err := verifyFlows(webdriver); err != nil {
		t.Fatal(err)
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
