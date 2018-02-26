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
	"fmt"
	"testing"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestDockerSimple(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"docker run -d -t -i --name test-skydive-docker-simple busybox", false},
		},

		tearDownCmds: []helper.Cmd{
			{"docker rm -f test-skydive-docker-simple", false},
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin += `.V().Has("Type", "netns", "Manager", "docker")`
			gremlin += `.Out("Type", "container", "Docker.ContainerName", "/test-skydive-docker-simple")`

			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestDockerShareNamespace(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"docker run -d -t -i --name test-skydive-docker-share-ns busybox", false},
			{"docker run -d -t -i --name test-skydive-docker-share-ns2 --net=container:test-skydive-docker-share-ns busybox", false},
		},

		tearDownCmds: []helper.Cmd{
			{"docker rm -f test-skydive-docker-share-ns", false},
			{"docker rm -f test-skydive-docker-share-ns2", false},
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh

			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin += `.V().Has("Type", "netns", "Manager", "docker")`
			gremlin += `.Out().Has("Type", "container", "Docker.ContainerName", Within("/test-skydive-docker-share-ns", "/test-skydive-docker-share-ns2"))`
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 2 {
				return fmt.Errorf("Expected 2 nodes, got %+v", nodes)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestDockerNetHost(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"docker run -d -t -i --net=host --name test-skydive-docker-net-host busybox", false},
		},

		tearDownCmds: []helper.Cmd{
			{"docker rm -f test-skydive-docker-net-host", false},
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh

			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin := prefix + `.V().Has("Docker.ContainerName", "/test-skydive-docker-net-host", "Type", "container")`
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 container, got %+v", nodes)
			}

			gremlin = prefix + `.V().Has("Type", "netns", "Manager", "docker", "Name", "test-skydive-docker-net-host")`
			nodes, err = gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 0 {
				return fmt.Errorf("There should be only no namespace managed by Docker, got %+v", nodes)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestDockerLabels(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"docker run -d -t -i --label a.b.c=123 --label a~b/c@d=456 --name test-skydive-docker-labels busybox", false},
		},

		tearDownCmds: []helper.Cmd{
			{"docker rm -f test-skydive-docker-labels", false},
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh

			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin := prefix + `.V().Has("Docker.ContainerName", "/test-skydive-docker-labels",`
			gremlin += ` "Type", "container", "Docker.Labels.a.b.c", "123", "Docker.Labels.a~b/c@d", "456")`
			fmt.Printf("Gremlin: %s\n", gremlin)
			_, err := gh.GetNode(gremlin)
			if err != nil {
				return err
			}

			return nil
		}},
	}

	RunTest(t, test)
}
