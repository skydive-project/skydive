import { Node } from '../node/index';

export default class Edge {
    ID: string;
    Host: string;
    Metadata: any;
    source: Node;
    target: Node;
    selected: boolean = false;
    latencyTimestamp: number;
    latency: number;
    bandwidth: number;
    bandwidthAbsolute: number;
    bandwidthBaseline: number;

    static createFromData(ID: string, Host: string, Metadata: any, source: Node, target: Node): Edge {
        const edge = new Edge();
        edge.ID = ID;
        edge.Host = Host;
        edge.source = source;
        edge.target = target;
        edge.Metadata = Metadata;
        return edge;
    }

    get id() {
        return this.ID;
    }

    hasRelationType(relationType: string) {
        return this.Metadata.RelationType === relationType;
    }

    hasType(Type: string) {
        return this.Metadata.Type === Type;
    }

    d3_id(): any {
        return this.ID;
    }

    equalsTo(compareTo: Edge): boolean {
        return compareTo.ID === this.ID;
    }
}
