import * as deepAssign from 'deep-assign';
import TransformationRegistry from './transformation';
import BaseSimplificationStrategy from './base';
import { transformSkydiveDataForSimplification } from "../helpers";


function isEqualHashMap(data: any, compareTo: any) {
    return Object.keys(compareTo).every((key: any, i: number, arr: any): boolean => {
        if (typeof compareTo[key] === "string" && compareTo[key] !== data[key]) {
            return false;
        }
        if (typeof compareTo[key] === "string" && compareTo[key] === data[key]) {
            return true;
        }
        return isEqualHashMap(data[key], compareTo[key]);
    });
}


const patternsToBeFlattened: any = [
    {
        "topology": {
            "first": {
                "Metadata": {
                    "Driver": "tun",
                    "Type": "internal"
                }
            },
            "nodes": [
                {
                    "node": {
                        "Metadata": {
                            "Driver": "openvswitch",
                            "Type": "patch",
                        }
                    },
                    "edges": [
                        {
                            "Metadata": {
                                "RelationType": "ownership"
                            },
                            "Parent": "parent",
                            "Child": "me",
                        },
                        {
                            "Metadata": {
                                "RelationType": "layer2"
                            },
                            "Parent": "parent",
                            "Child": "me",
                        }
                    ],
                    "nodes": [
                        {
                            "node": {
                                "Metadata": {
                                    "Driver": "openvswitch",
                                    "Type": "patch",
                                }
                            },
                            "edges": [
                                {
                                    "Metadata": {
                                        "RelationType": "layer2",
                                        "Type": "patch"
                                    },
                                    "Parent": "parent",
                                    "Child": "me",
                                }
                            ],
                            "nodes": []
                        }

                    ]
                },
            ]
        }
    },
    {
        "topology": {
            "first": {
                "Metadata": {
                    "Driver": "tun",
                    "Type": "internal"
                }
            },
            "nodes": [
                {
                    "node": {
                        "Metadata": {
                            "Driver": "openvswitch",
                            "Type": "patch",
                        }
                    },
                    "edges": [
                        {
                            "Metadata": {
                                "RelationType": "ownership"
                            },
                            "Parent": "me",
                            "Child": "parent",
                        },
                        {
                            "Metadata": {
                                "RelationType": "layer2"
                            },
                            "Parent": "me",
                            "Child": "parent",
                        }
                    ],
                    "nodes": [
                        {
                            "node": {
                                "Metadata": {
                                    "Driver": "openvswitch",
                                    "Type": "patch",
                                }
                            },
                            "edges": [
                                {
                                    "Metadata": {
                                        "RelationType": "layer2",
                                        "Type": "patch"
                                    },
                                    "Parent": "me",
                                    "Child": "parent",
                                }
                            ],
                            "nodes": []
                        }

                    ]
                },
            ]
        }
    }
];




export default class FlattenSamplesOfTopologyStrategy extends BaseSimplificationStrategy {

    doesConnectionMatchTwoNodes(connection: any, parentNodeID: any, childNodeID: any) {
        if (connection.Metadata.RelationType !== "ownership") {
            if (connection.Child === "me" && !this.data[parentNodeID].layers.has(childNodeID)) {
                return false;
            }
            if (connection.Parent === "parent" && !this.data[childNodeID].layers.has(parentNodeID)) {
                return false;
            }
            if (connection.Child === "parent" && !this.data[childNodeID].layers.has(parentNodeID)) {
                return false;
            }
            if (connection.Parent === "me" && !this.data[parentNodeID].layers.has(childNodeID)) {
                return false;
            }
        }
        if (connection.Metadata.RelationType === "ownership") {
            if (connection.Child === "me" && !this.data[parentNodeID].children.has(childNodeID)) {
                return false;
            }
            if (connection.Parent === "parent" && !this.data[childNodeID].parent.has(parentNodeID)) {
                return false;
            }
        }
        return true;
    }

