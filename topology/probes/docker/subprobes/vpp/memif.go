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
	"reflect"
	"strings"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
)

const memifTunnelType = "memif"

// detectMemifs detects all memifs and returns whether memifs changed in compare to previously detected memifs.
func (p *Subprobe) detectMemifs(vpp *vppData, containerName string, containerInfo *containerInfo, dockerExecResults map[string]map[string]string) bool {
	infoStr, exists := dockerExecResults[containerName][vppCLIShowMemif]
	if !exists {
		logging.GetLogger().Errorf("can't get output from  %v, skipping memif detection", vppCLIShowMemif)
		return false
	}

	// retrieving mapping between memif and path to its socket (path on host OS)
	socketIDToPath := make(map[string]string)
	socketsPart := strings.Split(infoStr, "interface")[0]
	socketsDataParts := strings.Split(socketsPart, "\n")[2:]
	for _, line := range socketsDataParts {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			data := strings.Split(trimmed, " ")
			socketIDToPath[data[0]] = data[len(data)-1]
		}
	}
	memifToSocketFile := make(map[string]string)
	for hw := range vpp.linkedHardware {
		if strings.HasPrefix(hw, "memif") {
			ids := strings.Split(strings.TrimPrefix(hw, "memif"), "/")
			if len(ids) != 2 {
				logging.GetLogger().Errorf("unexpected memif name: %v, expected memif[socket-id]/[id]", hw)
				return false
			}
			vppSocketPath := socketIDToPath[ids[0]]
			for hostPath, vppPath := range containerInfo.volumes {
				if strings.HasPrefix(vppSocketPath, vppPath) {
					memifToSocketFile[hw] = hostPath + strings.TrimPrefix(vppSocketPath, vppPath)
					continue
				}
			}
		}
	}
	if !reflect.DeepEqual(vpp.memifToSocketFile, memifToSocketFile) {
		vpp.memifToSocketFile = memifToSocketFile
		return true
	}
	return false
}

func (p *Subprobe) removeCachedMemifEdgesForContainer(graphInfo *containerGraphInfo) {
	// creating helping data structure to ask directly if memifNodeID should be removed
	removalNodeIDs := make(map[graph.Identifier]struct{})
	for _, memifNodeID := range graphInfo.memifNametoNodeID {
		removalNodeIDs[memifNodeID] = struct{}{}
	}

	// update global values (all containerInfo.containerGraphInfo.globalMemifEdges references the same global instance -> need to update it only once using any reference to it)
	for node1ID, inner := range graphInfo.globalMemifEdges {
		if _, contains := removalNodeIDs[node1ID]; contains {
			delete(graphInfo.globalMemifEdges, node1ID)
			continue
		}
		for node2ID := range inner {
			if _, contains := removalNodeIDs[node2ID]; contains {
				delete(graphInfo.globalMemifEdges[node1ID], node2ID)
			}
		}
	}
}

func (p *Subprobe) deleteAllMemifsForVPPFromGraph(containerName string, containerGraphInfo *containerGraphInfo) {
	for memif, nodeID := range containerGraphInfo.memifNametoNodeID {
		p.deleteMemifFromGraph(nodeID, memif, containerName)
	}
	containerGraphInfo.memifNametoNodeID = make(map[string]graph.Identifier)
}

// updateMemifs updates memif and related edges in graph. This method should be called only if memif change is detected.
func (p *Subprobe) updateMemifs(containerNames map[string]*containerInfo, detectedVPPs map[string]*vppData) {
	p.addMissingMemifs(containerNames, detectedVPPs)
	memifEdges := p.computeMemifEdgesGraphInfo(containerNames, detectedVPPs)
	p.addMemIfTunnelEdgesToGraph(memifEdges)
	p.saveMemifEdgesGraphInfo(containerNames, memifEdges)
	p.removeObsoleteMemifs(containerNames, detectedVPPs)
}

// addMissingMemifs adds memifs detected in VPP that are not in graph
func (p *Subprobe) addMissingMemifs(containerNames map[string]*containerInfo, detectedVPPs map[string]*vppData) {
	for containerName, containerInfo := range containerNames {
		if vpp, existsVpp := detectedVPPs[containerName]; existsVpp {
			for memif := range vpp.memifToSocketFile {
				if containerInfo.containerGraphInfo.memifNametoNodeID == nil {
					containerInfo.containerGraphInfo.memifNametoNodeID = make(map[string]graph.Identifier)
				}
				if _, contains := containerInfo.containerGraphInfo.memifNametoNodeID[memif]; !contains {
					p.addMemifToGraph(memif, &containerInfo.containerGraphInfo, containerName)
				}
			}
		}
	}
}

