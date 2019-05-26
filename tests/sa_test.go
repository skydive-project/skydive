// +build sa_tests

/*
 * Copyright (C) 2019 IBM, Inc.
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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/contrib/objectstore/subscriber"
	g "github.com/skydive-project/skydive/gremlin"
)

const securityAdvisorConfigTemplate = `---
objectstore:
  endpoint: http://127.0.0.1:9000
  region: local
  bucket: bucket
  access_key: user
  secret_key: password
  subscriber_url: ws://%s/ws/subscriber/flow
`

func setupSecurityAdvisor(c *TestContext) error {
	cfg := viper.New()

	analyzers := config.GetStringSlice("analyzers")
	if len(analyzers) == 0 {
		return errors.New("Missing analyzers config")
	}

	f, err := ioutil.TempFile("", "skydive_agent")
	if err != nil {
		return fmt.Errorf("failed to create configuration file: %s", err)
	}
	if _, err = f.Write([]byte(fmt.Sprintf(securityAdvisorConfigTemplate, analyzers[0]))); err != nil {
		return fmt.Errorf("failed to write configuration file: %s", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("failed to close configuration file: %s", err)
	}

	configFile, err := os.Open(f.Name())
	if err != nil {
		return fmt.Errorf("failed to open configuration file: %s", err)
	}
	cfg.SetConfigType("yaml")
	if err := cfg.MergeConfig(configFile); err != nil {
		return fmt.Errorf("failed to update configuration: %s", err)
	}

	s, err := subscriber.NewSubscriberFromConfig(cfg)
	if err != nil {
		return err
	}

	s.Start()
	c.data["subscriber"] = s
	return err
}

func tearDownsecurityAdvisor(c *TestContext) error {
	rawS, ok := c.data["subscriber"]
	if ok {
		s := rawS.(*subscriber.Subscriber)
		s.Stop()

		storage := s.GetStorage()
		objectKeys, err := storage.ListObjects()
		if err != nil {
			return err
		}

		for _, objectKey := range objectKeys {
			if err = storage.DeleteObject(objectKey); err != nil {
				return err
			}
		}
	}
	return nil
}

const (
	network     = "mynet"
	hostname    = "myhost"
	name        = "myhost"
	subnet      = "172.18.0.0/16"
	ip          = "172.18.0.22"
	networkName = "0_0_myhost_0"
)

func checkSecurityAdvisor(c *CheckContext) error {
	storage := c.data["subscriber"].(*subscriber.Subscriber).GetStorage()

	objectKeys, err := storage.ListObjects()
	if err != nil {
		return fmt.Errorf("Failed to list objects: %s", err)
	}

	flows := make([]*subscriber.SecurityAdvisorFlow, 0)
	for _, objectKey := range objectKeys {
		var objectFlows []*subscriber.SecurityAdvisorFlow
		if err := storage.ReadObjectFlows(objectKey, &objectFlows); err != nil {
			return fmt.Errorf("Failed to read object flows: %s", err)
		}

		flows = append(flows, objectFlows...)
	}

	found := false
	for _, fl := range flows {
		if fl.Network != nil && fl.Network.A == ip && fl.Network.B == ip {
			// FIXME: currently docker networkName resolution is not supported
			// if fl.Network.BName != networkName {
			//	return fmt.Errorf("Expected '"+networkName+"' B_Name, but got: %s", fl.Network.BName)
			// }

			found = true
		}
	}

	if !found {
		return errors.New("No flows found with src/dest ip " + ip)
	}

	return nil
}

func TestSecurityAdvisor(t *testing.T) {
	veth := g.G.V().Has("Name", name, "Type", "container").Both("Type", "netns").Both("Type", "veth")

	test := &Test{
		setupFunction: setupSecurityAdvisor,

		preCleanup: true,

		setupCmds: []Cmd{
			{"docker network create --subnet=" + subnet + " " + network, true},
			{"docker run -d --name " + name + " --hostname " + hostname + " --net " + network + " --ip " + ip + " nginx", true},
		},

		injections: []TestInjection{{
			from:  veth,
			to:    veth,
			count: 1,
		}},

		tearDownCmds: []Cmd{
			{"docker rm -f " + name, false},
			{"docker network rm " + network, false},
		},

		captures: []TestCapture{
			{gremlin: veth},
		},

		mode: OneShot,

		checks: []CheckFunction{checkSecurityAdvisor},

		tearDownFunction: tearDownsecurityAdvisor,
	}

	RunTest(t, test)
}
