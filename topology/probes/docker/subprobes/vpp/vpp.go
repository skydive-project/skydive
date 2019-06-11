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

// Package vpp contains docker subprobe that extends docker topology graph with information about running VPP
// (https://wiki.fd.io/view/VPP) and related interfaces (memif - https://docs.fd.io/vpp/17.10/libmemif_doc.html, veth)
package vpp

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes/docker/subprobes"
	"github.com/skydive-project/skydive/topology/probes/netns"
)

// Subprobe describes a VPP topology graph enhancements
type Subprobe struct {
	containerListChan chan containerListChange
	graph             *graph.Graph
	wg                sync.WaitGroup
	cancel            context.CancelFunc
}

// NewSubprobe creates a new topology Docker subprobe
func NewSubprobe(dockerNSProbe *netns.Probe) (*Subprobe, error) {
	logging.GetLogger().Debug("VPP probe creating...")
	dockerProbe := &Subprobe{
		containerListChan: make(chan containerListChange, 100),
		graph:             dockerNSProbe.Graph,
	}

	return dockerProbe, nil
}

// Start the probe (this is only relevant only for checking VPP that is installed native (no docker, but directly in host linux))
func (p *Subprobe) Start() {
	logging.GetLogger().Debug("VPP subprobe starting...")
	var ctx context.Context
	ctx, p.cancel = context.WithCancel(context.Background())
	p.wg.Add(1)
	go p.handleTopologyUpdates(ctx)
}

// Stop the probe (this is only relevant only for checking VPP that is installed native (no docker, but directly in host linux)
func (p *Subprobe) Stop() {
	logging.GetLogger().Debug("VPP subprobe stopping...")
	p.cancel()
	p.wg.Wait()
}

// RegisterContainer is called by docker probe to notify subprobe about container addition detection (registration)
func (p *Subprobe) RegisterContainer(data *subprobes.ContainerRegistrationData) error {
	logging.GetLogger().Debugf("VPP subprobe catched registration of docker container: %+v ", data)
	volumes := p.volumes(&data.Info)
	containerName, err := p.containerName(data.Node)
	if err != nil {
		return fmt.Errorf("can't extract container name for metadata of graph node %v due to: %v", data.Node, err)
	}

	p.containerListChan <- containerListChange{
		changeType:    addContainer,
		containerName: containerName,
		containerInfo: containerInfo{
			containerDockerInfo: containerDockerInfo{
				volumes: volumes,
			},
			containerGraphInfo: containerGraphInfo{
				containerNodeID: data.Node.ID,
				nsRootNodeID:    data.NSRootID,
			},
		},
	}
	return nil
}

// UnregisterContainer is called by docker probe to notify subprobe about container removal detection (unregistration)
func (p *Subprobe) UnregisterContainer(data *subprobes.ContainerUnregistrationData) error {
	logging.GetLogger().Debugf("VPP subprobe catched unregistration of docker container: %+v ", data)
	containerName, err := p.containerName(data.Node)
	if err != nil {
		return fmt.Errorf("can't extract container name for metadata of graph node %v", data.Node)
	}

	p.containerListChan <- containerListChange{
		changeType:    removeContainer,
		containerName: containerName,
	}
	return nil
}

