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

    delMemberByID(nodeID: string) {
        this.members.removeNodeByID(nodeID);
    }

    isEqualTo(group: Group) {
        return this.ID === group.ID;
    }

    d3_id() {
        return this.ID;
    }

    collapse(collapseChildren: boolean = true) {
        this.collapsed = true;
        if (collapseChildren) {
            this.children.groups.forEach((g: Group) => g.collapse(collapseChildren));
        }
    }

    uncollapse(uncollapseChildren: boolean = true) {
        this.collapsed = false;
        if (uncollapseChildren) {
            this.children.groups.forEach((g: Group) => g.uncollapse(uncollapseChildren));
        }
    }

    hasOutsideLink() {
        return !!this.members.nodes.some((n: Node) => {
            const edges = n.edges;
            return edges.edges.some((e: Edge) => {
                if (e.Metadata.RelationType !== "ownership" && !e.source.group.isEqualTo(e.target.group)) return true;
            });
        })
    }

    getAllMembers(): Node[] {
        let members = this.members.nodes;
        this.children.groups.forEach((children: Group) => {
            members = members.concat(children.getAllMembers())
        });
        return members;
    }

}
