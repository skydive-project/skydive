import * as events from 'events';
import { TopologyLayoutI } from "../../index";
import { NodeRegistry } from '../node/index';
import LayoutConfig from '../../config';
import LayoutContext from './layout_context';
import { Node } from '../node/index';

export interface LayoutUII {
    createRoot(): void;
    selector: string;
    width: number;
    height: number;
    start(): void;
    simulation: any;
    svg: any;
    g: any;
    restartsimulation(): void;
    zoomIn(): void;
    zoomOut(): void;
    zoomFit(): void;
    zoomed(): void;
    alphaTarget(): void;
    alphaTargetRestart(): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
}

export interface LayoutUIConstructableI {
    new(selector: string): LayoutUII;
}


export class LayoutUI implements LayoutUII {
    selector: string;
    width: number;
    height: number;
    svg: any;
    g: any;
    simulation: any;
    zoom: any;
    layoutContext: LayoutContext;
    constructor(selector: string) {
        this.selector = selector;
    }
    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext;
    }
    createRoot() {
        this.width = $(this.selector).width() - 20;
        this.height = $(window).height() - (window.$(this.selector).offset() && window.$(this.selector).offset().top || 0);
        this.zoom = window.d3.zoom()
            .on("zoom", this.zoomed.bind(this));
        this.svg = window.d3.select(this.selector).append("svg");
        this.svg
            .attr("width", this.width)
            .attr("height", this.height).call(this.zoom).on("dblclick.zoom", null);
        const defsMarker = (type: string, target: string, point: string) => {
            let id = "arrowhead-" + type + "-" + target + "-" + point;

            let refX = 1.65
            let refY = 0.15
            let pathD = "M0,0 L0,0.3 L0.5,0.15 Z"
            if (type === "egress" || point === "end") {
                pathD = "M0.5,0 L0.5,0.3 L0,0.15 Z"
            }

            if (target === "deny") {
                refX = 1.85
                refY = 0.3
                const a = "M0.1,0 L0.6,0.5 L0.5,0.6 L0,0.1 Z"
                const b = "M0,0.5 L0.1,0.6 L0.6,0.1 L0.5,0 Z"
                pathD = a + " " + b
            }

            let color = "rgb(0, 128, 0, 0.8)"
            if (target === "deny") {
                color = "rgba(255, 0, 0, 0.8)";
            }

            this.svg.append("defs").append("marker")
                .attr("id", id)
                .attr("refX", refX)
                .attr("refY", refY)
                .attr("markerWidth", 1)
                .attr("markerHeight", 1)
                .attr("orient", "auto")
                .attr("markerUnits", "strokeWidth")
                .append("path")
                .attr("fill", color)
                .attr("d", pathD);
        }

        defsMarker("ingress", "deny", "begin");
        defsMarker("ingress", "deny", "end");
        defsMarker("ingress", "allow", "begin");
        defsMarker("ingress", "allow", "end");
        defsMarker("egress", "deny", "begin");
        defsMarker("egress", "deny", "end");
        defsMarker("egress", "allow", "begin");
        defsMarker("egress", "allow", "end");
        this.g = this.svg.append("g");
    }
    start() {
        const visibleNodes = this.layoutContext.dataManager.nodeManager.getVisibleNodes(this.layoutContext.collapseLevel, this.layoutContext.isAutoExpand());
        this.layoutContext.subscribeToEvent('ui.simulation.alphatarget', this.alphaTarget.bind(this));
        this.layoutContext.subscribeToEvent('ui.simulation.alphatarget.restart', this.alphaTargetRestart.bind(this));
        this.simulation = window.d3.forceSimulation([])
            .force("charge", window.d3.forceManyBody().strength(-500))
            .force("link", window.d3.forceLink([]).distance((e: any) => {
                return this.layoutContext.config.getValue('link.distance', e);
            }).strength(0.9).iterations(2))
            .force("collide", window.d3.forceCollide().radius(80).strength(0.1).iterations(1))
            .force("center", window.d3.forceCenter(this.width / 2, this.height / 2))
            .force("x", window.d3.forceX(0).strength(0.01))
            .force("y", window.d3.forceY(0).strength(0.01))
            .alphaDecay(0.0090);

        this.simulation.on("tick", (...args: Array<any>) => {
            this.layoutContext.e.emit('ui.tick', ...args);
        });
    }

    zoomIn() {
        this.svg.transition().duration(500).call(this.zoom.scaleBy, 1.1);
    }

    zoomOut() {
        this.svg.transition().duration(500).call(this.zoom.scaleBy, 0.9);
    }

    zoomFit() {
        var bounds = this.g.node().getBBox();
        var parent = this.g.node().parentElement;
        var fullWidth = parent.clientWidth, fullHeight = parent.clientHeight;
        var width = bounds.width, height = bounds.height;
        var midX = bounds.x + width / 2, midY = bounds.y + height / 2;
        if (width === 0 || height === 0) return;
        var scale = 0.75 / Math.max(width / fullWidth, height / fullHeight);
        var translate = [fullWidth / 2 - midX * scale, fullHeight / 2 - midY * scale];

        var t = window.d3.zoomIdentity
            .translate(translate[0] + 30, translate[1])
            .scale(scale);
        this.svg.transition().duration(500).call(this.zoom.transform, t);
    }

    zoomed() {
        this.g.attr("transform", window.d3.event.transform);
    }

    restartsimulation() {
        const visibleNodes = this.layoutContext.dataManager.nodeManager.getVisibleNodes(this.layoutContext.collapseLevel, this.layoutContext.isAutoExpand());
        // console.log(visibleNodes.map((n: Node) => n.ID));
        this.simulation.nodes(visibleNodes);
        this.simulation.force("link").links(this.layoutContext.dataManager.edgeManager.getVisibleEdges(visibleNodes));
        this.simulation.alpha(1).restart();
    }

    alphaTarget() {
        this.simulation.alphaTarget(0);
    }

    alphaTargetRestart() {
        this.simulation.alphaTarget(0.05).restart();
    }

}
