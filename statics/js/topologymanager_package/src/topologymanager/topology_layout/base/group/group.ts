import { Node, NodeRegistry } from '../node/index';
import { Edge, EdgeRegistry } from '../edge/index';
import GroupRegistry from './registry';

export default class Group {
    ID: number;
    owner: Node;
    Type: string;
    parent: Group;
    members: NodeRegistry = new NodeRegistry();
    children: GroupRegistry = new GroupRegistry();
    static currentGroupId = 1;
    level: number = 1;
    depth: number = 1;
    collapsed: boolean = true;
    d: string = "";
    // collapseLinks: EdgeRegistry = new EdgeRegistry();

    static createFromData(owner: Node, Type: string): Group {
        const group = new Group();
        group.owner = owner;
        group.Type = Type;
        group.ID = Group.currentGroupId;
        ++Group.currentGroupId;
        return group;
    }

    setParent(parent: Group) {
        this.parent = parent;
    }

    addMember(node: Node) {
        this.members.addNode(node);
    }

    delMember(node: Node) {
        this.members.removeNodeByID(node.id);
    }

    isEqualTo(group: Group) {
        return this.ID === group.ID;
    }

    d3_id() {
        return this.ID;
    }

    collapse() {
        this.collapsed = true;
    }

    uncollapse() {
        this.collapsed = false;
    }

    hasOutsideLink() {
        return !!this.members.nodes.some((n: Node) => {
            const edges = n.edges;
            return edges.edges.some((e: Edge) => {
                if (e.Metadata.RelationType !== "ownership" && !e.source.group.isEqualTo(e.target.group)) return true;
            });
        })
    }

}
