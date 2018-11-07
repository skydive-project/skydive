import { EdgeRegistry } from '../edge/index';
import { Group } from '../group/index';
import { Edge } from '../edge/index';

export default class Node {
    ID: string;
    Name: string;
    Host: string;
    Metadata: any;
    selected: boolean = false;
    x: number;
    y: number;
    fx: number = null;
    fy: number = null;
    group: Group = null;
    emphasized: boolean = false;
    highlighted: boolean = false;
    fixed: boolean = false;
    edges: EdgeRegistry = new EdgeRegistry();
    collapsed: boolean = true;
    _d3_id: any;
    static createFromData(ID: string, Name: string, Host: string, Metadata: any): Node {
        const node = new Node();
        node.ID = ID;
        node.Name = Name;
        node.Host = Host;
        node.Metadata = Metadata;
        return node;
    }

    get id() {
        return this.ID;
    }

    equalsTo(d: Node): boolean {
        return d.ID == this.ID;
    }

    hasType(Type: string) {
        return this.Metadata.Type === Type;
    }

    d3_id(): any {
        return this.id;
    }

    isGroupOwner(group?: Group, Type?: string): boolean {
        return this.group && this.group.owner.equalsTo(this) && (!Type || Type === this.group.Type);
    }

    clone(): Node {
        return Node.createFromData(this.ID, this.Name, this.Host, this.Metadata);
    }

    isCaptureOn(): boolean {
        return "Capture/id" in this.Metadata;
    }

    isCaptureAllowed() {
        const allowedTypes = ["device", "veth", "ovsbridge", "geneve", "vlan", "bond", "ovsport",
            "internal", "tun", "bridge", "vxlan", "gre", "gretap", "dpdkport"];
        return allowedTypes.indexOf(this.Metadata.Type) >= 0;
    }

    getD3XCoord() {
        return this.x;
    }

    getD3YCoord() {
        return this.y;
    }

    onTheScreen(): boolean {
        return !!(this.x && this.y);
    }

    toggleExpandAllGroups() {
        this.collapsed = !this.collapsed;
        if (this.collapsed) {
            this.group.collapse();
        } else {
            this.group.uncollapse();
        }
    }
    getNeighborWithType(Type: string): Node {
        let nodeToReturn;
        this.edges.edges.some((e: Edge) => {
            if (e.source.equalsTo(this) && e.target.hasType(Type)) {
                nodeToReturn = e.source;
                return true;
            }
            if (e.target.equalsTo(this) && e.source.hasType(Type)) {
                nodeToReturn = e.target;
                return true;
            }
            return false;
        });
        return nodeToReturn;
    }

    getTargets(): Node[] {
        return this.edges.edges.map((e: Edge) => e.target);
    }


}
