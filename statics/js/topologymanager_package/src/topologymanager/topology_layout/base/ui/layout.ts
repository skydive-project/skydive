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
        this.svg
            .attr("width", this.width)
            .attr("height", this.height);
        this.g = this.svg.append("g");
    }
    start() {
    }

    restartsimulation() {
    }

}