    returnDeadNodesIfNodesConnectedToNodeMatchExpectedConnections(mainNode: any, nodeToCheck: any) {
        let deadNodes: any = [];
        const layerConnections: any = nodeToCheck.edges.reduce((accum: any, edge: any) => {
            if (edge.Metadata.RelationType !== "ownership") {
                accum.push(nodeToCheck);
            }
            return accum;
        }, []);
        const parentConnections: any = nodeToCheck.edges.reduce((accum: any, edge: any) => {
            if (edge.Metadata.RelationType === "ownership" && edge.Child === "me") {
                accum.push(nodeToCheck);
            }
            return accum;
        }, []);
        const childConnections: any = nodeToCheck.edges.reduce((accum: any, edge: any) => {
            if (edge.Metadata.RelationType === "ownership" && edge.Parent === "me") {
                accum.push(nodeToCheck);
            }
            return accum;
        }, []);
        let isMatched = false;
        let matchedNode: any = null;
        this.data[mainNode.ID].layers.forEach((layerId: any) => {
            if (isMatched) {
                return;
            }
            layerConnections.forEach((layerData: any) => {
                if (isMatched) {
                    return;
                }
                if (!isEqualHashMap({ Metadata: this.data[layerId] }, layerData.node)) {
                    return;
                }
                isMatched = layerData.edges.every((connection: any) => {
                    return this.doesConnectionMatchTwoNodes(connection, mainNode.ID, layerId);
                });
                if (isMatched) {
                    matchedNode = {
                        ID: layerId,
                        Metadata: this.data[layerId]
                    }
                }

            });
        });
        this.data[mainNode.ID].parent.forEach((parentId: any) => {
            if (isMatched) {
                return;
            }
            parentConnections.forEach((parentData: any) => {
                if (isMatched) {
                    return;
                }
                if (!isEqualHashMap({
                    Metadata: this.data[parentId]
                }, parentData.node)) {
                    return;
                }
                isMatched = parentData.edges.every((connection: any) => {
                    return this.doesConnectionMatchTwoNodes(connection, mainNode.ID, parentId);
                });
                if (isMatched) {
                    matchedNode = {
                        ID: parentId,
                        Metadata: this.data[parentId]
                    }
                }

            });
        });
        this.data[mainNode.ID].children.forEach((childrenId: any) => {
            if (isMatched) {
                return;
            }
            childConnections.forEach((childData: any) => {
                if (isMatched) {
                    return;
                }
                if (!isEqualHashMap({ Metadata: this.data[childrenId] }, childData.node)) {
                    return;
                }
                isMatched = childData.edges.every((connection: any) => {
                    return this.doesConnectionMatchTwoNodes(connection, mainNode.ID, childrenId);
                });
                if (isMatched) {
                    matchedNode = {
                        ID: childrenId,
                        Metadata: this.data[childrenId]
                    }
                }
            });
        });
        if (isMatched && nodeToCheck.nodes.length) {
            // console.log('matched node and edges', JSON.stringify(mainNode), JSON.stringify(nodeToCheck));
            nodeToCheck.nodes.every((nodeConnectedTo: any) => {
                if (!isEqualHashMap(matchedNode, nodeConnectedTo.node)) {
                    return true;
                }
                if (matchedNode.ID === mainNode.ID) {
                    return true;
                }
                const isMatchedAndDeadNodes: any = this.returnDeadNodesIfNodesConnectedToNodeMatchExpectedConnections(matchedNode, nodeConnectedTo);
                if (!isMatchedAndDeadNodes.isMatched) {
                    return false;
                }
                deadNodes = deadNodes.concat(isMatchedAndDeadNodes.deadNodes);
                return true;
            });
        } else {
            if (nodeToCheck.nodes.length) {
                // console.log('not matched node and edges', JSON.stringify(mainNode), JSON.stringify(nodeToCheck));
            } else {
                // console.log('matched but no nodes/edges need to be checked', matchedNode.ID);
                isMatched = true;
            }
        }
        if (isMatched) {
            deadNodes.push(matchedNode.ID);
        }
        return {
            isMatched, deadNodes
        };
    }

    findDeadNodesInSpecificSubTopology(node: any, topology: any): any {
        let deadNodes: any = {};
        const isFoundAnyNotMatchedNode = !topology.nodes.every((nodeConnectedTo: any) => {
            const isMatchedAndDeadNodes: any = this.returnDeadNodesIfNodesConnectedToNodeMatchExpectedConnections(node, nodeConnectedTo);
            if (!isMatchedAndDeadNodes.isMatched) {
                return false;
            }
            deadNodes = deepAssign(
                deadNodes,
                isMatchedAndDeadNodes.deadNodes.reduce((accum: any, deadNode: any) => {
                    accum[deadNode] = node.ID;
                    return accum;
                }, {})
            );
            return true;
        });
        if (isFoundAnyNotMatchedNode) {
            return {};
        }
        return deadNodes;
    }

    flattenSamplesOfTopology(data: any): any {
        let topologyDeadNodes: any = {};
        const nodesToBeFlattened: any = {};
        while (true) {
            const notReplaced = patternsToBeFlattened.some((topology: any) => {
                data.Obj.Nodes.forEach((node: any) => {
                    if (!isEqualHashMap(node, topology.topology.first)) {
                        return false;
                    }
                    console.log('matched node - try to flat sub graph', node.Metadata.Name, topology.topology.first);
                    const deadNodes = this.findDeadNodesInSpecificSubTopology(node, topology.topology);
                    console.log('matched node - try to flat sub graph. Dead nodes are', deadNodes);
                    if (!Object.keys(deadNodes).length) {
                        return false;
                    }
                    const transformationHandler = TransformationRegistry.getMergeNodeStrategy("skydive");
                    data = transformationHandler(data, deadNodes);
                    // console.log(JSON.stringify(data));
                    this.data = transformSkydiveDataForSimplification(data);
                    return true;
                });
            });
            if (!notReplaced) {
                break;
            }
        };
        return data;
    }

    simplifyData(dataFormat: String, data: any) {
        this.data = transformSkydiveDataForSimplification(data);
        data = this.flattenSamplesOfTopology(data);
        return data;
    }

}
