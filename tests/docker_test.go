/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"testing"

	g "github.com/skydive-project/skydive/gremlin"
)

func TestDockerSimple(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"docker run -d -t -i --name test-skydive-docker-simple busybox", false},
		},

		tearDownCmds: []Cmd{
			{"docker rm -f test-skydive-docker-simple", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Type", "netns", "Manager", "docker")
			gremlin = gremlin.Out("Type", "container", "Docker.ContainerName", "/test-skydive-docker-simple")

			nodes, err := c.gh.GetNodes(gremlin)
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
		setupCmds: []Cmd{
			{"docker run -d -t -i --name test-skydive-docker-share-ns busybox", false},
			{"docker run -d -t -i --name test-skydive-docker-share-ns2 --net=container:test-skydive-docker-share-ns busybox", false},
		},

		tearDownCmds: []Cmd{
			{"docker rm -f test-skydive-docker-share-ns", false},
			{"docker rm -f test-skydive-docker-share-ns2", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Type", "netns", "Manager", "docker")
			gremlin = gremlin.Out().Has("Type", "container", "Docker.ContainerName", g.Within("/test-skydive-docker-share-ns", "/test-skydive-docker-share-ns2"))
			nodes, err := c.gh.GetNodes(gremlin)
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
		setupCmds: []Cmd{
			{"docker run -d -t -i --net=host --name test-skydive-docker-net-host busybox", false},
		},

		tearDownCmds: []Cmd{
			{"docker rm -f test-skydive-docker-net-host", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Docker.ContainerName", "/test-skydive-docker-net-host", "Type", "container")
			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 container, got %+v", nodes)
			}

			gremlin = c.gremlin.V().Has("Type", "netns", "Manager", "docker", "Name", "test-skydive-docker-net-host")
			if nodes, err = c.gh.GetNodes(gremlin); err != nil {
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
		setupCmds: []Cmd{
			{"docker run -d -t -i --label a.b.c=123 --label a~b/c@d=456 --name test-skydive-docker-labels busybox", false},
		},

		tearDownCmds: []Cmd{
			{"docker rm -f test-skydive-docker-labels", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Docker.ContainerName", "/test-skydive-docker-labels", "Type", "container", "Docker.Labels.a.b.c", "123", "Docker.Labels.a~b/c@d", "456")
			_, err := c.gh.GetNode(gremlin)
			return err
		}},
	}

	RunTest(t, test)
}
