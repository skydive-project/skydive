import * as deepAssign from 'deep-assign';

/**
Transforms data from skydive format into format needed for
 **simplification
We need to have a graph with a following structure. Each root should
 **be similar to {
  "Host": HostName,
  "Name": NodeName,
  "Type": NodeType,
  "children": set of children nodes,
  "parent": set of parent nodes,
  "layers": set of layered nodes
}
**/
export function transformSkydiveDataForSimplification(data: any): any {
    let simplifyData: any = {}
    data.Obj.Nodes.forEach((item: any) => {
        simplifyData[item.ID] = {
            Host: item.Host,
            Name: item.Metadata.Name,
            Type: item.Metadata.Type,
            Driver: item.Metadata.Driver,
            children: new Set([]),
            parent: new Set([]),
            layers: new Set([])
        }
    });
    data.Obj.Edges.forEach((item: any) => {
        // this is some kind of layer between two nodes, so lets make
        // this node to have both nodes as parent and children
        if (item.Metadata.RelationType !== "ownership") {
            simplifyData[item.Child].layers.add(item.Parent);
            simplifyData[item.Parent].layers.add(item.Child);
        } else {
            simplifyData[item.Parent].children.add(item.Child);
            simplifyData[item.Child].parent.add(item.Parent);
        }
    });
    return simplifyData;
}

function recursivelyBuildHierarchy(parentNode: any, nodesInfo: any, data: any) {
    if (parentNode.processed) {
        return;
    }
    parentNode.processed = true;
    let children: any = new Set([...parentNode.childrenIds]);
    // remove duplicated nodes from skydive hierarchy
    children.forEach((nodeId: any) => {
        const nodeInfo = nodesInfo[nodeId];
        if (parentNode.Type !== "libvirt" && nodeInfo.Name === parentNode.Name) {
            children.delete(nodeId);
            parentNode.childrenIds.delete(nodeId);
            nodeInfo.childrenIds.forEach((nodeId1: any) => {
                children.add(nodeId1);
                parentNode.childrenIds.add(nodeId1);
            });
            nodeInfo.layerIds.forEach((nodeId1: any) => {
                children.add(nodeId1);
                parentNode.childrenIds.add(nodeId1);
            });

        }
    });
    children.forEach((nodeID: any) => {
        const nodeInfo = nodesInfo[nodeID];
        if (!nodeInfo) {
            return;
        }
        if (nodeInfo) {
            if (nodeInfo.Type !== "host") {
                parentNode.children.push(deepAssign(nodeInfo, data[nodeID]));
            }
        }
        recursivelyBuildHierarchy(nodesInfo[nodeID], nodesInfo, data);
    });
}