// handleTopologyUpdates handles topology changes related to VPP and propagates them to UI graph
func (p *Subprobe) handleTopologyUpdates(ctx context.Context) {
	defer p.wg.Done()
	containerNames := make(map[string]*containerInfo)
	detectedVPPs := make(map[string]*vppData) // detected VPPs identified by container name (simplification for now: container can have max 1 VPP)
	ticker := time.NewTicker(p.probeInterval())
	for {
		select {
		case msg := <-p.containerListChan: //topology changes catched from docker probe
			switch msg.changeType {
			case addContainer:
				containerNames[msg.containerName] = &msg.containerInfo
				p.updateState(containerNames, detectedVPPs, nil)
			case removeContainer:
				cgi := containerNames[msg.containerName]
				delete(containerNames, msg.containerName)
				p.updateState(containerNames, detectedVPPs, &cgi.containerGraphInfo)
			}
		case <-ticker.C: // regular interval updates
			p.updateState(containerNames, detectedVPPs, nil)
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

// probeInterval computes interval between probe checks based on configuration
func (p *Subprobe) probeInterval() time.Duration {
	interval := config.GetInt("agent.topology.docker.vpp.probeinterval")
	if interval < 1 { // making probe interval 0 would be CPU intensive and negative values doesn't make sense
		interval = 1
	}
	return time.Duration(interval) * time.Second
}

// updateState detects changes (up/down container, up/down VPP, VPP state changes, memif changes, VPP node connection
// changes) and updates graph according to new information.
func (p *Subprobe) updateState(containerNames map[string]*containerInfo, detectedVPPs map[string]*vppData, removedContainerGraphInfo *containerGraphInfo) {
	updateMemifs := p.updateStateForAllRunningContainers(containerNames, detectedVPPs)
	p.updateStateForRemovedContainers(containerNames, detectedVPPs, removedContainerGraphInfo)
	if updateMemifs {
		p.updateMemifs(containerNames, detectedVPPs) // update memif interfaces in existing containers if needed (these are possible cross container changes in graph->need to have all info updated first)
	}
}

// updateStateForAllRunningContainers checks VPP topology and updates graph if needed for every currently running container
func (p *Subprobe) updateStateForAllRunningContainers(containerNames map[string]*containerInfo, detectedVPPs map[string]*vppData) bool {
	// retrieve actual data about VPP
	dockerExecResults := p.dockerDataRetrieval(containerNames)

	// update state for all running containers based on new detected data (dockerExecResults)
	updateMemifs := false
	for containerName, containerInfo := range containerNames {
		updateMemifs = updateMemifs || p.updateStateForRunningContainer(containerName, containerInfo, detectedVPPs, dockerExecResults)
	}
	return updateMemifs
}

// updateStateForRunningContainer checks VPP topology and updates graph if needed for currently running container <containerName>.
func (p *Subprobe) updateStateForRunningContainer(containerName string, containerInfo *containerInfo, detectedVPPs map[string]*vppData,
	dockerExecResults map[string]map[string]string) bool {
	vpp, previouslyDetected := detectedVPPs[containerName]
	graphInfo := &containerInfo.containerGraphInfo
	vppVersion, detectedVPP := dockerExecResults[containerName][vppCLIShowVersion] // VPP version successfully retrieved <=> dockerExecResults contains result <=> detected running VPP

	if detectedVPP && !previouslyDetected { // new VPP detected
		vpp = &vppData{
			version:           vppVersion,
			linkedHardware:    make(map[string]struct{}),
			memifToSocketFile: make(map[string]string),
		}
		detectedVPPs[containerName] = vpp
		if err := p.addVPPToGraph(vpp, graphInfo, containerName); err != nil {
			logging.GetLogger().Errorf("Can't add vpp from container %v to graph (ignoring further graph additions "+
				"for this vpp) due to: %v", containerName, err)
		}
		p.updateHardwareLinks(vpp, containerName, graphInfo, dockerExecResults)
		return p.detectMemifs(vpp, containerName, containerInfo, dockerExecResults)
	} else if detectedVPP && previouslyDetected { // update previously detected VPP (metadata, connection to other nodes, etc.)
		p.updateHardwareLinks(vpp, containerName, graphInfo, dockerExecResults)
		return p.detectMemifs(vpp, containerName, containerInfo, dockerExecResults)
	} else if !detectedVPP && previouslyDetected { // previously detected VPP went down
		p.removeCachedMemifEdgesForContainer(graphInfo)
		p.deleteAllMemifsForVPPFromGraph(containerName, graphInfo)
		p.deleteVPPFromGraph(vpp, graphInfo)
		graphInfo.vppNodeID = "" // graphInfo cleanup of old data
		delete(detectedVPPs, containerName)
	}
	return false
}

// updateStateForRemovedContainers updates graph and tracked state related to removed containers that could in their
// previous running state populate graph/state with VPP topology nodes/data.
func (p *Subprobe) updateStateForRemovedContainers(containerNames map[string]*containerInfo, detectedVPPs map[string]*vppData, removedContainerGraphInfo *containerGraphInfo) {
	// clean already detected VPPs if they are not running in already removed container (removing container use case)
	for containerName, vpp := range detectedVPPs {
		if _, found := containerNames[containerName]; !found {
			p.removeCachedMemifEdgesForContainer(removedContainerGraphInfo)
			p.deleteAllMemifsForVPPFromGraph(containerName, removedContainerGraphInfo)
			p.deleteVPPFromGraph(vpp, removedContainerGraphInfo)
			delete(detectedVPPs, containerName)
		}
	}
}

// updateHardwareLinks is detecting Hardware Links between VPP and OS-level interfaces(Hardware that VPP is running on) and adding/removing them to/from graph.
// Detection is done only from vpp side, because change from OS side can be
// 1. addition (also vpp must add this otherwise no link is there) or
// 2. remove (edges disappear automatically in graph)
// and in either case, the problem will resolve itself just by watching VPP side.
func (p *Subprobe) updateHardwareLinks(vpp *vppData, containerName string, graphInfo *containerGraphInfo, dockerExecResults map[string]map[string]string) {
	diff, err := p.detectHardwareLinks(vpp, containerName, dockerExecResults)
	if err != nil {
		logging.GetLogger().Errorf("Can't detect VPP links to OS hardware interfaces in container %v due to: %v", containerName, err)
	} else {
		if diff {
			if err := p.updateVPPHardwareLinksToGraph(vpp, graphInfo); err != nil {
				logging.GetLogger().Errorf("Can't update VPP links to OS hardware interfaces in container %v due to: %v", containerName, err)
			}
		}
	}
}

// updateVPPHardwareLinksToGraph does update edges between VPP and Hardware nodes (interfaces like memif, veth,...)
func (p *Subprobe) updateVPPHardwareLinksToGraph(vpp *vppData, graphInfo *containerGraphInfo) error {
	p.graph.Lock()
	defer p.graph.Unlock()

	// initialize graph info if it was never used before
	if graphInfo.linkedHardwareNodesToEdges == nil {
		graphInfo.linkedHardwareNodesToEdges = make(map[graph.Identifier]graph.Identifier)
	}

	// get nodes
	vppNode := p.graph.GetNode(graphInfo.vppNodeID)
	if vppNode == nil {
		return fmt.Errorf("can't find graph node (vpp node) with id %v", graphInfo.vppNodeID)
	}
	nsRootNode := p.graph.GetNode(graphInfo.nsRootNodeID)
	if nsRootNode == nil {
		return fmt.Errorf("can't find graph node (docker container ns root node) with id %v", graphInfo.nsRootNodeID)
	}
	linkNodes := p.linkedHardwareNodes(nsRootNode, vpp)

	// create actual links (if they does not exist)
	for linkNodeID, linkNode := range linkNodes {
		if _, contains := graphInfo.linkedHardwareNodesToEdges[linkNodeID]; !contains {
			graphInfo.linkedHardwareNodesToEdges[linkNodeID] = graph.GenID()
			edge, err := p.graph.NewEdge(graphInfo.linkedHardwareNodesToEdges[linkNodeID], vppNode, linkNode, topology.Layer2Metadata())
			if err != nil {
				return fmt.Errorf("can't create edge between hardware node (id=%v) and vpp node (id=%v) due to: %v", linkNodeID, vppNode.ID, err)
			}
			p.graph.AddEdge(edge)
		}
	}

	// removing old edges the was detected and created in previous updates
	for oldLinkNodeID, oldLinkEdgeID := range graphInfo.linkedHardwareNodesToEdges {
		if _, contains := linkNodes[oldLinkNodeID]; !contains {
			delete(graphInfo.linkedHardwareNodesToEdges, oldLinkNodeID)
			e := p.graph.GetEdge(oldLinkEdgeID)
			if e == nil {
				logging.GetLogger().Warningf("can't remove VPP Hardware link edge, because we can't find it in graph by id (id=%v)", oldLinkEdgeID)
				continue
			}
			err := p.graph.DelEdge(e)
			if err != nil {
				logging.GetLogger().Warningf("can't remove VPP Hardware link edge(id=%v) due to: %v", oldLinkEdgeID, err)
			}
		}
	}

	return nil
}

// linkedHardwareNodes provides all currently existing graph nodes that represent hardware directly linked to VPP (as from VPP console "show hardware")
func (p *Subprobe) linkedHardwareNodes(nsRootNode *graph.Node, vpp *vppData) map[graph.Identifier]*graph.Node {
	linkNodes := make(map[graph.Identifier]*graph.Node)
	for _, rootEdge := range p.graph.GetNodeEdges(nsRootNode, topology.OwnershipMetadata()) {
		directChild := p.graph.GetNode(rootEdge.Child)
		if directChild == nil {
			logging.GetLogger().Warningf("can't find direct child(id %v)of nsRoot node, ignoring it", rootEdge.Child)
			continue
		}

		if t, ok := directChild.Metadata["Type"].(string); ok && t == "veth" {
			if name, ok := directChild.Metadata["Name"].(string); ok {
				if _, shouldBeLink := vpp.linkedHardware[name]; shouldBeLink {
					linkNodes[directChild.ID] = directChild
				}
			}
		}
	}
	return linkNodes
}

// detectHardwareLinks retrieves from VPP CLI information about used hardware links (links to physical hardware, i.e. eth0 interface)
// and returns whether these links changed in compare to previous state check.
func (p *Subprobe) detectHardwareLinks(vpp *vppData, containerName string, dockerExecResults map[string]map[string]string) (bool, error) {
	vppHardware, exists := dockerExecResults[containerName][vppCLIShowHardware]
	if !exists {
		return false, fmt.Errorf("can't get output from  %v, skipping hardware links additions", vppCLIShowHardware)
	}
	re, err := regexp.Compile(`\s*up\s*(\S*)\s`) //(?:up|down)
	if err != nil {
		return false, fmt.Errorf("can't compile regular expression for parsing vpp hardware listing output")
	}
	linkedHardware := make(map[string]struct{})
	for _, line := range strings.Split(vppHardware, "\n") {
		if matches := re.FindStringSubmatch(line); matches != nil {
			hardwareName := strings.TrimPrefix(matches[1], "host-") // removing "host" prefix for Veth links
			linkedHardware[hardwareName] = struct{}{}
		}
	}
	if !reflect.DeepEqual(vpp.linkedHardware, linkedHardware) {
		vpp.linkedHardware = linkedHardware
		return true, nil
	}
	return false, nil
}

// addVPPToGraph adds graph node representing running VPP
func (p *Subprobe) addVPPToGraph(vpp *vppData, graphInfo *containerGraphInfo, containerName string) error {
	p.graph.Lock()
	defer p.graph.Unlock()

	// get container node
	nsRootNode, err := p.nsRootNode(graphInfo, containerName)
	if err != nil {
		return fmt.Errorf("can't add vpp node to graph due to: %v", err)
	}

	// add vpp node
	metadata := graph.Metadata{
		"Type":      "vpp",
		"Manager":   "docker",
		"Name":      "VPP",
		"Container": containerName,
		"version":   vpp.version,
	}
	graphInfo.vppNodeID = graph.GenID()
	vppNode, err := p.graph.NewNode(graphInfo.vppNodeID, metadata)
	if err != nil {
		return fmt.Errorf("can't create new VPP node in graph (container %v) due to:%v", containerName, err)
	}
	topology.AddOwnershipLink(p.graph, nsRootNode, vppNode, nil)
	return nil
}

// nsRootNode retrieves graph.Identifier for container namespace root node (nsRoot). It contains also failsafe in case of
// changed graph.Identifier (nsRootNode is handled by docker probe) that finds nsRoot node by metadata search.
func (p *Subprobe) nsRootNode(graphInfo *containerGraphInfo, containerName string) (*graph.Node, error) {
	nsRootNode := p.graph.GetNode(graphInfo.nsRootNodeID)
	if nsRootNode == nil {
		// reacquiring nsRootNode based on metadata node search (reason: observed occasional change of namespace root node, so node.id is not stable)
		nsRootNodes := p.graph.GetNodes(graph.Metadata{"Type": "netns", "Name": containerName})
		if len(nsRootNodes) > 0 {
			nsRootNode = nsRootNodes[0]
			graphInfo.nsRootNodeID = nsRootNode.ID
		} else {
			return nil, fmt.Errorf("can't find graph node (docker container ns root node) with id %v", graphInfo.nsRootNodeID)
		}
	}
	return nsRootNode, nil
}

// deleteVPPFromGraph removes graph node representing running VPP
func (p *Subprobe) deleteVPPFromGraph(vpp *vppData, graphInfo *containerGraphInfo) {
	p.graph.Lock()
	defer p.graph.Unlock()

	vppNode := p.graph.GetNode(graphInfo.vppNodeID)
	if vppNode == nil {
		logging.GetLogger().Errorf("can't find previously created vpp node with id %v", graphInfo.vppNodeID)
		return
	}

	if err := p.graph.DelNode(vppNode); err != nil {
		logging.GetLogger().Errorf("can't delete vpp node with id %v", graphInfo.vppNodeID)
		return
	}
}