// computeMemifEdgesGraphInfo computes new edges for memif tunnels based detected vpp info. Edge Ids are of course not
// computed here (they are created by edge creation), but places in map are created here. In edge creation, the ids are
// filled at right places in data structure computed here.
func (p *Subprobe) computeMemifEdgesGraphInfo(containerNames map[string]*containerInfo, detectedVPPs map[string]*vppData) map[graph.Identifier]map[graph.Identifier]graph.Identifier {
	graphInfo := p.firstContainerGraphInfo(containerNames)
	if graphInfo == nil {
		logging.GetLogger().Errorf("can't get first container graph info -> can't get global memif edges info->skipping memif/memif edges update")
	}
	memifEdges := make(map[graph.Identifier]map[graph.Identifier]graph.Identifier) // map[first memif node id]map[second memif node id] = id of edge connecting that 2 memifs
	socketFileToMemifs := p.computeSocketFileMemifMapping(containerNames, detectedVPPs)

	// computing all edges and copying edge id already used in graph if edge is already in graph (this can be also done by make 5 for cycles, but computation complexity explodes with higher count of containers/memifs)
	for containerName, containerInfo := range containerNames {
		if vpp, existsVpp := detectedVPPs[containerName]; existsVpp {
			for memif, hostSocketFile := range vpp.memifToSocketFile {
				memif1NodeID := containerInfo.containerGraphInfo.memifNametoNodeID[memif]
				for _, memif2NodeID := range p.otherEndsOfMemifTunnel(containerName, memif, hostSocketFile, socketFileToMemifs) {
					if memif1NodeID < memif2NodeID { // filtering duplicate edges (1 edge will be detected 2 times -> using only 1 detection by introducing artificial condition that is true only for 1 edge from 2 detections)
						if memifEdges[memif1NodeID] == nil {
							memifEdges[memif1NodeID] = make(map[graph.Identifier]graph.Identifier)
						}
						memifEdges[memif1NodeID][memif2NodeID] = graphInfo.globalMemifEdges[memif1NodeID][memif2NodeID] //if not in graphInfo.globalMemifEdges, default value will be filled ("") and later edge id will created and filed here (new edges detection condition: memifEdges[x][y] == "")
						if graphInfo.globalMemifEdges[memif1NodeID][memif2NodeID] != "" {
							graphInfo.globalMemifEdges[memif1NodeID][memif2NodeID] = "" // need for further finding edges for removal
						}
					}
				}
			}
		}
	}
	return memifEdges
}

func (p *Subprobe) firstContainerGraphInfo(containerNames map[string]*containerInfo) *containerGraphInfo {
	for _, contrainerInfo := range containerNames {
		return &contrainerInfo.containerGraphInfo
	}
	return nil
}

// otherEndsOfMemifTunnel computes nodeIDs of memif node that represents end point of tunnels that start with given node (<oneEndContainerName>, <oneEndMemif>).
func (p *Subprobe) otherEndsOfMemifTunnel(oneEndContainerName string, oneEndMemif string, hostSocketFile string,
	socketFileToMemifs map[string][]string) []graph.Identifier {
	ends := make([]graph.Identifier, 0)
	for _, pairMemIfStr := range socketFileToMemifs[hostSocketFile] {
		pairMemIf := strings.Split(pairMemIfStr, "#")
		if !(pairMemIf[0] == oneEndContainerName && pairMemIf[1] == oneEndMemif) { // not current memif => other memif connected with current memif
			ends = append(ends, graph.Identifier(pairMemIf[2]))
		}
	}
	return ends
}

// computeSocketFileMemifMapping computes mapping that maps host socket file to memif identification tuple (containerName, memif, memifNodeID)
func (p *Subprobe) computeSocketFileMemifMapping(containerNames map[string]*containerInfo, detectedVPPs map[string]*vppData) map[string][]string {
	// creating help mapping for connecting memifs (reverse map of remembered data)
	socketFileToMemifs := make(map[string][]string)
	for containerName, containerInfo := range containerNames {
		if vpp, existsVpp := detectedVPPs[containerName]; existsVpp {
			for memif, hostSocketFile := range vpp.memifToSocketFile {
				memifNodeID := string(containerInfo.containerGraphInfo.memifNametoNodeID[memif]) // graph.Identifier = string
				socketFileToMemifs[hostSocketFile] = append(socketFileToMemifs[hostSocketFile], containerName+"#"+memif+"#"+memifNodeID)
			}
		}
	}
	return socketFileToMemifs
}

