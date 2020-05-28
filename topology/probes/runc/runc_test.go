/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package runc

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/skydive-project/skydive/graffiti/logging"
	tp "github.com/skydive-project/skydive/topology/probes"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
)

const stateBasename = "state.json"

const stateContent = `{
  "id": "c96f6a897a632970bd4d0aa34f94452a4ce236b1dc239bf010bcf12f98d5bc56",
  "init_process_pid": 31046,
  "init_process_start": 541882281,
  "created": "2019-02-19T09:11:38.404438797Z",
  "config": {
    "mounts": [
      {
        "source": "/var/data/cripersistentstorage/io.containerd.grpc.v1.cri/sandboxes/87537844e40aaa9672ea2505600a94325221e901d04237356aff20c64849cbe0/resolv.conf",
        "destination": "/etc/resolv.conf"
      },
      {
        "source": "/var/data/kubelet/pods/5b9a9c62-3426-11e9-9f59-76f6e92e93c2/etc-hosts",
        "destination": "/etc/hosts"
      }
    ]
  }
}`

const hostsBasename = "etc-hosts"

const hostsContent = `# Kubernetes-managed hosts file.
127.0.0.1	localhost
::1	localhost ip6-localhost ip6-loopback
fe00::0	ip6-localnet # 1 space
fe00::0	 ip6-mcastprefix # 2 spaces
fe00::1	ip6-allnodes # 1 tab
fe00::2		ip6-allrouters # 1 space; 1 tab
172.30.247.227	myapp`

func TestLabels(t *testing.T) {
	var state containerState
	state.Config.Labels = []string{
		"io.kubernetes.container.ports={\"containerPort\":9090,\"protocol\":\"TCP\"}",
		"io.kubernetes.pod.namespace=kube-system",
	}

	handler := &ProbeHandler{
		ProbeHandler: &ns.ProbeHandler{
			Ctx: tp.Context{Logger: logging.GetLogger()},
		},
	}

	labels := handler.getLabels(state.Config.Labels)
	value, err := labels.GetField("io.kubernetes.container.ports.containerPort")
	if err != nil || value.(float64) != 9090 {
		t.Error("unable to find expected label value")
	}
}

func tempFile(filename, content string) (string, error) {
	tmpfile, err := ioutil.TempFile("", filename)
	if err != nil {
		return "", err
	}

	_, errWrite := tmpfile.Write([]byte(content))

	errClose := tmpfile.Close()

	if errWrite != nil {
		return "", errWrite
	}

	if errClose != nil {
		return "", errClose
	}

	return tmpfile.Name(), nil
}

func TestParseState(t *testing.T) {
	filename, err := tempFile(stateBasename, stateContent)
	if err != nil {
		t.Errorf("%s", err)
	}
	defer os.Remove(filename)

	_, err = parseState(filename)
	if err != nil {
		t.Errorf("unable to parse state file: %s", filename)
	}
}

func TestGetHostsFromState(t *testing.T) {
	filename, err := tempFile(stateBasename, stateContent)
	if err != nil {
		t.Errorf("%s", err)
	}
	defer os.Remove(filename)

	state, err := parseState(filename)
	if err != nil {
		t.Errorf("unable to parse state file: %s", filename)
	}

	_, err = getHostsFromState(state)
	if err != nil {
		t.Errorf("unable to find /etc/hosts entry in file: %s", filename)
	}
}

func TestReadHosts(t *testing.T) {
	filename, err := tempFile(hostsBasename, hostsContent)
	if err != nil {
		t.Errorf("%s", err)
	}
	defer os.Remove(filename)

	hosts, err := readHosts(filename)
	if err != nil {
		t.Errorf("unable to parse file: %s", filename)
	}

	const ip = "172.30.247.227"
	const hostname = "myapp"

	if hosts.IP != ip {
		t.Errorf("wrong IP: expected %s, actual %s", ip, hosts.IP)
	}

	if hosts.Hostname != hostname {
		t.Errorf("wrong Hostname: expected %s, actual %s", hostname, hosts.Hostname)
	}

	if _, ok := hosts.ByIP[ip]; !ok {
		t.Errorf("unable to find ip address entry: %s", ip)
	}
}
