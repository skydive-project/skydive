import * as events from 'events';
import LayoutContext from './layout_context';
import { EdgeRegistry, Edge } from '../edge/index';
import { Node } from '../node/index';

export interface EdgeUII {
    createRoot(g: any): void;
    root: any;
    gLink: any;
    gLinkWrap: any;
    gLinkLabel: any;
    tick(): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
    update(): void;
    linkWraps(): any;
    delLinkLabel(e: Edge): void;
}

export interface EdgeUIConstructableI {
    new(): EdgeUII;
}

export class EdgeUI implements EdgeUII {
    gLink: any;
    gLinkWrap: any;
    gLinkLabel: any;
    root: any;
    previousVisibleEdgeIds: Array<string> = [];
    linkLabelData: any = {};
    layoutContext: LayoutContext;
    bandwidthIntervalID: any;
    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext
        this.bandwidthIntervalID = window.setInterval(this.updateLinkLabelHandler.bind(this), this.layoutContext.config.getValue('bandwidth').updatePeriod);
    }
    createRoot(g: any) {
        this.root = g;
        this.gLinkWrap = g.append("g").attr('class', 'link-wraps').selectAll(".link-wrap");
        this.gLink = g.append("g").attr('class', 'links').selectAll(".link");
        this.gLinkLabel = g.append("g").attr('class', 'link-labels').selectAll(".link-label");
    }
    tick() {
        this.gLink.attr("d", function(d: Edge) { if (d.source.x && d.target.x) return 'M ' + d.source.x + " " + d.source.y + " L " + d.target.x + " " + d.target.y; });
        this.gLinkLabel.attr("transform", function(d: any) {
            if (d.link.target.x < d.link.source.x) {
                var bbox = this.getBBox();
                var rx = bbox.x + bbox.width / 2;
                var ry = bbox.y + bbox.height / 2;
                return "rotate(180 " + rx + " " + ry + ")";
            }
            else {
                return "rotate(0)";
            }
        });

        this.gLinkWrap.attr('x1', function(d: any) { return d.link.source.x; })
            .attr('y1', function(d: any) { return d.link.source.y; })
            .attr('x2', function(d: any) { return d.link.target.x; })
            .attr('y2', function(d: any) { return d.link.target.y; });

    }

    update() {
        const visibleNodes = this.layoutContext.dataManager.nodeManager.getVisibleNodes(this.layoutContext.collapseLevel, this.layoutContext.isAutoExpand());
        const visibleEdges = this.layoutContext.dataManager.edgeManager.getVisibleEdges(visibleNodes);
        this.gLink = this.gLink.data(visibleEdges, function(d: Edge) { return d.d3_id(); });
        this.gLink.exit().remove();

        const linkEnter = this.gLink.enter()
            .append("path")
            .attr("id", function(d: Edge) { return "link-" + d.d3_id(); })
            .on("click", this.onEdgeClick.bind(this))
            .on("mouseover", this.highlightLink.bind(this))
            .on("mouseout", this.unhighlightLink.bind(this))
            .attr("class", this.linkClass);

        this.gLink = linkEnter.merge(this.gLink);
        this.gLinkWrap = this.gLinkWrap.data(this.linkWraps(), function(d: any) { return d.link.ID; });
        this.gLinkWrap.exit().remove();

        var linkWrapEnter = this.gLinkWrap.enter()
            .append("line")
            .attr("id", function(d: any) { return "link-wrap-" + d.link.d3_id(); })
            .on("click", (d: any) => { this.onEdgeClick(d.link); })
            .on("mouseover", (d: any) => { this.highlightLink(d.link); })
            .on("mouseout", (d: any) => { this.unhighlightLink(d.link); })
            .attr("class", this.linkWrapClass)
            .attr("marker-end", (d: any) => { return this.arrowhead(d.link); });

        this.gLinkWrap = linkWrapEnter.merge(this.gLinkWrap);
        const visibleEdgeIds = visibleEdges.map((e: Edge) => e.ID);
        // console.log('currently visible edges', visibleEdgeIds);
        // console.log('visibleNodes', visibleNodes.map((n: Node) => n.Name));
        // console.log('before visible edges', this.previousVisibleEdgeIds);
        this.layoutContext.dataManager.edgeManager.edges.forEach((e: Edge) => {
            if (visibleEdgeIds.indexOf(e.ID) !== -1) {
                return;
            }
            if (this.previousVisibleEdgeIds.indexOf(e.ID) === -1) {
                return;
            }
            // console.log('del Link', e.ID);
            this.delLink(e);
        });
        this.previousVisibleEdgeIds = visibleEdgeIds;
    }

    linkWraps() {
        const visibleNodes = this.layoutContext.dataManager.nodeManager.getVisibleNodes(this.layoutContext.collapseLevel, this.layoutContext.isAutoExpand());
        return this.layoutContext.dataManager.edgeManager.getVisibleEdges(visibleNodes).reduce((accum: any, e: Edge) => {
            if (!e.source.onTheScreen() || !e.target.onTheScreen()) {
                return accum;
            }
            accum.push({ link: e });
            return accum;
        }, []);
    }

    onEdgeClick(d: Edge) {
        this.layoutContext.e.emit('edge.select', d);
    }

    highlightLink(d: Edge) {
        const t = window.d3.transition()
            .duration(300)
            .ease(window.d3.easeLinear);
        this.root.select("#link-wrap-" + d.d3_id()).transition(t).style("stroke", "rgba(30, 30, 30, 0.15)");
        this.root.select("#link-" + d.d3_id()).transition(t).style("stroke-width", 2);
    }

    unhighlightLink(d: Edge) {
        const t = window.d3.transition()
            .duration(300)
            .ease(window.d3.easeLinear);
        this.root.select("#link-wrap-" + d.d3_id()).transition(t).style("stroke", null);
        this.root.select("#link-" + d.d3_id()).transition(t).style("stroke-width", null);
    }

    linkClass(d: Edge) {
        var clazz = "link real-edge " + d.Metadata.RelationType;

        if (d.Metadata.Type) clazz += " " + d.Metadata.Type;

        return clazz;
    }

    linkWrapClass(d: any) {
        var clazz = "link-wrap real-edge-wrap";

        return clazz;
    }

    arrowhead(link: Edge) {
        const none = "url(#arrowhead-none)";
        if (link.source.Metadata.Type !== "networkpolicy") {
            return none
        }
        if (link.target.Metadata.Type !== "pod") {
            return none
        }
        if (link.Metadata.RelationType !== "networkpolicy") {
            return none;
        }
        return "url(#arrowhead-" + link.Metadata.PolicyType + "-" + link.Metadata.PolicyTarget + "-" + link.Metadata.PolicyPoint + ")";
    }

    delLink(e: Edge) {
        const link = this.gLink.select("#link-" + e.d3_id());
        link.remove();
        const linkWrap = this.gLinkWrap.select("#link-wrap-" + e.d3_id());
        linkWrap.remove();
    }

    // @todo to be improved ? - move to another abstraction ?
    delLinkLabel(e: Edge) {

        if (!(e.ID in this.linkLabelData))
            return;
        delete this.linkLabelData[e.ID];

        this.bindLinkLabelData();
        this.gLinkLabel.exit().remove();

        // force a tick
        this.layoutContext.e.emit('ui.tick');
    }

    bindLinkLabelData() {
        this.gLinkLabel = this.gLinkLabel.data(Object.keys(this.linkLabelData).map(linkId => this.linkLabelData[linkId]), function(d: any) { return d.id; });
    }

    styleReturn(d: any, values: any) {
        if (d.active)
            return values[0];
        if (d.warning)
            return values[1];
        if (d.alert)
            return values[2];
        return values[3];
    }

    styleStrokeDasharray(d: any) {
        return this.styleReturn(d, ["20", "20", "20", ""]);
    }

    styleStrokeDashoffset(d: any) {
        return this.styleReturn(d, ["80 ", "80", "80", ""]);
    }

    styleAnimation(d: any) {
        var animate = function(speed: any) {
            return "dash " + speed + " linear forwards infinite";
        };
        return this.styleReturn(d, [animate("6s"), animate("3s"), animate("1s"), ""]);
    }

    styleStroke(d: any) {
        return this.styleReturn(d, ["YellowGreen", "Yellow", "Tomato", ""]);
    }

    updateLinkLabelHandler() {
        var self = this;

        this.updateLinkLabelData();
        this.bindLinkLabelData();

        // update links which don't have traffic
        var exit = this.gLinkLabel.exit();
        exit.each((d: any) => {
            this.gLink.select("#link-" + d.link.d3_id())
                .classed("link-label-active", false)
                .classed("link-label-warning", false)
                .classed("link-label-alert", false)
                .style("stroke-dasharray", "")
                .style("stroke-dashoffset", "")
                .style("animation", "")
                .style("stroke", "");
        });
        exit.remove();

        var enter = this.gLinkLabel.enter()
            .append('text')
            .attr("id", function(d: any) { return "link-label-" + d.link.d3_id(); })
            .attr("class", "link-label");
        enter.append('textPath')
            .attr("startOffset", "50%")
            .attr("xlink:href", function(d: any) { return "#link-" + d.link.d3_id(); });
        this.gLinkLabel = enter.merge(this.gLinkLabel);

        this.gLinkLabel.select('textPath')
            .classed("link-label-active", function(d: any) { return d.active; })
            .classed("link-label-warning", function(d: any) { return d.warning; })
            .classed("link-label-alert", function(d: any) { return d.alert; })
            .text(function(d: any) { return d.text; });

        this.gLinkLabel.each((d: any) => {
            this.gLink.select("#link-" + d.link.d3_id())
                .classed("link-label-active", d.active)
                .classed("link-label-warning", d.warning)
                .classed("link-label-alert", d.alert)
                .style("stroke-dasharray", this.styleStrokeDasharray(d))
                .style("stroke-dashoffset", this.styleStrokeDashoffset(d))
                .style("animation", this.styleAnimation(d))
                .style("stroke", this.styleStroke(d));
        });

        // force a tick
        this.layoutContext.e.emit('ui.tick');
    }

    updateLinkLabelData() {
        const driver = this.layoutContext.linkLabelStrategy;

        const visibleNodes = this.layoutContext.dataManager.nodeManager.getVisibleNodes(this.layoutContext.collapseLevel, this.layoutContext.isAutoExpand());
        const visibleEdges = this.layoutContext.dataManager.edgeManager.getVisibleEdges(visibleNodes);
        visibleEdges.forEach((e: Edge) => {
            if (e.Metadata.RelationType !== "layer2")
                return;
            driver.updateData(e);

            if (driver.hasData(e)) {
                this.linkLabelData[e.ID] = {
                    id: "link-label-" + e.ID,
                    text: driver.getText(e),
                    active: driver.isActive(e),
                    warning: driver.isWarning(e),
                    alert: driver.isAlert(e),
                };
            } else {
                delete this.linkLabelData[e.ID];
            }

        });
    }

}

