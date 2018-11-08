import * as events from 'events';
import LayoutConfig from '../../config';
import { NodeRegistry } from '../node/index';
import { pinIndicatorImg, captureIndicatorImg, minusImg, plusImg, nodeImgMap, managerImgMap } from './helpers/index';
import { Node } from '../node/index';
import LayoutContext from './layout_context';

export interface NodeUII {
    createRoot(g: any): void;
    g: any;
    root: any;
    rootParent: any;
    tick(): void;
    update(): void;
    unselectNode(d: Node): void;
    onNodeShiftClick(d: Node): void;
    pinNode(d: Node): void;
    unpinNode(d: Node): void;
    collapseImg(d: Node): string;
    highlightNodeID(d: Node): void;
    unhighlightNodeID(d: Node): void;
    emphasizeNodeID(d: Node): void;
    deemphasizeNodeID(d: Node): void;
    deemphasizeNode(d: Node): void
    emphasizeNode(d: Node): void;
    managerImg(d: Node): string;
    unselectNode(d: Node): void;
    collapseByNode(d: Node): void;
    nodeSize(d: Node): number;
    nodeImg(d: Node): string;
    nodeTitle(d: Node): string;
    onNodeDragEnd(d: Node): void;
    nodeClass(d: Node): string;
    onNodeClick(d: Node): void;
    selectNode(d: Node): void;
    stateSet(d: Node): void;
    managerSet(d: Node): void;
    captureStarted(d: Node): void;
    captureStopped(d: Node): void;
    groupOwnerSet(d: Node): void;
    groupOwnerUnset(d: Node): void;
    onNodeDragStart(d: Node): void;
    onNodeDrag(d: Node): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
    collapseGroupLink(d: Node): void;
}

export interface NodeUIConstructableI {
    new(): NodeUII;
}


