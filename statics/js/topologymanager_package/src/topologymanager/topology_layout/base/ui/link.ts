import * as events from 'events';
import LayoutContext from './layout_context';

export interface EdgeUII {
    createRoot(g: any): void;
    root: any;
    gLink: any;
    tick(): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
    update(): void;
}

export interface EdgeUIConstructableI {
    new(): EdgeUII;
}

export class EdgeUI implements EdgeUII {
    gLink: any;
    root: any;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext
    }
    createRoot(g: any) {
        this.root = g;
        this.gLink = g.append("g").attr('class', 'links').selectAll(".link");
    }
    tick() {
    }

    update() {
    }

}

