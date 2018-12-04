import * as events from 'events';
import LayoutConfig from '../../../config';
import LayoutContext from './layout_context';

export interface NodeUII {
    createRoot(g: any): void;
    g: any;
    root: any;
    tick(): void;
    update(): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
}

export interface NodeUIConstructableI {
    new(): NodeUII;
}


export class NodeUI implements NodeUII {
    g: any;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext;
    }
    createRoot(g: any) {
        this.g = g.append("g").attr('class', 'nodes').selectAll(".node");
    }
    get root() {
        return this.g;
    }
    tick() {
    }
    update() {
    }


}
