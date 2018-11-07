import * as events from 'events';
import LayoutConfig from '../../config';
import LayoutContext from './layout_context';
import { GroupRegistry, Group } from '../group/index';
import { Node } from '../node/index';

export interface GroupUII {
    createRoot(g: any): void;
    g: any;
    root: any;
    tick(): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
    update(): void;
    convexHull(g: Group): any;
    groupClass(d: Group): string;
}

export interface GroupUIConstructableI {
    new(): GroupUII;
}

function cross(a: any, b: any, c: any) {
    return (b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0]);
}

function computeUpperHullIndexes(points: any) {

    let i;
    let size = 2;
    const n = points.length, indexes = [0, 1];

    for (i = 2; i < n; ++i) {
        while (size > 1 && cross(points[indexes[size - 2]], points[indexes[size - 1]], points[i]) <= 0)--size;
        indexes[size++] = i;
    }

    return indexes.slice(0, size);
}

export class GroupUI implements GroupUII {
    g: any;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext
    }
    createRoot(g: any) {
        this.g = g.append("g").attr('class', 'groups').selectAll(".group");
    }
    get root() {
        return this.g;
    }
    tick() {
        this.root.attrs((d: Group) => {
            if (d.Type !== "ownership") return;

            var hull = this.convexHull(d);

            if (hull && hull.length) {
                return {
                    'd': hull ? "M" + hull.join("L") + "Z" : d.d,
                    'stroke-width': 64 + d.depth * 50,
                };
            } else {
                return { 'd': '' };
            }
        });
    }

    update() {
        this.g = this.g.data(this.layoutContext.dataManager.groupManager.getVisibleGroups(this.layoutContext.collapseLevel, this.layoutContext.isAutoExpand()), function(d: Group) { return d.d3_id(); });
        this.g.exit().remove();

        const groupEnter = this.g.enter()
            .append("path")
            .attr("class", this.groupClass)
            .attr("id", function(d: Group) { return "group-" + d.d3_id(); });

        this.g = groupEnter.merge(this.g).order();
    }

    groupClass(d: Group): string {
        var clazz = "group " + d.owner.Metadata.Type;

        if (d.owner.Metadata.Probe) clazz += " " + d.owner.Metadata.Probe;

        return clazz;
    }

    convexHull(g: Group) {
        const members = g.getAllMembers();
        const memberIdToMember: any = members.reduce((accum: any, n: Node) => {
            accum[n.ID] = n;
            return accum;
        }, {});
        let n = Object.keys(memberIdToMember).length;
        if (n < 1) return null;

        if (n == 1) {
            return members[0].x && members[0].y ? [[members[0].x, members[0].y], [members[0].x + 1, members[0].y + 1]] : null;
        }

        let i;
        let node: Node;
        const sortedPoints = [], flippedPoints = [];
        const memberIds = Object.keys(memberIdToMember);
        for (i = 0; i < n; ++i) {
            node = memberIdToMember[memberIds[i]];
            if (node.getD3XCoord() && node.getD3YCoord()) sortedPoints.push([node.getD3XCoord(), node.getD3YCoord(), sortedPoints.length]);
        }
        n = sortedPoints.length;
        if (n < 1) {
            return null;
        }
        if (n === 1) {
            return [[sortedPoints[0][0], sortedPoints[0][1]], [sortedPoints[0][0] + 1, sortedPoints[0][1] + 1]];
        }
        sortedPoints.sort(function(a, b) {
            return a[0] - b[0] || a[1] - b[1];
        });
        for (i = 0; i < sortedPoints.length; ++i) {
            flippedPoints[i] = [sortedPoints[i][0], -sortedPoints[i][1]];
        }

        const upperIndexes = computeUpperHullIndexes(sortedPoints),
            lowerIndexes = computeUpperHullIndexes(flippedPoints);

        const skipLeft = lowerIndexes[0] === upperIndexes[0],
            skipRight: any = lowerIndexes[lowerIndexes.length - 1] === upperIndexes[upperIndexes.length - 1],
            hull = [];

        for (i = upperIndexes.length - 1; i >= 0; --i) {
            const coords = sortedPoints[upperIndexes[i]];
            hull.push([coords[0], coords[1]]);
        }
        for (i = +skipLeft; i < lowerIndexes.length - skipRight; ++i) {
            const coords = sortedPoints[lowerIndexes[i]];
            hull.push([coords[0], coords[1]]);
        }
        return hull;
    }

}
