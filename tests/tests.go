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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/tebeka/selenium"

	"github.com/skydive-project/skydive/agent"
	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api"
	gclient "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/tests/helper"
)

const (
	Replay = iota
	OneShot
)

const testConfig = `---
ws_pong_timeout: 10

analyzers:
  - 127.0.0.1:8082

analyzer:
  listen: 0.0.0.0:8082
  storage:
    backend: {{.Storage}}
  analyzer_username: admin
  analyzer_password: password

agent:
  listen: 8081
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
      - docker
    netlink:
      metrics_update: 5

  metadata:
    info: This is compute node

flow:
  expire: 600
  update: 10

ovs:
  ovsdb: unix:///var/run/openvswitch/db.sock
  oflow:
    enable: true

storage:
  elasticsearch:
    host: 127.0.0.1:9200
  orientdb:
    addr: http://127.0.0.1:2480
    database: Skydive
    username: root
    password: {{.OrientDBRootPassword}}

graph:
  backend: {{.GraphBackend}}

logging:
  level: DEBUG

auth:
  type: noauth

etcd:
  data_dir: /tmp/skydive-etcd
  embedded: {{.EmbeddedEtcd}}
  servers:
    - {{.EtcdServer}}
`

type TestContext struct {
	gh        *gclient.GremlinQueryHelper
	client    *shttp.CrudClient
	captures  []*api.Capture
	setupTime time.Time
	data      map[string]interface{}
}

type TestCapture struct {
	gremlin    string
	kind       string
	bpf        string
	rawPackets int
}

type CheckFunction func(c *CheckContext) error

type CheckContext struct {
	*TestContext
	startTime   time.Time
	successTime time.Time
	time        time.Time
}

type Test struct {
	setupCmds        []helper.Cmd
	setupFunction    func(c *TestContext) error
	settleFunction   func(c *TestContext) error
	tearDownCmds     []helper.Cmd
	tearDownFunction func(c *TestContext) error
	captures         []TestCapture
	retries          int
	mode             int
	checks           []CheckFunction
	checkContexts    []*CheckContext
}

func (c *TestContext) getWholeGraph(t *testing.T, at time.Time) string {
	var g interface{}

	gremlin := "G"
	if !at.IsZero() {
		gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(at))
	}

	switch helper.GraphOutputFormat {
	case "ascii":
		header := make(http.Header)
		header.Set("Accept", "vnd.graphviz")
		resp, err := c.gh.Request(gremlin, header)
		if err != nil {
			t.Fatal(err.Error())
		}

		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		cmd := exec.Command("graph-easy", "--as_ascii")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err.Error())
		}

		if _, err = stdin.Write(b); err != nil {
			t.Fatal(err.Error())
		}
		stdin.Write([]byte("\n"))
		stdin.Close()

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err.Error())
		}

		return "\n" + string(output)

	default:
		if err := c.gh.Query(gremlin, &g); err != nil {
			t.Error(err.Error())
		}

		b, err := json.Marshal(&g)
		if err != nil {
			t.Fatal(err.Error())
		}

		return string(b)
	}
}

func (c *TestContext) getAllFlows(t *testing.T, at time.Time) string {
	gremlin := "G"
	if !at.IsZero() {
		gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(at))
	}
	gremlin += ".V().Flows()"

	flows, err := c.gh.GetFlows(gremlin)
	if err != nil {
		t.Error(err.Error())
		return ""
	}

	return helper.FlowsToString(flows)
}

