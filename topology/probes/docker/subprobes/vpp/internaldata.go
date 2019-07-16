// +build docker_vpp,linux

// Copyright (c) 2019 PANTHEON.tech s.r.o.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vpp

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/docker"
)

// vppData is container for date extracted from VPP using CLI (or binary API)
type vppData struct {
	version           string              // CLI: show version
	linkedHardware    map[string]struct{} // CLI: show hardware (provided HARDWARE column)
	memifToSocketFile map[string]string   // map[memif name]=host OS full path to memif socket, info taken from docker container info + CLI: show hardware + CLI: show memif
}

// change types for containerListChange struct
const (
	addContainer = iota
	removeContainer
)

// containerListChange is message struct informing about change in running containers list. This message is created from
// information provided by hooked docker probe events.
type containerListChange struct {
	containerInfo
	changeType    int
	containerName string
}

// containerInfo represents all info retrieved in time for given docker container
type containerInfo struct {
	containerDockerInfo
	containerGraphInfo
}

// containerDockerInfo is docker-related info retrieved in time for given docker container
type containerDockerInfo struct {
	volumes map[string]string // host path to container path
}

// containerGraphInfo is UI graph-related info retrieved in time for given docker container
type containerGraphInfo struct {
	containerNodeID            graph.Identifier
	nsRootNodeID               graph.Identifier
	vppNodeID                  graph.Identifier
	linkedHardwareNodesToEdges map[graph.Identifier]graph.Identifier                      // last detected (and projected to graph) edges from VPP to hardware interfaces, map[hardware interface node id] = edge id
	memifNametoNodeID          map[string]graph.Identifier                                // last detected (and projected to graph) memifs, map[memif name from vpp] = memif node id in graph
	globalMemifEdges           map[graph.Identifier]map[graph.Identifier]graph.Identifier // last detected (and projected to graph) memif edges for whole graph and not only for container graph, map[first memif node id]map[second memif node id] = id of edge connecting that 2 memifs
}

// volumes extracts volume information from types.ContainerJSON <info>
func (p *Subprobe) volumes(info *types.ContainerJSON) map[string]string {
	volumes := make(map[string]string)
	for _, mountCfg := range info.HostConfig.Binds {
		mountPair := strings.Split(mountCfg, ":")
		volumes[mountPair[0]] = mountPair[1]
	}
	return volumes
}

// containerName extracts container name from metadata of container root graph node
func (p *Subprobe) containerName(node *graph.Node) (string, error) {
	dockerInfo, ok := node.Metadata["Docker"].(docker.Metadata)
	if !ok {
		return "", fmt.Errorf("no Docker data found")
	}
	return dockerInfo.ContainerName, nil
}
