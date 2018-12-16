import * as events from 'events';
import { TopologyLayoutI } from "../../index";
import LayoutConfig from '../../../config';
import LayoutContext from './layout_context';

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
    layoutContext: LayoutContext;
    zoom: any;

    constructor(selector: string) {
        this.selector = selector;
    }

    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext;
    }

    createRoot() {
        this.width = $(this.selector).width() - 20;
        this.height = $(window).height() - (window.$(this.selector).offset() && window.$(this.selector).offset().top || 0);
        this.svg = window.d3.select(this.selector).append("svg");
        this.zoom = window.d3.zoom()
            .on("zoom", this.zoomed.bind(this));
        this.svg
            .attr("width", this.width)
            .attr("height", this.height)
            .call(this.zoom);
        this.simulation = window.d3.forceSimulation();
        this.simulation.nodes([]);
        this.simulation.on("tick", () => {
            this.layoutContext.e.emit('ui.tick');
        });
        const filter = this.svg.append("filter").attr("x", 0).attr("y", 0)
            .attr("width", 1).attr("height", 1).attr("id", "selectedNodeFilter");
        filter.append("feFlood").attr("flood-color", "#ccc");
        filter.append("feComposite").attr("in", "SourceGraphic");
        this.g = this.svg.append("g").attr("transform", function() { return "translate(0,33)" });;
    }

    start() {
    }

    restartsimulation() {
    }

    zoomed() {
        this.g.attr("transform", window.d3.event.transform);
    }

    zoomIn() {
        this.svg.transition().duration(500).call(this.zoom.scaleBy, 1.1);
    }

    zoomOut() {
        this.svg.transition().duration(500).call(this.zoom.scaleBy, 0.9);
    }

}
