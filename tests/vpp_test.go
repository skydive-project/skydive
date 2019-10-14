// +build vpp

/*
 * Copyright (C) 2019 Red Hat, Inc.
 * Copyright (c) 2019 PANTHEON.tech s.r.o.
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
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/topology/probes/vpp"
)

const (
	dockerImageWithRunningVPP = "ligato/vpp-base:19.04"
	vppWaitScript             = "sh -c 'retry=%d;until docker exec %s vppctl sh version || [ $retry -eq 0 ]; do retry=$(( retry-1 ));sleep 0.5s;echo \"VPP not ready-retries left \"$retry;done'"
)

func createLoopback(t *testing.T) string {
	cmd := "vppctl loopback create-interface"
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		t.Error("Can't create vpp loopback interface ", err)
	}
	time.Sleep(vpp.VPPPollingTime)
	return strings.Trim(string(out), "\r\n")
}

func deleteLoopback(t *testing.T, intf string) string {
	cmd := "vppctl loopback delete-interface intfc " + intf
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		t.Error("Can't delete vpp loopback interface ", intf, " : ", err)
	}
	time.Sleep(vpp.VPPPollingTime)
	return strings.Trim(string(out), "\r\n")
}

func TestVPPLoopback(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"vppctl show interface", true},
		},

		tearDownCmds: []Cmd{
			{"vppctl show interface", true},
		},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			var nodes []*graph.Node
			var err error
			loop1 := createLoopback(t)
			gremlin := c.gremlin.V().Has("Driver", "vpp", "Name", loop1)
			if nodes, err = c.gh.GetNodes(gremlin); err != nil {
				return err
			}
			if len(nodes) < 1 {
				return fmt.Errorf("Expected one interface, got %+v", nodes)
			}

			loop2 := createLoopback(t)
			gremlin = c.gremlin.V().Has("Driver", "vpp", "Name", loop2)
			if nodes, err = c.gh.GetNodes(gremlin); err != nil {
				return err
			}
			if len(nodes) < 1 {
				return fmt.Errorf("Expected one interface, got %+v", nodes)
			}

			deleteLoopback(t, loop1)
			gremlin = c.gremlin.V().Has("Driver", "vpp", "Name", loop1)
			if nodes, err = c.gh.GetNodes(gremlin); err != nil {
				return err
			}
			if len(nodes) != 0 {
				return fmt.Errorf("Expected no interface, got %+v", nodes)
			}

			deleteLoopback(t, loop2)

			return nil
		}},
	}
	RunTest(t, test)
}

func TestVPPInDocker(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{fmt.Sprintf("docker run -d -t -i --privileged --name test-skydive-docker-running-vpp %s", dockerImageWithRunningVPP), false},
			{fmt.Sprintf(vppWaitScript, 30, "test-skydive-docker-running-vpp"), true},
		},

		tearDownCmds: []Cmd{
			{"docker rm -f test-skydive-docker-running-vpp", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			return assertOneEndNode(c, c.gremlin.V().Has("Type", "netns").
				Out("Type", "vpp"))
		}},
	}

	RunTest(t, test)
}

func TestVPPConnectingToVeth(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{fmt.Sprintf("docker run -d -t -i --privileged --name test-skydive-docker-vpp-to-veth %s", dockerImageWithRunningVPP), false},
			{fmt.Sprintf(vppWaitScript, 30, "test-skydive-docker-vpp-to-veth"), true},
			{"docker exec test-skydive-docker-vpp-to-veth ip link add name veth-container type veth peer name veth-host", true}, // creating veth tunnel (that can be used to tunnel docker container and docker host)
			{"docker exec test-skydive-docker-vpp-to-veth ip link set dev veth-container up", true},
			{"docker exec test-skydive-docker-vpp-to-veth ip link set dev veth-host up", true},                     // no need for this test to actually push veth-host to network namespace of docker host OS
			{"docker exec test-skydive-docker-vpp-to-veth vppctl create host-interface name veth-container", true}, // grabbing and using veth-container end of tunnel in VPP
			{"docker exec test-skydive-docker-vpp-to-veth vppctl set int state host-veth-container up", true},
		},

		tearDownCmds: []Cmd{
			{"docker rm -f test-skydive-docker-vpp-to-veth", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			return assertOneEndNode(c, c.gremlin.V().Has("Type", "interface", "Driver", "vpp", "Name", "host-veth-container").
				In("Type", "veth", "Name", "veth-container"))
		}},
	}

	RunTest(t, test)
}

func TestVPPMemifTunnel(t *testing.T) {
	vpp1Container := "test-skydive-docker-vpp1-with-memif-tunnel"
	vpp2Container := "test-skydive-docker-vpp2-with-memif-tunnel"
	test := &Test{
		setupCmds: []Cmd{
			// prepare container-shared folder (docker would create it automatically, but creating it now and with user that is running test resolves permission problems in teardown)
			{"mkdir /tmp/skydivetests-dockervpp-sockets", false},

			// starting docker contrainers
			{fmt.Sprintf("docker run -d -t -i -v /tmp/skydivetests-dockervpp-sockets/:/run/othersockets/ --privileged --name %s %s", vpp1Container, dockerImageWithRunningVPP), false},
			{fmt.Sprintf("docker run -d -t -i -v /tmp/skydivetests-dockervpp-sockets/:/run/othersockets/ --privileged --name %s %s", vpp2Container, dockerImageWithRunningVPP), false},

			// waiting for VPPs to start inside containers
			{fmt.Sprintf(vppWaitScript, 30, vpp1Container), true},
			{fmt.Sprintf(vppWaitScript, 30, vpp2Container), true},

			// creating memif tunnel
			{fmt.Sprintf("docker exec %s vppctl create memif socket id 1 filename /run/othersockets/another-memif.sock", vpp1Container), true},
			{fmt.Sprintf("docker exec %s vppctl create interface memif socket-id 1 id 0 master", vpp1Container), true},
			{fmt.Sprintf("docker exec %s vppctl set int state memif1/0 up", vpp1Container), true},
			{fmt.Sprintf("docker exec %s vppctl create memif socket id 1 filename /run/othersockets/another-memif.sock", vpp2Container), true},
			{fmt.Sprintf("docker exec %s vppctl create interface memif socket-id 1 id 0 slave", vpp2Container), true},
			{fmt.Sprintf("docker exec %s vppctl set int state memif1/0 up", vpp2Container), true},
		},

		tearDownCmds: []Cmd{
			// removing memif socket file (it was created by VPP,but removing it from VPP doesn't remove the physical
			// file->removing reference from VPPs and removing it on docker container level to prevent permission problems)
			{fmt.Sprintf("docker exec %s vppctl delete interface memif memif1/0", vpp1Container), true},
			{fmt.Sprintf("docker exec %s vppctl delete interface memif memif1/0", vpp2Container), true},
			{fmt.Sprintf("docker exec %s vppctl delete memif socket id 1", vpp1Container), true},
			{fmt.Sprintf("docker exec %s vppctl delete memif socket id 1", vpp2Container), true},
			{fmt.Sprintf("docker exec %s rm -rf /run/othersockets/another-memif.sock", vpp1Container), true},

			// removing docker containers
			{fmt.Sprintf("docker rm -f %s", vpp1Container), false},
			{fmt.Sprintf("docker rm -f %s", vpp2Container), false},

			// removing container-shared folder for memif socket file
			{"rm -rf /tmp/skydivetests-dockervpp-sockets", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			return assertOneEndNode(c, c.gremlin.V().Has("Type", "vpp", "Program", "vpe").
				Out("Type", "interface", "Name", "memif1/0", "VPP.SocketFilename", "/run/othersockets/another-memif.sock", "VPP.Master", true).
				Out("Type", "interface", "Name", "memif1/0", "VPP.SocketFilename", "/run/othersockets/another-memif.sock", "VPP.Master", false).
				In("Type", "vpp", "Program", "vpe"))
		}},
	}

	RunTest(t, test)
}

func assertOneEndNode(c *CheckContext, queryString gremlin.QueryString) error {
	nodes, err := c.gh.GetNodes(queryString)
	if err != nil {
		return err
	}

	if len(nodes) != 1 {
		return fmt.Errorf("expected 1 end node, got %+v", nodes)
	}

	return nil
}