func RunTest(t *testing.T, test *Test) {
	client, err := api.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
	if err != nil {
		t.Fatalf("Failed to create client: %s", err.Error())
	}

	var captures []*api.Capture
	defer func() {
		for _, capture := range captures {
			client.Delete("capture", capture.ID())
		}
	}()

	for _, tc := range test.captures {
		capture := api.NewCapture(tc.gremlin, tc.bpf)
		capture.Type = tc.kind
		capture.RawPacketLimit = tc.rawPackets
		if err = client.Create("capture", capture); err != nil {
			t.Fatal(err)
		}
		captures = append(captures, capture)
	}

	helper.ExecCmds(t, test.setupCmds...)

	context := &TestContext{
		gh:       gclient.NewGremlinQueryHelper(&shttp.AuthenticationOpts{}),
		client:   client,
		captures: captures,
		data:     make(map[string]interface{}),
	}

	err = common.Retry(func() error {
		for _, capture := range captures {
			nodes, err := context.gh.GetNodes(capture.GremlinQuery)
			if err != nil {
				return err
			}

			if len(nodes) == 0 {
				return fmt.Errorf("No node matching capture %s, graph: %s", capture.GremlinQuery, context.getWholeGraph(t, time.Now()))
			}

			for _, node := range nodes {
				tp, err := node.GetFieldString("Type")
				if err != nil || !common.IsCaptureAllowed(tp) {
					continue
				}

				captureID, err := node.GetFieldString("Capture.ID")
				if err != nil {
					return fmt.Errorf("Node %+v matched the capture but capture is not enabled, graph: %s", node, context.getWholeGraph(t, time.Now()))
				}

				if captureID != capture.ID() {
					return fmt.Errorf("Node %s matches multiple captures, graph: %s", node.ID, context.getWholeGraph(t, time.Now()))
				}
			}
		}

		return nil
	}, 15, time.Second)

	if err != nil {
		g := context.getWholeGraph(t, time.Now())
		helper.ExecCmds(t, test.tearDownCmds...)
		t.Fatalf("Failed to setup captures: %s, graph: %s", err.Error(), g)
	}

	retries := test.retries
	if retries <= 0 {
		retries = 30
	}

	settleTime := time.Now()

	if test.settleFunction != nil {
		err = common.Retry(func() error {
			return test.settleFunction(context)
		}, retries, time.Second)

		if err != nil {
			g := context.getWholeGraph(t, settleTime)
			f := context.getAllFlows(t, settleTime)
			helper.ExecCmds(t, test.tearDownCmds...)
			t.Errorf("Test failed to settle: %s, graph: %s, flows: %s", err.Error(), g, f)
			return
		}
	}

	context.setupTime = time.Now()

	if test.setupFunction != nil {
		if err = test.setupFunction(context); err != nil {
			g := context.getWholeGraph(t, context.setupTime)
			f := context.getAllFlows(t, context.setupTime)
			helper.ExecCmds(t, test.tearDownCmds...)
			t.Fatalf("Failed to setup test: %s, graph: %s, flows: %s", err.Error(), g, f)
		}
	}

	test.checkContexts = make([]*CheckContext, len(test.checks))

	for i, check := range test.checks {
		checkContext := &CheckContext{
			TestContext: context,
			startTime:   time.Now(),
		}
		test.checkContexts[i] = checkContext

		err = common.Retry(func() error {
			if err = check(checkContext); err != nil {
				return err
			}
			checkContext.successTime = time.Now()
			if checkContext.time.IsZero() {
				checkContext.time = checkContext.successTime
			}
			return nil
		}, retries, time.Second)

		if err != nil {
			g := checkContext.getWholeGraph(t, checkContext.startTime)
			f := checkContext.getAllFlows(t, checkContext.startTime)
			helper.ExecCmds(t, test.tearDownCmds...)
			t.Errorf("Test failed: %s, graph: %s, flows: %s", err.Error(), g, f)
			return
		}
	}

	if test.tearDownFunction != nil {
		if err = test.tearDownFunction(context); err != nil {
			helper.ExecCmds(t, test.tearDownCmds...)
			t.Fatalf("Fail to tear test down: %s", err.Error())
		}
	}

	helper.ExecCmds(t, test.tearDownCmds...)

	if test.mode == Replay {
		for i, check := range test.checks {
			checkContext := test.checkContexts[i]
			t.Logf("Replaying test with time %s (Unix: %d), startTime %s (Unix: %d)", checkContext.time, checkContext.time.Unix(), checkContext.startTime, checkContext.startTime.Unix())
			err = common.Retry(func() error {
				return check(checkContext)
			}, retries, time.Second)

			if err != nil {
				t.Errorf("Failed to replay test: %s, graph: %s, flows: %s", err.Error(), checkContext.getWholeGraph(t, checkContext.time), checkContext.getAllFlows(t, checkContext.time))
			}
		}
	}
}

func pingRequest(t *testing.T, context *TestContext, packet *api.PacketParamsReq) error {
	return context.client.Create("injectpacket", packet)
}

func ping(t *testing.T, context *TestContext, ipVersion int, src string, dst string, count int64, id int64) error {
	packet := &api.PacketParamsReq{
		Src:      src,
		Dst:      dst,
		Type:     fmt.Sprintf("icmp%d", ipVersion),
		Count:    count,
		ID:       id,
		Interval: 1000,
	}

	return pingRequest(t, context, packet)
}

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

func getFirstAvailableIPv4Addr() (string, error) {
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
			if ip = ip.To4(); ip != nil {
				return ip.String(), nil
			}
		}
	}
	return "", fmt.Errorf("No IP found")
}

// delay is just a wrapper to introduce a delay after a function call
// ex: delay(5*time.Second, sh.ClickOn)
func delay(delay time.Duration, err ...error) error {
	if len(err) > 0 && err[0] != nil {
		return err[0]
	}
	time.Sleep(delay)
	return nil
}

func delaySec(sec int, err ...error) error {
	return delay(time.Duration(sec)*time.Second, err...)
}

func init() {
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 100

	if helper.Standalone {
		if err := helper.InitConfig(testConfig); err != nil {
			panic(fmt.Sprintf("Failed to initialize config: %s", err.Error()))
		}

		if err := logging.InitLogging(); err != nil {
			panic(fmt.Sprintf("Failed to initialize logging system: %s", err.Error()))
		}

		server := analyzer.NewServerFromConfig()
		server.Start()

		agent := agent.NewAgent()
		agent.Start()

		// TODO: check for storage status instead of sleeping
		time.Sleep(3 * time.Second)
	}
}
