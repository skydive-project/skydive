//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package topology

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// ContainerMetadata describe the metadata of a docker container
// easyjson:json
// gendecoder
type ContainerMetadata struct {
	ID             string
	Image          string `json:",omitempty"`
	ImageID        string `json:",omitempty"`
	Runtime        string
	Status         string
	InitProcessPID int64
	Hosts          Hosts          `json:",omitempty"`
	Labels         graph.Metadata `json:",omitempty" field:"Metadata"`
}

// Hosts describes a /etc/hosts file
// easyjson:json
// gendecoder
type Hosts struct {
	IP       string         `json:",omitempty"`
	Hostname string         `json:",omitempty"`
	ByIP     graph.Metadata `json:",omitempty" field:"Metadata"`
}

func newHosts() *Hosts {
	return &Hosts{ByIP: graph.Metadata{}}
}

// ReadHosts parses a 'hosts' file such as /etc/hosts
func ReadHosts(path string) (*Hosts, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hosts := newHosts()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if err := scanner.Err(); err != nil {
			return nil, err
		}

		if i := strings.IndexByte(line, '#'); i >= 0 {
			// Discard comment.
			line = line[0:i]
		}

		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}

		ip := net.ParseIP(f[0])
		if ip == nil {
			continue
		}

		if ip.IsLoopback() {
			continue
		}

		hosts.IP = ip.String()
		ips := make([]string, len(f))
		for i := 1; i < len(f); i++ {
			hosts.Hostname = strings.ToLower(f[i])
			ips[i-1] = hosts.Hostname
		}
		hosts.ByIP[hosts.IP] = ips
	}

	return hosts, nil
}

// ContainerMetadataDecoder implements a json message raw decoder
func ContainerMetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var m ContainerMetadata
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal container metadata %s: %s", string(raw), err)
	}

	return &m, nil
}

// RegisterContainer registers container metadata decoder
func RegisterContainer() {
	graph.NodeMetadataDecoders["Container"] = ContainerMetadataDecoder
}