export class NodeUI implements NodeUII {
    g: any;
    rootParent: any;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext;
    }
    createRoot(g: any) {
        this.g = g.append("g").attr('class', 'nodes').selectAll(".node");
        this.rootParent = g;
    }
    get root() {
        return this.g;
    }
    tick() {
        this.root.attr("transform", function(d: Node) { return "translate(" + d.x + "," + d.y + ")"; });
    }
    update() {
        const nodes = this.layoutContext.dataManager.nodeManager.getVisibleNodes(this.layoutContext.collapseLevel, this.layoutContext.isAutoExpand());
        this.g = this.root.data(nodes, function(d: Node) { return d.d3_id(); });
        this.root.exit().remove();

        var nodeEnter = this.root.enter()
            .append("g")
            .attr("class", this.nodeClass)
            .attr("id", function(d: Node) { return "node-" + d.d3_id(); })
            .attr('collapsed', function(d: Node) {
                if (d.isGroupOwner()) {
                    return d.group.collapsed;
                }
                return null;
            })
            .on("click", this.onNodeClick.bind(this))
            .on("dblclick", this.collapseByNode.bind(this))
            .call(window.d3.drag()
                .on("start", this.onNodeDragStart.bind(this))
                .on("drag", this.onNodeDrag.bind(this))
                .on("end", this.onNodeDragEnd.bind(this)));

        nodeEnter.append("circle")
            .attr("r", this.nodeSize);

        // node picto
        nodeEnter.append("image")
            .attr("id", function(d: Node) { return "node-img-" + d.d3_id(); })
            .attr("class", "picto")
            .attr("x", -12)
            .attr("y", -12)
            .attr("width", "24")
            .attr("height", "24")
            .attr("xlink:href", this.nodeImg);

        // node rectangle
        nodeEnter.append("rect")
            .attr("class", "node-text-rect")
            .attr("width", (d: Node) => { return this.nodeTitle(d).length * 10 + 10; })
            .attr("height", 25)
            .attr("x", (d: Node) => {
                return this.nodeSize(d) * 1.6 - 5;
            })
            .attr("y", -8)
            .attr("rx", 4)
            .attr("ry", 4);

        // node title
        nodeEnter.append("text")
            .attr("dx", (d: Node) => {
                return this.nodeSize(d) * 1.6;
            })
            .attr("dy", 10)
            .text(this.nodeTitle);

        nodeEnter.filter(function(d: Node) { return d.isGroupOwner(); })
            .each(this.groupOwnerSet.bind(this));

        nodeEnter.filter(function(d: Node) { return d.Metadata.Capture; })
            .each(this.captureStarted.bind(this));

        nodeEnter.filter(function(d: Node) { return d.Metadata.Manager; })
            .each(this.managerSet.bind(this));

        nodeEnter.filter(function(d: Node) { return d.emphasized; })
            .each(this.emphasizeNode.bind(this));

        this.g = nodeEnter.merge(this.root);
    }

    stateSet(d: Node) {
        this.rootParent.select("#node-" + d.d3_id()).attr("class", this.nodeClass);
    }


    managerSet(d: Node) {
        var size = this.nodeSize(d);
        var node = this.rootParent.select("#node-" + d.d3_id());

        node.append("circle")
            .attr("class", "manager")
            .attr("r", 12)
            .attr("cx", size - 2)
            .attr("cy", size - 2);

        node.append("image")
            .attr("class", "manager")
            .attr("x", size - 12)
            .attr("y", size - 12)
            .attr("width", 20)
            .attr("height", 20)
            .attr("xlink:href", this.managerImg(d));
    }


    captureStarted(d: Node) {
        var size = this.nodeSize(d);
        this.rootParent.select("#node-" + d.d3_id()).append("image")
            .attr("class", "capture")
            .attr("x", -size - 8)
            .attr("y", size - 8)
            .attr("width", 16)
            .attr("height", 16)
            .attr("xlink:href", captureIndicatorImg);
    }

    captureStopped(d: Node) {
        this.rootParent.select("#node-" + d.d3_id()).select('image.capture').remove();
    }

    collapseGroupLink(d: Node) {
        this.rootParent.select("#node-" + d.d3_id())
            .attr('collapsed', d.group.collapsed)
            .select('image.collapsexpand')
            .attr('xlink:href', this.collapseImg);
    }

    groupOwnerSet(d: Node) {
        var self = this;

        var o = this.rootParent.select("#node-" + d.d3_id());

        o.append("image")
            .attr("class", "collapsexpand")
            .attr("width", 16)
            .attr("height", 16)
            .attr("x", (d: Node) => { return -this.nodeSize(d) - 4; })
            .attr("y", (d: Node) => { return -this.nodeSize(d) - 4; })
            .attr("xlink:href", this.collapseImg);
        o.select('circle').attr("r", this.nodeSize);
    }

    groupOwnerUnset(d: Node) {
        var o = this.rootParent.select("#node-" + d.d3_id());
        o.select('image.collapsexpand').remove();
        o.select('circle').attr("r", this.nodeSize);
    }

    onNodeDragStart(d: Node) {
        if (!window.d3.event.active) {
            this.layoutContext.e.emit('ui.simulation.alphatarget.restart');
        }

        if (window.d3.event.sourceEvent.shiftKey && d.isGroupOwner()) {
            var i, members = d.group.members.nodes;
            for (i = members.length - 1; i >= 0; i--) {
                members[i].fx = members[i].x;
                members[i].fy = members[i].y;
            }
        } else {
            d.fx = d.x;
            d.fy = d.y;
        }
    }

    onNodeDrag(d: Node) {
        var dx = window.d3.event.x - d.fx, dy = window.d3.event.y - d.fy;

        if (window.d3.event.sourceEvent.shiftKey && d.isGroupOwner()) {
            var i, members = d.group.members.nodes;
            for (i = members.length - 1; i >= 0; i--) {
                members[i].fx += dx;
                members[i].fy += dy;
            }
        } else {
            d.fx += dx;
            d.fy += dy;
        }
    }

    onNodeDragEnd(d: Node) {
        if (!window.d3.event.active) {
            this.layoutContext.e.emit('ui.simulation.alphatarget');
        }

        if (d.isGroupOwner()) {
            var i, members = d.group.members.nodes;
            for (i = members.length - 1; i >= 0; i--) {
                if (!members[i].fixed) {
                    members[i].fx = null;
                    members[i].fy = null;
                }
            }
        } else {
            if (!d.fixed) {
                d.fx = null;
                d.fy = null;
            }
        }
    }

    nodeClass(d: Node) {
        var clazz = "node " + d.Metadata.Type;

        if (d.Metadata.Probe) clazz += " " + d.Metadata.Probe;
        if (d.Metadata.State == "DOWN") clazz += " down";
        if (d.highlighted) clazz += " highlighted";
        if (d.selected) clazz += " selected";

        return clazz;
    }

    onNodeClick(d: Node) {
        if (window.d3.event.shiftKey) return this.onNodeShiftClick(d);
        if (window.d3.event.altKey) return this.collapseByNode(d);

        if (d.selected) return;

        this.layoutContext.e.emit('node.select', d);
        this.selectNode(d);

    }

    selectNode(d: Node) {
        var circle = this.rootParent.select("#node-" + d.d3_id())
            .classed('selected', true)
            .select('circle');
        circle.transition().duration(500).attr('r', +circle.attr('r') + 3);
        d.selected = true;
    }

    unselectNode(d: Node) {
        var circle = this.rootParent.select("#node-" + d.d3_id())
            .classed('selected', false)
            .select('circle');
        if (!circle) return;
        circle.transition().duration(500).attr('r', circle ? +circle.attr('r') - 3 : 0);
        d.selected = false;
    }

    collapseByNode(d: Node) {
        if (d.Metadata.Type === "host") {
            if (d.group.collapsed)
                this.layoutContext.e.emit('host.uncollapse', d);
            else
                this.layoutContext.e.emit('host.collapse', d);
        } else {
            if (d.isGroupOwner()) {
                if (d.group) {
                    this.layoutContext.e.emit('ui.group.collapse', d.group)
                }
            }
            this.layoutContext.e.emit('ui.update');
        }
    }

    nodeSize(d: Node) {
        var size;
        switch (d.Metadata.Type) {
            case "host": size = 30; break;
            case "netns": size = 26; break;
            case "port":
            case "ovsport": size = 22; break;
            case "switch":
            case "ovsbridge": size = 24; break;
            default:
                size = d.isGroupOwner() ? 26 : 20;
        }
        if (d.selected) size += 3;

        return size;
    }

    nodeImg(d: Node) {
        var t = d.Metadata.Type || "default";
        return (t in nodeImgMap) ? nodeImgMap[t] : nodeImgMap["default"];
    }

    nodeTitle(d: Node) {
        if (d.Metadata.Type === "host") {
            return d.Metadata.Name.split(".")[0];
        }
        return d.Metadata.Name ? d.Metadata.Name.length > 12 ? d.Metadata.Name.substr(0, 12) + "..." : d.Metadata.Name : "";
    }

    emphasizeNodeID(d: Node) {
        d.emphasized = true;
        const id = d.d3_id();
        if (!this.rootParent.select("#node-emphasize-" + id).empty()) return;

        var circle;
        if (this.rootParent.select("#node-highlight-" + id).empty()) {
            circle = this.rootParent.select("#node-" + id).insert("circle", ":first-child");
        } else {
            circle = this.rootParent.select("#node-" + id).insert("circle", ":nth-child(2)");
        }

        circle.attr("id", "node-emphasize-" + id)
            .attr("class", "emphasized")
            .attr("r", (d: Node) => { return this.nodeSize(d) + 8; });
    }

    deemphasizeNodeID(d: Node) {
        d.emphasized = false;
        this.rootParent.select("#node-emphasize-" + d.d3_id()).remove();
    }

    deemphasizeNode(d: Node) {
        this.deemphasizeNodeID(d);
    }

    emphasizeNode(d: Node) {
        this.emphasizeNodeID(d);
    }

    managerImg(d: Node) {
        var t = d.Metadata.Orchestrator || d.Metadata.Manager || "default";
        return (t in managerImgMap) ? managerImgMap[t] : managerImgMap["default"];
    }

    collapseImg(d: Node) {
        if (d.group && d.group.collapsed) return plusImg;
        return minusImg;
    }

    highlightNodeID(d: Node) {
        d.highlighted = true;
        const id = d.d3_id();
        if (!this.rootParent.select("#node-highlight-" + id).empty()) return;

        this.rootParent.select("#node-" + id)
            .insert("circle", ":first-child")
            .attr("id", "node-highlight-" + id)
            .attr("class", "highlighted")
            .attr("r", (d: Node) => { return this.nodeSize(d) + 16; });
    }

    unhighlightNodeID(d: Node) {
        const id = d.d3_id();
        d.highlighted = false;
        this.rootParent.select("#node-highlight-" + id).remove();
    }

    pinNode(d: Node) {
        const id = d.d3_id();
        var size = this.nodeSize(d);
        this.rootParent.select("#node-" + id).append("image")
            .attr("class", "pin")
            .attr("x", size - 12)
            .attr("y", -size - 4)
            .attr("width", 16)
            .attr("height", 16)
            .attr("xlink:href", pinIndicatorImg);
        d.fixed = true;
        d.fx = d.x;
        d.fy = d.y;
    }

    unpinNode(d: Node) {
        const id = d.d3_id();
        this.rootParent.select("#node-" + id).select('image.pin').remove();
        d.fixed = false;
        d.fx = null;
        d.fy = null;
    }

    onNodeShiftClick(d: Node) {
        if (!d.fixed) {
            this.pinNode(d);
        } else {
            this.unpinNode(d);
        }
    }

}