export function prepareDataForHierarchyLayout(initialData: any) {
    const data = transformSkydiveDataForSimplification(initialData);

    const node = {
        Type: "tophierarchy",
        Name: "Hierarchy top",
        ID: "Tophierarchy",
        childrenIds: new Set([]),
        children: new Array(),
        layerIds: new Set([]),
        processed: false
    }
    const nodesInfo: any = {};
    nodesInfo[node.ID] = node;
    let nodes: Array<String> = [];
    const nodeTypesToCheck: Array<String> = ["libvirt"];
    const skipHosts: Set<String> = new Set([]);
    // find initial list of nodes, actually, its vm nodes
    Object.keys(data).forEach((nodeId: any) => {
        if (nodeTypesToCheck.indexOf(data[nodeId].Type) === -1) {
            return;
        }
        nodes.push(nodeId);
    });

    let i = 0;
    let nodesCount: number = Object.keys(data).length;
    const processedNodes: Set<String> = new Set([]);
    while (true) {
        const nodeId: any = nodes[i];
        const nodeData = data[nodeId];
        if (!nodeData) {
            break
        }
        let goThroughNodes: Array<String> = [];
        try {
            if (!nodesInfo[nodeId]) {
                nodesInfo[nodeId] = {
                    Type: nodeData.Type,
                    childrenIds: new Set([]),
                    ID: nodeId,
                    children: [],
                    layerIds: new Set([]),
                    processed: false,
                    Name: nodeData.Name
                }
            }
            if (nodeData.Type === "libvirt") {
                node.childrenIds.add(nodeId);
            }
            if (nodeData.Type === "host") {
                continue
            }
            // sometimes nodes are not connected directly but
            // connected through layer2 relations, its important
            // for us as well, for instance, here is a typical
            // connection from skydive data structure
            // {
            // "ID":"08883be7-f8be-583d-4bf8-fcd3e611a05f",
            // "Metadata":{
            // "RelationType":"layer2"
            // },
            // "Parent":"ec13216d-af7e-42df-5efe-a478733545de",
            // "Child":"71ace516-0ade-432f-4531-4b5e2a828ced",
            // "Host":"DELL2",
            // "CreatedAt":1524827146311,
            // "UpdatedAt":1524827146311
            // },
            // it needs to be treated as well to make sure we
            // don't lose any information from topology
            if (nodeData.layers.size) {
                const nodesToBeAdded = nodeData.layers;
                nodesToBeAdded.forEach((nodeId1: any) => {
                    nodesInfo[nodeId].childrenIds.add(nodeId1);
                    nodeData.children.add(nodeId1);
                    data[nodeId1].layers.delete(nodeId);
                    data[nodeId1].children.delete(nodeId);
                    if (nodesInfo[nodeId1]) {
                        nodesInfo[nodeId1].layerIds.delete(nodeId);
                        nodesInfo[nodeId1].childrenIds.delete(nodeId);
                    }
                });
                goThroughNodes = goThroughNodes.concat([...nodesToBeAdded]);
            }
            // handle list of children of specific node
            if (nodeData.children.size) {
                const childrenToBeAdded = nodeData.children;
                if (childrenToBeAdded.size) {
                    goThroughNodes = goThroughNodes.concat([...childrenToBeAdded]);
                }
            }
            // handle list of parent of specific node
            if (nodeData.parent.size) {
                const parentsToBeAdded = nodeData.parent;
                if (parentsToBeAdded.size) {
                    parentsToBeAdded.forEach((nodeId1: any) => {
                        nodesInfo[nodeId].childrenIds.add(nodeId1);
                    });
                    goThroughNodes = goThroughNodes.concat([...parentsToBeAdded]);
                }
            }
        } finally {
            // break if total list of handled nodes equal total
            // size of topology
            if (processedNodes.size === nodesCount) {
                break;
            }
            // extend list of nodes to be visited
            if (goThroughNodes.length) {
                const uniqueNodes = new Set(goThroughNodes);
                let uniqueNodes1 = [...uniqueNodes];
                nodes = nodes.concat(uniqueNodes1);
            }
            processedNodes.add(nodeId);
            ++i;
        }
    }
    const IDToData: any = {};
    initialData.Obj.Nodes.forEach((node: any) => {
        IDToData[node.ID] = node;
    });
    recursivelyBuildHierarchy(node, nodesInfo, IDToData);
    // @todo, this part should be fixed, it's a workaround to fix a
    // problem with endless loop and circular structure in javascript
    nodesInfo["Tophierarchy"].children.forEach((node: any) => {
        fixProblemWithRecursionInData(node, node.children, [node.ID]);
    });

    return nodesInfo["Tophierarchy"];
}

function fixProblemWithRecursionInData(parentNode: any, children: any, nodesInHierarchy: any) {
    children.forEach((child: any) => {
        child.children = child.children.filter((child1: any) => {
            return nodesInHierarchy.indexOf(child1.ID) === -1;
        });
        child.childrenIds = new Set([...child.childrenIds].filter((child1) => {
            return nodesInHierarchy.indexOf(child1) === -1;
        }));
        nodesInHierarchy.push(child.ID);

        fixProblemWithRecursionInData(child, child.children, nodesInHierarchy);
    });
}
