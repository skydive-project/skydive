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

package tests

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/tebeka/selenium"

	gclient "github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
)

type seleniumHelper struct {
	addr             string
	port             int
	webdriver        selenium.WebDriver
	gh               *gclient.GremlinQueryHelper
	fakeMousePointer bool
	activeTabID      string
	t                *testing.T
	currVideoName    string
}

func (s *seleniumHelper) connect() error {
	return s.webdriver.Get(fmt.Sprintf("http://%s:%d", s.addr, s.port))
}

func (s *seleniumHelper) findElement(selection, xpath string) (el selenium.WebElement, err error) {
	common.Retry(func() error {
		el, err = s.webdriver.FindElement(selection, xpath)
		if err != nil || el == nil {
			return fmt.Errorf("Failed to find element for %s (error: %+v)", xpath, err)
		}
		return nil
	}, 10, time.Second)

	return
}

func (s *seleniumHelper) screenshot(path string) error {
	content, err := s.webdriver.Screenshot()
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

func (s *seleniumHelper) zoomFit() error {
	fit, err := s.findElement(selenium.ByID, "zoom-fit")
	if err != nil {
		return err
	}
	fit.Click()

	return nil
}

func (s *seleniumHelper) zoomOut() error {
	for i := 0; i != 5; i++ {
		zo, err := s.findElement(selenium.ByID, "zoom-out")
		if err != nil {
			return err
		}
		if err = zo.Click(); err != nil {
			return err
		}
	}
	return nil
}

func (s *seleniumHelper) clickOnNode(gremlin string) error {
	node, err := s.gh.GetNode(gremlin)
	if err != nil {
		return err
	}

	return common.Retry(func() error {
		el, err := s.webdriver.FindElement(selenium.ByXPATH, ".//*[@id='node-img-"+string(node.ID)+"']")
		if err != nil {
			return err
		}
		if err := s.clickOn(el); err != nil {
			return fmt.Errorf("Failed to click on source node: %s", err.Error())
		}
		return nil
	}, 20, 200*time.Millisecond)
}

func (s *seleniumHelper) expand() error {
	expand, err := s.findElement(selenium.ByID, "expand")
	if err != nil {
		return err
	}

	if err = s.clickOn(expand); err != nil {
		return err
	}

	return nil
}

func (s *seleniumHelper) expandGroup(gremlin string) error {
	node, err := s.gh.GetNode(gremlin)
	if err != nil {
		return err
	}
	if err = s.webdriver.KeyDown(selenium.AltKey); err != nil {
		return err
	}

	err = common.Retry(func() error {
		el, err := s.findElement(selenium.ByXPATH, ".//*[@id='node-img-"+string(node.ID)+"']")
		if err != nil {
			return err
		}

		if err = s.clickOn(el); err != nil {
			return err
		}

		el, err = s.findElement(selenium.ByXPATH, ".//*[@id='node-"+string(node.ID)+"']")
		if err != nil {
			return err
		}

		if collapsed, err := el.GetAttribute("collapsed"); err != nil || collapsed != "false" {
			return errors.New("group still collapsed")
		}

		return nil
	}, 20, 10*time.Millisecond)

	if err != nil {
		return err
	}

	if err = s.webdriver.KeyUp(selenium.AltKey); err != nil {
		return err
	}

	return nil
}

func (s *seleniumHelper) getFlowRow(gremlin string) (selenium.WebElement, error) {
	var flows []*flow.Flow
	var err error

	retry := func() error {
		flows, err = s.gh.GetFlows(gremlin)
		if err != nil {
			return err
		}

		if len(flows) == 0 {
			return errors.New("No flow found")
		}

		return nil
	}
	if err = common.Retry(retry, 20, 200*time.Millisecond); err != nil {
		time.Sleep(5 * time.Minute)

		return nil, err
	}

	// try to move to one of the flow
	var el selenium.WebElement
	retry = func() error {
		for _, f := range flows {
			el, err = s.webdriver.FindElement(selenium.ByXPATH, fmt.Sprintf(".//*[@id='flow-%s']", f.UUID))
			if el != nil {
				return nil
			}
		}
		return errors.New("Not found")
	}
	if err := common.Retry(retry, 20, 200*time.Millisecond); err != nil {
		return nil, err
	}

	return el, nil
}

func (s *seleniumHelper) highlightFlow(gremlin string) error {
	el, err := s.getFlowRow(gremlin)
	if err != nil {
		return err
	}

	return s.moveOn(el)
}

func (s *seleniumHelper) scrollDownRightPanel() error {
	s.webdriver.ExecuteScript("$('#right-panel').animate({scrollTop: $('#right-panel').get(0).scrollHeight}, 500);", nil)
	return nil
}

func (s *seleniumHelper) clickOnFlow(gremlin string) error {
	el, err := s.getFlowRow(gremlin)
	if err != nil {
		return err
	}

	return s.clickOn(el)
}

func (s *seleniumHelper) showNodeFlowTable(gremlin string) error {
	if err := s.clickOnNode(gremlin); err != nil {
		return err
	}

	s.scrollDownRightPanel()

	return nil
}

func (s *seleniumHelper) selectNode(id string, gremlin string) error {
	if err := s.clickOnByID(id); err != nil {
		return err
	}
	if err := s.clickOnNode(gremlin); err != nil {
		return err
	}

	return nil
}

func (s *seleniumHelper) fillTextBoxByID(id string, text string) error {
	box, err := s.findElement(selenium.ByXPATH, fmt.Sprintf(".//*[@id='%s']", id))
	if err != nil || box == nil {
		return fmt.Errorf("Not found text box: %v", err)
	}
	if err = box.Clear(); err != nil {
		return err
	}
	if err = box.SendKeys(text); err != nil {
		return err
	}

	return nil
}

func (s *seleniumHelper) activateTab(id string) error {
	if id == s.activeTabID {
		return nil
	}

	if err := s.clickOnByID(id); err != nil {
		return err
	}
	s.activeTabID = id

	return nil
}

func (s *seleniumHelper) startShortestPathCapture(g1 string, g2 string, bpf string) error {
	if err := s.activateTab("Captures"); err != nil {
		return err
	}

	if err := s.clickOnByID("create-capture"); err != nil {
		return err
	}

	if err := s.selectNode("node-selector-1", g1); err != nil {
		return err
	}

	if err := s.selectNode("node-selector-2", g2); err != nil {
		return err
	}

	if bpf != "" {
		if err := s.clickOnByID("capture-bpf"); err != nil {
			return err
		}
		if err := s.fillTextBoxByID("capture-bpf", bpf); err != nil {
			return err
		}
	}

	if err := s.clickOnByID("start-capture"); err != nil {
		return err
	}

	if err := s.closeNotification(); err != nil {
		return err
	}

	return nil
}

func (s *seleniumHelper) startGremlinCapture(gremlin string) error {
	if err := s.activateTab("Captures"); err != nil {
		return err
	}

	if err := s.clickOnByID("create-capture"); err != nil {
		return err
	}

	if err := s.clickOnByID("by-gremlin"); err != nil {
		return err
	}

	if err := s.fillTextBoxByID("capture-query", gremlin); err != nil {
		return err
	}

	if err := s.clickOnByID("start-capture"); err != nil {
		return err
	}

	if err := s.closeNotification(); err != nil {
		return err
	}

	//check capture created with the given query
	captures, err := s.webdriver.FindElements(selenium.ByClassName, "query")
	if err != nil {
		return err
	}
	var foundCapture bool
	for _, capture := range captures {
		if txt, _ := capture.Text(); txt == gremlin {
			foundCapture = true
			break
		}

	}
	if !foundCapture {
		return fmt.Errorf("Capture not found in the list")
	}
	return nil
}

func (s *seleniumHelper) closeNotification() error {
	notification, err := s.findElement(selenium.ByXPATH, ".//*[@id='notification']")
	if err != nil {
		return err
	}

	el, _ := notification.FindElement(selenium.ByClassName, "close")
	s.clickOn(el)

	return nil
}

func (s *seleniumHelper) injectPacket(g1 string, g2 string, count int) error {
	if err := s.activateTab("Generator"); err != nil {
		return err
	}

	if err := s.selectNode("inject-src", g1); err != nil {
		return err
	}

	if err := s.selectNode("inject-dst", g2); err != nil {
		return err
	}

	if count != 0 {
		if err := s.clickOnByID("inject-count"); err != nil {
			return err
		}

		if err := s.fillTextBoxByID("inject-count", fmt.Sprintf("%d", count)); err != nil {
			return err
		}
	}

	if err := s.clickOnByID("inject"); err != nil {
		return err
	}

	return nil
}

func (s *seleniumHelper) flowQuery(gremlin string) error {
	if err := s.activateTab("Flows"); err != nil {
		return err
	}

	flowQuery, err := s.findElement(selenium.ByXPATH, ".//*[@id='flow-table-query']")
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

	return nil
}

func (s *seleniumHelper) enableFakeMousePointer() error {
	script := `
		var css = document.createElement("link");
		css.type = 'text/css';
		css.rel = 'stylesheet';
		css.onload = function() {
			var js = document.createElement("script");
			js.type = "application/javascript";
			js.onload = function() {
				fakeMousePointer = new FakeMousePointer();
			};
			js.src = "/statics/js/fake-mouse.js";
			document.body.appendChild(js);
		}
		css.href = "/statics/css/fake-mouse.css";
		document.body.appendChild(css);
	`
	if _, err := s.webdriver.ExecuteScript(script, nil); err != nil {
		return err
	}
	s.fakeMousePointer = true

	return nil
}

func (s *seleniumHelper) fakeMousePointerMoveTo(x int, y int) error {
	script := fmt.Sprintf("fakeMousePointer.moveTo(%d, %d, arguments[0]);", x, y)
	if _, err := s.webdriver.ExecuteScriptAsync(script, nil); err != nil {
		return err
	}
	return nil
}

func (s *seleniumHelper) fakeMousePointerClickOn(el selenium.WebElement) error {
	if _, err := s.webdriver.ExecuteScriptAsync("fakeMousePointer.clickOn(arguments[0], arguments[1]);", []interface{}{el}); err != nil {
		return err
	}
	return nil
}

func (s *seleniumHelper) fakeMousePointerMoveOn(el selenium.WebElement) error {
	if _, err := s.webdriver.ExecuteScriptAsync("fakeMousePointer.moveOn(arguments[0], arguments[1]);", []interface{}{el}); err != nil {
		return err
	}
	return nil
}

func (s *seleniumHelper) fakeMousePointerClickTo(x int, y int) error {
	script := fmt.Sprintf("fakeMousePointer.clickTo(%d, %d, arguments[0]);", x, y)
	if _, err := s.webdriver.ExecuteScriptAsync(script, nil); err != nil {
		return err
	}
	return nil
}

func (s *seleniumHelper) clickOn(el selenium.WebElement) error {
	if s.fakeMousePointer {
		if err := s.fakeMousePointerClickOn(el); err != nil {
			return err
		}
	}

	if err := el.Click(); err != nil {
		return err
	}

	return nil
}

func (s *seleniumHelper) moveOn(el selenium.WebElement) error {
	if s.fakeMousePointer {
		if err := s.fakeMousePointerMoveOn(el); err != nil {
			return err
		}
	}

	if err := el.MoveTo(0, 0); err != nil {
		return err
	}

	return nil
}

func (s *seleniumHelper) clickOnByID(id string) error {
	el, err := s.findElement(selenium.ByXPATH, fmt.Sprintf(".//*[@id='%s']", id))
	if err != nil || el == nil {
		return fmt.Errorf("%s: %s", id, err)
	}

	return s.clickOn(el)
}

func (s *seleniumHelper) moveOnByID(id string) error {
	el, err := s.findElement(selenium.ByXPATH, fmt.Sprintf(".//*[@id='%s']", id))
	if err != nil || el == nil {
		return fmt.Errorf("%s: %s", id, err)
	}

	return s.moveOn(el)
}

func (s *seleniumHelper) startVideoRecord(name string) {
	cmds := []helper.Cmd{
		{"docker exec grid start-video", true},
	}
	helper.ExecCmds(s.t, cmds...)
	s.currVideoName = name
}

func (s *seleniumHelper) stopVideoRecord() {
	cmds := []helper.Cmd{
		{"docker exec grid stop-video", true},
		{"docker cp grid:/videos/. .", true},
		{fmt.Sprintf("mv cdd.mp4 %s.mp4", s.currVideoName), true},
	}
	helper.ExecCmds(s.t, cmds...)

	s.currVideoName = ""
}

func (s *seleniumHelper) quit() {
	s.webdriver.Quit()

	tearDownCmds := []helper.Cmd{
		{"docker stop grid", true},
		{"docker rm -f grid", true},
	}
	helper.ExecCmds(s.t, tearDownCmds...)
}

func newSeleniumHelper(t *testing.T, analyzerAddr string, analyzerPort int) (*seleniumHelper, error) {
	setupCmds := []helper.Cmd{
		{"docker pull elgalu/selenium", true},
		{"docker run -d --name=grid -p 4444:24444 -p 5900:25900 -e --shm-size=1g -p 6080:26080 -e SCREEN_WIDTH=1600 -e SCREEN_HEIGHT=1000 -e NOVNC=true -e VIDEO_FILE_NAME=cdd skydive/cdd-docker-selenium", true},
		{"docker exec grid wait_all_done 30s", true},
	}
	helper.ExecCmds(t, setupCmds...)

	caps := selenium.Capabilities{"browserName": "chrome"}
	webdriver, err := selenium.NewRemote(caps, "http://localhost:4444/wd/hub")
	if err != nil {
		return nil, err
	}
	webdriver.MaximizeWindow("")
	webdriver.SetAsyncScriptTimeout(5 * time.Second)

	os.Setenv("SKYDIVE_ANALYZERS", fmt.Sprintf("%s:%d", analyzerAddr, analyzerPort))

	authOptions := &shttp.AuthenticationOpts{}
	gh := gclient.NewGremlinQueryHelper(authOptions)

	sh := &seleniumHelper{
		addr:        analyzerAddr,
		port:        analyzerPort,
		webdriver:   webdriver,
		gh:          gh,
		activeTabID: "Captures",
		t:           t,
	}

	return sh, nil
}
