import * as events from 'events';
import LayoutConfig from '../../../config';
import LayoutContext from './layout_context';

export interface GroupUII {
    createRoot(g: any): void;
    g: any;
    root: any;
    tick(): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
    update(): void;
}

export interface GroupUIConstructableI {
    new(): GroupUII;
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
    }

    update() {
    }

}
