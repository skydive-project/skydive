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
  max_flows_per_object: 6000
  max_seconds_per_object: 0
  max_seconds_per_stream: 86400
  max_flow_array_size: 100000
  flow_transformer: security-advisor
`

func initSecurityAdvisorConfig(cfg *viper.Viper) error {
	analyzers := config.GetStringSlice("analyzers")
	if len(analyzers) == 0 {
		return errors.New("Missing analyzers config")
	}

	f, err := ioutil.TempFile("", "skydive_agent")
	if err != nil {
		return fmt.Errorf("failed to create configuration file: %s", err.Error())
	}
	if _, err = f.Write([]byte(fmt.Sprintf(securityAdvisorConfigTemplate, analyzers[0]))); err != nil {
		return fmt.Errorf("failed to write configuration file: %s", err.Error())
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("failed to close configuration file: %s", err.Error())
	}

	configFile, err := os.Open(f.Name())
	if err != nil {
		return fmt.Errorf("failed to open configuration file: %s", err.Error())
	}
	cfg.SetConfigType("yaml")
	if err := cfg.MergeConfig(configFile); err != nil {
		return fmt.Errorf("failed to update configuration: %s", err.Error())
	}
	return nil
}

func securityAdvisorSetup(c *TestContext) error {
	cfg := viper.New()
	if err := initSecurityAdvisorConfig(cfg); err != nil {
		return fmt.Errorf("Failed to init security advisor configuration: %s", err.Error())
	}

	s, err := subscriber.NewSubscriberFromConfig(cfg)
	if err != nil {
		return err
	}

	s.Start()
	c.data["subscriber"] = s
	return err
}

func securityAdvisorTearDown(c *TestContext) error {
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

func Test_SecurityAdvisor(t *testing.T) {
	test := &Test{
		setupFunction: securityAdvisorSetup,

		setupCmds: []Cmd{
			{"podman run -d --name secadvtest --ip=10.88.0.49 --hostname=myhost nginx", false},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "cni0"),
			toIP:  "10.88.0.49",
			count: 1,
		}},

		tearDownCmds: []Cmd{
			{"podman kill secadvtest", false},
			{"podman rm secadvtest", false},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "cni0")},
		},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			storage := c.data["subscriber"].(*subscriber.Subscriber).GetStorage()

			objectKeys, err := storage.ListObjects()
			if err != nil {
				return fmt.Errorf("Failed to list objects: %s", err.Error())
			}

			flows := make([]*subscriber.SecurityAdvisorFlow, 0)
			for _, objectKey := range objectKeys {
				var objectFlows []*subscriber.SecurityAdvisorFlow
				if err := storage.ReadObjectFlows(objectKey, &objectFlows); err != nil {
					return fmt.Errorf("Failed to read object flows: %s", err.Error())
				}

				flows = append(flows, objectFlows...)
			}

			found := false
			for _, fl := range flows {
				if fl.Network != nil && fl.Network.B == "10.88.0.49" {
					if fl.NodeType != "bridge" {
						return fmt.Errorf("Expected 'bridge' NodeType, but got: %s", fl.NodeType)
					}

					if fl.Network.BName != "0_0_myhost_0" {
						return fmt.Errorf("Expected '0_0_myhost_0' B_Name, but got: %s", fl.Network.BName)
					}

					found = true
				}
			}

			if !found {
				return errors.New("No flows found with destination 10.88.0.49")
			}

			return nil
		}},

		tearDownFunction: securityAdvisorTearDown,
	}

	RunTest(t, test)
}
