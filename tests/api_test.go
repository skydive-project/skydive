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

package tests

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
)

type testAPIServer struct {
	authType     string
	dataDir      string
	passwordFile string
	analyzer     *analyzer.Server
}

const confApi = `---
auth:
  type: {{.AuthType}}
  basic:
    file: {{.PasswordFile}}

analyzer:
  listen: :{{.AnalyzerPort}}
  flowtable_expire: 600
  flowtable_update: 10
  flowtable_agent_ratio: 0.5

etcd:
  embedded: {{.EmbeddedEtcd}}
  port: 2374
  data_dir: {{.EtcdDataDir}}
  servers: {{.EtcdServer}}

logging:
  default: {{.LogLevel}}
`

func createAPIServer(t *testing.T, auth string) (*testAPIServer, error) {
	tmpDir, err := ioutil.TempDir("/tmp", "skydive-api-tests")
	if err != nil {
		return nil, err
	}

	var passwordFile string
	switch auth {
	case "basic":
		f, err := ioutil.TempFile("/tmp", "api-test-basicauth")
		if err != nil {
			return nil, err
		}
		passwordFile = f.Name()

		if _, err := f.WriteString("admin:$apr1$/tk0tCNm$fBaXEudF9OTyFUhuqoIwp/"); err != nil {
			return nil, err
		}

		if err := f.Close(); err != nil {
			return nil, err
		}
	}

	ts := NewTestStorage()

	params := make(helper.HelperParams)
	params["AuthType"] = auth
	params["PasswordFile"] = passwordFile
	params["EtcdDataDir"] = tmpDir

	analyzer := helper.StartAnalyzerWithConfig(t, confApi, ts, params)

	return &testAPIServer{
		authType:     auth,
		analyzer:     analyzer,
		dataDir:      tmpDir,
		passwordFile: passwordFile,
	}, nil
}

type testAPIClient struct {
	*shttp.CrudClient
}

func (c *testAPIClient) create(kind string, resource interface{}) error {
	tries := 1
	for {
		if err := c.Create(kind, resource); err == nil {
			return nil
		} else {
			if tries == 5 {
				return err
			}
		}
		time.Sleep(time.Second)
		tries += 1
	}
}

func (s *testAPIServer) GetClient() (*testAPIClient, error) {
	authenticationOpts := shttp.AuthenticationOpts{Username: "admin", Password: "password"}
	client := shttp.NewCrudClient(s.analyzer.HTTPServer.Addr, s.analyzer.HTTPServer.Port, &authenticationOpts, "api")
	if client == nil {
		return nil, errors.New("Failed to create client")
	}
	return &testAPIClient{client}, nil
}

func (s *testAPIServer) Stop() {
	s.analyzer.Stop()
	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
	if s.passwordFile != "" {
		os.Remove(s.passwordFile)
	}
	if s.dataDir != "" {
		os.RemoveAll(s.dataDir)
	}
}

func TestAlertApi(t *testing.T) {
	apiServer, err := createAPIServer(t, "noauth")
	if err != nil {
		t.Fatal(err)
	}

	defer apiServer.Stop()
	apiClient, err := apiServer.GetClient()
	if err != nil {
		t.Fatal(err)
	}

	alert := api.NewAlert()
	alert.Expression = "G.V().Has('MTU', GT(1500))"
	if err := apiClient.create("alert", alert); err != nil {
		t.Fatalf("Failed to create alert: %s", err.Error())
	}

	alert2 := api.NewAlert()
	alert2.Expression = "G.V().Has('MTU', Gt(1500))"
	if err := apiClient.Get("alert", alert.UUID, &alert2); err != nil {
		t.Error(err)
	}

	if *alert != *alert2 {
		t.Errorf("Alert corrupted: %+v != %+v", alert, alert2)
	}

	var alerts map[string]api.Alert
	if err := apiClient.List("alert", &alerts); err != nil {
		t.Error(err)
	} else {
		if len(alerts) != 1 {
			t.Errorf("Wrong number of alerts: got %d, expected 1", len(alerts))
		}
	}

	if alerts[alert.UUID] != *alert {
		t.Errorf("Alert corrupted: %+v != %+v", alerts[alert.UUID], alert)
	}

	if err := apiClient.Delete("alert", alert.UUID); err != nil {
		t.Errorf("Failed to delete alert: %s", err.Error())
	}

	var alerts2 map[string]api.Alert
	if err := apiClient.List("alert", &alerts2); err != nil {
		t.Errorf("Failed to list alerts: %s", err.Error())
	} else {
		if len(alerts2) != 0 {
			t.Errorf("Wrong number of alerts: got %d, expected 0 (%+v)", len(alerts2), alerts2)
		}
	}
}

func TestCaptureApi(t *testing.T) {
	apiServer, err := createAPIServer(t, "basic")
	if err != nil {
		t.Fatal(err)
	}

	defer apiServer.Stop()
	apiClient, err := apiServer.GetClient()
	if err != nil {
		t.Fatal(err)
	}

	capture := api.NewCapture("G.V().Has('Name', 'br-int')", "port 80")
	if err := apiClient.create("capture", capture); err != nil {
		t.Fatalf("Failed to create alert: %s", err.Error())
	}

	capture2 := &api.Capture{}
	if err := apiClient.Get("capture", capture.ID(), &capture2); err != nil {
		t.Error(err)
	}

	if *capture != *capture2 {
		t.Errorf("Capture corrupted: %+v != %+v", capture, capture2)
	}

	var captures map[string]api.Capture
	if err := apiClient.List("capture", &captures); err != nil {
		t.Error(err)
	} else {
		if len(captures) != 1 {
			t.Errorf("Wrong number of captures: got %d, expected 1", len(captures))
		}
	}

	if captures[capture.ID()] != *capture {
		t.Errorf("Capture corrupted: %+v != %+v", captures[capture.ID()], capture)
	}

	if err := apiClient.Delete("capture", capture.ID()); err != nil {
		t.Errorf("Failed to delete capture: %s", err.Error())
	}

	var captures2 map[string]api.Capture
	if err := apiClient.List("capture", &captures2); err != nil {
		t.Errorf("Failed to list captures: %s", err.Error())
	} else {
		if len(captures2) != 0 {
			t.Errorf("Wrong number of captures: got %d, expected 0 (%+v)", len(captures2), captures2)
		}
	}

	if err := apiClient.Get("capture", capture.ID(), &capture2); err == nil {
		t.Errorf("Found delete capture: %s", capture.ID())
	}
}
