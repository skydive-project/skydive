import { Node } from '../node/index';
import Edge from './edge';

export default class EdgeRegistry {
    edges: Array<Edge> = [];

    addEdgeFromData(ID: string, Host: string, Metadata: any, source: Node, target: Node) {
        this.edges.push(Edge.createFromData(ID, Host, Metadata, source, target));
    }

    getEdgeById(ID: string): Edge {
        return this.edges.find((e: Edge) => e.ID === ID);
    }

    get size() {
        return this.edges.length;
    }

    removeEdgeByID(ID: string) {
        this.edges = this.edges.filter((e: Edge) => e.ID !== ID);
    }

    removeEdgeByHost(host: string) {
        this.edges = this.edges.filter((e: Edge) => e.Host !== host);
    }

    getEdgesWithRelationType(relationType: string): Array<Edge> {
        return this.edges.filter((e: Edge) => e.hasRelationType(relationType));
    }

    addEdge(e: Edge) {
        this.edges.push(e);
    }

    removeOldData() {
        this.edges = [];
    }

    getVisibleEdges(visibleNodes: Array<Node>): Array<Edge> {
        const visibleNodeIds = visibleNodes.reduce((accum: Array<string>, node: Node) => {
            accum.push(node.ID);
            return accum;
        }, []);
        return this.edges.filter((e: Edge) => {
            return visibleNodeIds.indexOf(e.source.ID) !== -1 && visibleNodeIds.indexOf(e.target.ID) !== -1;
        });
    }

    getActive(): Edge {
        return this.edges.find((n: Edge) => n.selected);
    }

}
