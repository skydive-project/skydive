import Node from './node';
import { Group } from '../group/index';

export default class NodeRegistry {
    nodes: Array<Node> = [];

    addNodeFromData(ID: string, Name: string, Host: string, Metadata: any) {
        this.nodes.push(Node.createFromData(ID, Name, Host, Metadata));
    }

    getActive(): Node {
        return this.nodes.find((n: Node) => n.selected);
    }

    getNodeById(ID: string): Node {
        return this.nodes.find((n: Node) => n.ID === ID);
    }

    get size() {
        return this.nodes.length;
    }

    removeNodeByID(nodeID: string) {
        this.nodes = this.nodes.filter((n: Node) => n.ID !== nodeID);
    }

    removeNodeByHost(nodeHost: string) {
        this.nodes = this.nodes.filter((n: Node) => n.Host !== nodeHost);
    }

    isThereAnyNodeWithType(Type: string): boolean {
        return !!this.nodes.some((n: Node) => n.Metadata.Type === Type);
    }

    addNode(node: Node) {
        this.nodes.push(node);
    }

    getVisibleNodes(visibilityLevel: number = 1, autoExpand: boolean = false): Array<Node> {
        const nodes: Array<Node> = this.nodes.filter((node: Node) => {
            // if (node.Name === 'tapbbbf73d3-6a') {
            //     console.log(visibilityLevel, autoExpand, node.group, node.isGroupOwner(), node.visible);
            // }
            if (autoExpand) {
                return true;
            }
            if (node.group && node.group.level > visibilityLevel) {
                if (node.isGroupOwner() && node.group.level === visibilityLevel + 1) {
                    return true;
                }
                if (!node.group.collapsed) {
                    return true;
                }
                return node.visible;
            }
            if (node.isGroupOwner()) {
                return true;
            }
            if (node.group && !node.group.collapsed) {
                return true;
            }
            if (!node.group) {
                return true;
            }
            if (!node.group.collapsed) {
                return true;
            }
            return node.visible;
        });
        nodes.forEach((n: Node) => n.visible = true);
        return nodes;
    }

    removeOldData() {
        this.nodes = [];
    }

    removeEdgeByID(ID: string) {
        this.nodes.forEach((n: Node) => {
            n.edges.removeEdgeByID(ID);
        });
    }

    groupRemoved(g: Group) {
        this.nodes.forEach((n: Node) => {
            if (n.group && n.group.isEqualTo(g)) {
                n.group = null;
            }
        });
    }

    clone(): NodeRegistry {
        const registry = new NodeRegistry();
        this.nodes.forEach((n: Node) => registry.addNode(n));
        return registry;
    }
}
