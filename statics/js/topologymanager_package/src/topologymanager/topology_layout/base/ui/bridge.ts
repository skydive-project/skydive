import * as events from 'events';

import { LayoutUII, LayoutUI } from './layout';
import { NodeUII, NodeUI } from './node';
import { GroupUII, GroupUI } from './group';
import { EdgeUII, EdgeUI } from './link';
import LayoutConfig from '../../../config';
import LayoutContext from './layout_context';

export interface LayoutBridgeUII {
    e: events.EventEmitter;
    config: LayoutConfig;
    useEventEmitter(e: events.EventEmitter): void;
    layoutContext: LayoutContext;
    selector: string;
    layoutUI: LayoutUII;
    nodeUI: NodeUII;
    groupUI: GroupUII;
    edgeUI: EdgeUII;
    useLayoutUI(layoutUI: LayoutUII): void;
    useNodeUI(nodeUI: NodeUII): void;
    useGroupUI(groupUI: GroupUII): void;
    useEdgeUI(edgeUI: EdgeUII): void;
    useConfig(config: LayoutConfig): void;
    start(): void;
    remove(): void;
}

export interface LayoutBridgeUIConstructableI {
    new(selector: string): LayoutBridgeUII;
}

export class LayoutBridgeUI implements LayoutBridgeUII {
    edgeUI: EdgeUII;
    e: events.EventEmitter;
    selector: string;
    nodeUI: NodeUII;
    groupUI: GroupUII;
    layoutUI: LayoutUII;
    config: LayoutConfig;
    initialized: boolean = false;
    constructor(selector: string) {
        this.selector = selector;
    }
    useEventEmitter(e: events.EventEmitter) {
        this.e = e;
    }
    useNodeUI(nodeUI: NodeUII) {
        this.nodeUI = nodeUI;
    }
    useGroupUI(groupUI: GroupUII) {
        this.groupUI = groupUI;
    }
    useEdgeUI(edgeUI: EdgeUII) {
        this.edgeUI = edgeUI;
    }
    useConfig(config: LayoutConfig) {
        this.config = config;
    }
    useLayoutUI(layoutUI: LayoutUII) {
        this.layoutUI = layoutUI;
    }
    start() {
        this.initialized = false;
        this.layoutUI.useLayoutContext(this.layoutContext);
        this.layoutUI.createRoot();
        this.groupUI.useLayoutContext(this.layoutContext);
        this.groupUI.createRoot(this.layoutUI.g);
        this.edgeUI.useLayoutContext(this.layoutContext);
        this.edgeUI.createRoot(this.layoutUI.g);
        this.nodeUI.useLayoutContext(this.layoutContext);
        this.nodeUI.createRoot(this.layoutUI.g);
        this.layoutContext.subscribeToEvent('ui.tick', this.tick.bind(this));
        this.layoutContext.subscribeToEvent('ui.update', this.update.bind(this));
        this.layoutUI.start();
        this.e.emit('ui.update');
        this.initialized = true;
    }
    remove() {
    }
    get layoutContext(): LayoutContext {
        const context = new LayoutContext();
        context.e = this.e;
        context.config = this.config;
        return context;
    }
    tick() {
        this.edgeUI.tick();
        this.nodeUI.tick();
        this.groupUI.tick();
    }
    update() {
        if (!this.initialized) {
            return;
        }
        this.nodeUI.update();
        this.edgeUI.update();
        this.groupUI.update();
        this.layoutUI.restartsimulation();
    }
}