func (p *Subprobe) addMemIfTunnelEdgesToGraph(memifEdges map[graph.Identifier]map[graph.Identifier]graph.Identifier) {
	p.graph.Lock()
	defer p.graph.Unlock()

	for memif1NodeID, inner := range memifEdges {
		for memif2NodeID, edgeID := range inner {
			if edgeID == "" {
				// get nodes
				memif1Node := p.graph.GetNode(memif1NodeID)
				if memif1Node == nil {
					logging.GetLogger().Errorf("can't find graph node (first memif node) with id %v", memif1Node)
				}
				memif2Node := p.graph.GetNode(memif2NodeID)
				if memif1Node == nil {
					logging.GetLogger().Errorf("can't find graph node (second memif node) with id %v", memif2Node)
				}

				memifEdges[memif1NodeID][memif2NodeID] = graph.GenID()
				edge, err := p.graph.NewEdge(memifEdges[memif1NodeID][memif2NodeID], memif1Node, memif2Node, p.memIfTunnelEdgeMetadata())
				if err != nil {
					logging.GetLogger().Errorf("can't add graph edge between 2 memif nodes (%v <-> %v) due to: %v", memif1Node.ID, memif2Node.ID, err)
				}
				p.graph.AddEdge(edge)
			}
		}
	}
}

func (p *Subprobe) memIfTunnelEdgeMetadata() graph.Metadata {
	return graph.Metadata{"RelationType": topology.Layer2Link, "Type": memifTunnelType}
}

func (p *Subprobe) deleteMemifFromGraph(nodeID graph.Identifier, memif string, containerName string) {
	p.graph.Lock()
	defer p.graph.Unlock()

	memifNode := p.graph.GetNode(nodeID)
	if memifNode == nil {
		logging.GetLogger().Errorf("can't delete memif due to: can't find previously created memif (%v) node with id %v (container=%v)", memif, memifNode, containerName)
		return
	}

	if err := p.graph.DelNode(memifNode); err != nil {
		logging.GetLogger().Errorf("can't delete memif node with id %v due to: %v", nodeID, err)
	}
}

func (p *Subprobe) addMemifToGraph(memif string, containerGraphInfo *containerGraphInfo, containerName string) {
	p.graph.Lock()
	defer p.graph.Unlock()

	nsRootNode, err := p.nsRootNode(containerGraphInfo, containerName)
	if err != nil {
		logging.GetLogger().Errorf("can't add memif %v for vpp, because we can't find graph node (nsRoot node) with id %v due to: %v", memif, containerGraphInfo.nsRootNodeID, err)
		return
	}

	containerGraphInfo.memifNametoNodeID[memif] = graph.GenID()
	metadata := graph.Metadata{
		"Type":    "intf",
		"Manager": "docker",
		"Name":    memif,
	}
	memifNode, err := p.graph.NewNode(containerGraphInfo.memifNametoNodeID[memif], metadata)
	if err != nil {
		logging.GetLogger().Errorf("can't create memif (metadata=%+v) due to: %v", metadata, err)
		return
	}

	topology.AddOwnershipLink(p.graph, nsRootNode, memifNode, nil)

	vppNode := p.graph.GetNode(containerGraphInfo.vppNodeID)
	if vppNode == nil {
		logging.GetLogger().Errorf("can't link memif %v with vpp node, because we can't find graph node (vpp node) with id %v", memif, containerGraphInfo.vppNodeID)
		return
	}
	edge, err := p.graph.NewEdge(graph.GenID(), vppNode, memifNode, topology.Layer2Metadata()) // graph.Metadata{"RelationType": topology.Layer2Link, "Type": "veth"}
	if err != nil {
		logging.GetLogger().Errorf("can't link memif %v with vpp node %v due to: %v", memif, containerGraphInfo.vppNodeID, err)
		return
	}
	p.graph.AddEdge(edge)
}

// saveMemifEdgesGraphInfo sets global memif edge information. It must set it to all containers so it wont be lost when
// container is removed. Last container removal will remove also this information.
func (p *Subprobe) saveMemifEdgesGraphInfo(containerNames map[string]*containerInfo, memifEdges map[graph.Identifier]map[graph.Identifier]graph.Identifier) {
	for _, containerInfo := range containerNames {
		containerInfo.containerGraphInfo.globalMemifEdges = memifEdges
	}
}

// removeObsoleteMemifs removes memif nodes that are in graph, but not currently detected in vpp
func (p *Subprobe) removeObsoleteMemifs(containerNames map[string]*containerInfo, detectedVPPs map[string]*vppData) {
	for containerName, containerInfo := range containerNames {
		if vpp, existsVpp := detectedVPPs[containerName]; existsVpp {
			for memif, nodeID := range containerInfo.containerGraphInfo.memifNametoNodeID {
				if _, contains := vpp.memifToSocketFile[memif]; !contains {
					p.deleteMemifFromGraph(nodeID, memif, containerName)
					delete(containerInfo.containerGraphInfo.memifNametoNodeID, memif)
				}
			}
		}
	}
}
