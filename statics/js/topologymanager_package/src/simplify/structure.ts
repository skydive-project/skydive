import TransformationRegistry from './transformation';
import BaseSimplificationStrategy from './base';
import { transformSkydiveDataForSimplification } from "../helpers";

export default class StructureSimplificationStrategy extends BaseSimplificationStrategy {

    findDeadNodes() {
        let vmRelatedNodes: Set<String> = new Set([]);
        let nodes: Array<String> = [];
        const nodeTypesToCheck: Array<String> = ["libvirt"];
        const skipHosts: Set<String> = new Set([]);
        // find initial list of nodes, actually, its vm nodes
        Object.keys(this.data).forEach((nodeId: any) => {
            if (nodeTypesToCheck.indexOf(this.data[nodeId].Type) === -1) {
                return;
            }
            nodes.push(nodeId);
        });

        let i = 0;
        while (true) {
            const nodeId: any = nodes[i];
            const nodeData = this.data[nodeId];
            if (!nodeData) {
                break
            }
            let goThroughNodes: Array<String> = [];
            try {
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
                    goThroughNodes = goThroughNodes.concat([...nodeData.layers]);
                }
                // handle list of children of specific node
                if (nodeData.children.size) {
                    goThroughNodes = goThroughNodes.concat([...nodeData.children]);
                }
                // handle list of parent of specific node
                if (nodeData.parent.size) {
                    goThroughNodes = goThroughNodes.concat([...nodeData.parent]);
                }
            } finally {
                goThroughNodes = goThroughNodes.filter((nodeId1: any) => {
                    return nodes.indexOf(nodeId1) === -1;
                });
                if (!goThroughNodes.length && (nodes.length - 1) == i) {
                    break;
                }
                const uniqueNodes = new Set(goThroughNodes);
                let uniqueNodes1 = [...uniqueNodes];
                nodes = nodes.concat(uniqueNodes1);
                ++i;
            }
        }
        return Object.keys(this.data).filter((nodeId: any) => {
            return nodes.indexOf(nodeId) === -1;
        });
    }

    simplifyInitialTopology(dataFormat: String, data: any, deadNodes: Array<String>) {
        const transformationHandler = TransformationRegistry.getTransformationWay(dataFormat);
        return transformationHandler(data, deadNodes);
    }

    simplifyData(dataFormat: String, data: any) {
        this.data = transformSkydiveDataForSimplification(data);
        const deadNodes = this.findDeadNodes();
        const newTopology = this.simplifyInitialTopology(dataFormat, data, deadNodes);
        return newTopology;
    }

}
