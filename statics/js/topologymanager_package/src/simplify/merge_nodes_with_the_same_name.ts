import TransformationRegistry from './transformation';
import BaseSimplificationStrategy from './base';
import { transformSkydiveDataForSimplification } from "../helpers";

const patternsToBeSimplified: any = [
    {
        "nodes": {
            "openvswitchpatch": {
                "main": true,
                "Metadata": {
                    "Driver": "openvswitch",
                    "Type": "patch"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
        },
        "edges": {
            "ovsportopenvswitchpatchlayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "openvswitchpatch"
            }
        }
    }, {
        "nodes": {
            "openvswitchdpdkvhostuserclient": {
                "main": true,
                "Metadata": {
                    "Driver": "openvswitch",
                    "Type": "dpdkvhostuserclient"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
        },
        "edges": {
            "ovsportopenvswitchdpdkvhostuserclientlayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "openvswitchdpdkvhostuserclient"
            }
        }
    }, {
        "nodes": {
            "net_ixgbedpdk": {
                "main": true,
                "Metadata": {
                    "Driver": "net_ixgbe",
                    "Type": "dpdk"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
        },
        "edges": {
            "ovsportnet_ixgbedpdklayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "net_ixgbedpdk"
            }
        }
    }, {
        "nodes": {
            "tuninternal": {
                "main": true,
                "Metadata": {
                    "Driver": "tun",
                    "Type": "internal"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
            "ovsbridge": {
                "Metadata": {
                    "Type": "ovsbridge"
                }
            },
        },
        "edges": {
            "ovsporttuninternallayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "tuninternal"
            }, "ovsbridgeovsportownership": {
                "type": "ownership",
                "parent": "ovsbridge",
                "child": "ovsport"
            }, "ovsbridgeovsportlayer2": {
                "type": "layer2",
                "parent": "ovsbridge",
                "child": "ovsport"
            }
        }
    }, {
        "nodes": {
            "vethveth": {
                "main": true,
                "Metadata": {
                    "Driver": "veth",
                    "Type": "veth"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
        },
        "edges": {
            "ovsportvethvethlayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "vethveth"
            }
        }
    }
];

export default class StructureSimplificationStrategy extends BaseSimplificationStrategy {

    findSimilarNodes(skydiveData: any) {
        let nodes: any = {};
        let nodeNamesIds: any = {}
        Object.keys(this.data).forEach((nodeId: any) => {
            if (!nodeNamesIds[this.data[nodeId].Name]) {
                nodeNamesIds[this.data[nodeId].Name] = new Set();
            }
            nodeNamesIds[this.data[nodeId].Name].add(nodeId);
        });
        Object.keys(nodeNamesIds).forEach((nodeName: any) => {

            if (nodeNamesIds[nodeName].size == 1) {
                return;
            }
            // try to find a topology from list of hardcoded
            // topologies
            let nodeIdToPatternId: any = {};
            const patternToBeUsedToSimplifyPartOfGraph = patternsToBeSimplified.find((pattern: any) => {
                if (nodeNamesIds[nodeName].size !== Object.keys(pattern.nodes).length) {
                    return false;
                }
                let isFoundInPattern = false;
                isFoundInPattern = false;
                nodeIdToPatternId = {};
                for (var it = nodeNamesIds[nodeName].values(), nodeId1: any = null; nodeId1 = it.next().value;) {
                    Object.keys(pattern.nodes).forEach((patternID: any) => {
                        const nodePattern = pattern.nodes[patternID];
                        if (nodePattern.Metadata.Driver && nodePattern.Metadata.Driver !== this.data[nodeId1].Driver) {
                            return;
                        }
                        if (nodePattern.Metadata.Type !== this.data[nodeId1].Type) {
                            return;
                        }
                        nodeIdToPatternId[nodeId1] = (nodePattern.Metadata.Driver || "") + nodePattern.Metadata.Type;
                        isFoundInPattern = true;
                    });
                }
                const edgesToCompare: any = {};
                skydiveData.Obj.Edges.forEach((edge: any) => {
                    if (!nodeIdToPatternId[edge.Parent] || !nodeIdToPatternId[edge.Child]) {
                        return;
                    }
                    edgesToCompare[nodeIdToPatternId[edge.Parent] + nodeIdToPatternId[edge.Child] + edge.Metadata.RelationType] = {
                        "type": edge.Metadata.RelationType,
                        "parent": nodeIdToPatternId[edge.Parent],
                        "child": nodeIdToPatternId[edge.Child],
                    };
                });
                if (Object.keys(edgesToCompare).length !== Object.keys(pattern.edges).length) {
                    return false;
                }
                const edges: Set<String> = new Set(Object.keys(edgesToCompare));
                const patternEdges: Set<String> = new Set(Object.keys(pattern.edges));
                let difference = new Set(
                    [...edges].filter(x => !patternEdges.has(x)));
                if (difference.size) {
                    return false;
                }
                return true;
            });
            if (!patternToBeUsedToSimplifyPartOfGraph) {
                return;
            }
            const mainNodeId = Object.keys(nodeIdToPatternId).find((nodeId: any) => {
                return !!patternToBeUsedToSimplifyPartOfGraph.nodes[nodeIdToPatternId[nodeId]].main;
            });
            Object.keys(nodeIdToPatternId).forEach((nodeId: any) => {
                if (nodeId === mainNodeId) {
                    return;
                }
                nodes[nodeId] = mainNodeId;
            });
        });
        return nodes;
    }

    mergeAllNodes(dataFormat: String, data: any, similarNodes: any) {
        const transformationHandler = TransformationRegistry.getMergeNodeStrategy(dataFormat);
        return transformationHandler(data, similarNodes);
    }

    simplifyData(dataFormat: String, data: any) {
        this.data = transformSkydiveDataForSimplification(data);
        const similarNodes = this.findSimilarNodes(data);
        return this.mergeAllNodes(dataFormat, data, similarNodes);
    }

}
