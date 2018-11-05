import { Node } from '../node/index';
import Group from './group';


function fixDepthAndLevelForGroup(g: Group, level: number = 0) {
    const group = g;
    while (g) {
        if (level > g.depth) g.depth = level;
        level++;

        g = g.parent;
    }
    group.level = level;
}


export default class GroupRegistry {
    groups: Array<Group> = [];

    addGroupFromData(owner: Node, Type: string): Group {
        const g = Group.createFromData(owner, Type);
        this.groups.push(g);
        return g;
    }

    getGroupByOwner(owner: Node): Group {
        return this.groups.find((g: Group) => g.owner.equalsTo(owner));
    }

    getGroupByOwnerId(ownerId: string): Group {
        return this.groups.find((g: Group) => g.owner.ID == ownerId);
    }

    removeById(ID: number): void {
        this.groups = this.groups.filter((g: Group) => g.ID == ID);
    }

    addGroup(group: Group) {
        this.groups.push(group);
    }

    updateLevelAndDepth(collapseLevel: number = 0, isAutoExpand: boolean = false) {
        this.groups.forEach((g: Group) => {
            fixDepthAndLevelForGroup(g);
        });
        this.groups.forEach((g: Group) => {
            if (g.level > collapseLevel && isAutoExpand === false) {
                return;
            }
            if (isAutoExpand) {
                g.collapsed = !isAutoExpand;
                return;
            }
            if (collapseLevel >= g.level) {
                g.collapsed = false;
                return;
            }
        });
        this.groups.sort(function(a, b) { return a.level - b.level; });
    }

    getGroupsWithNoParent(): Array<Group> {
        return this.groups.filter((g: Group) => {
            return !!g.parent;
        });
    }

    get size() {
        return this.groups.length;
    }

    removeOldData() {
        this.groups = [];
    }

    getVisibleGroups(visibilityLevel: number, autoExpand: boolean): Array<Group> {
        return this.groups.filter((group: Group) => {
            if (autoExpand) {
                return true;
            }
            if (group.level > visibilityLevel) {
                if (group.level === visibilityLevel + 1) {
                    return true;
                }
                if (!group.collapsed) {
                    return true;
                }
                return false;
            }
            return true;
        });
    }

}
