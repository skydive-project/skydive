import * as events from 'events';

import DataManager from '../data_manager';
import { LayoutUII, LayoutUI } from './layout';
import LayoutConfig from '../../config';
import { Node } from '../node/index';
import { Edge } from '../edge/index';
import { Group } from '../group/index';
import LayoutContext from './layout_context';

export interface LayoutBridgeUII {
    e: events.EventEmitter;
    config: LayoutConfig;
    useEventEmitter(e: events.EventEmitter): void;
    layoutContext: LayoutContext;
    selector: string;
    dataManager: DataManager;
    layoutUI: LayoutUII;
    linkLabelStrategy: any;
    useLayoutUI(layoutUI: LayoutUII): void;
    useDataManager(dataManager: DataManager): void;
    useConfig(config: LayoutConfig): void;
    start(): void;
    setAutoExpand(autoExpand: boolean): void;
    setCollapseLevel(level: number): void;
    setMinimumCollapseLevel(level: number): void;
    useLinkLabelStrategy(linkLabelStrategy: any): void;
    remove(): void;
}

export interface LayoutBridgeUIConstructableI {
    new(selector: string): LayoutBridgeUII;
}

export class LayoutBridgeUI implements LayoutBridgeUII {
    e: events.EventEmitter;
    selector: string;
    dataManager: DataManager;
    layoutUI: LayoutUII;
    config: LayoutConfig;
    initialized: boolean = false;
    collapseLevel: number = 1;
    minimumCollapseLevel: number = 1;
    autoExpand: boolean = false;
    invalidGraph: boolean = false;
    intervalId: any;
    linkLabelStrategy: any;
    constructor(selector: string) {
        this.selector = selector;
    }
    useEventEmitter(e: events.EventEmitter) {
        this.e = e;
    }
    useLinkLabelStrategy(linkLabelStrategy: any) {
        this.linkLabelStrategy = linkLabelStrategy;
    }
    setAutoExpand(autoExpand: boolean) {
        this.autoExpand = autoExpand;
    }
    setCollapseLevel(level: number) {
        this.collapseLevel = level;
    }
    setMinimumCollapseLevel(level: number) {
        this.minimumCollapseLevel = level;
    }
    useDataManager(dataManager: DataManager) {
        this.dataManager = dataManager;
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
        this.layoutContext.subscribeToEvent('ui.tick', this.tick.bind(this));
        this.layoutContext.subscribeToEvent('ui.update', this.invalidateGraph.bind(this));
        this.intervalId = window.setInterval(() => {
            if (!this.invalidGraph) {
                return;
            }
            console.log('update graph');
            this.invalidGraph = false;
            this.update();
        }, 100);
        this.e.emit('ui.update');
        this.initialized = true;
    }
    remove() {
        if (this.intervalId) {
            window.clearInterval(this.intervalId);
            this.intervalId = null;
        }
    }
    get layoutContext(): LayoutContext {
        const context = new LayoutContext();
        context.getCollapseLevel = () => this.collapseLevel;
        context.getMinimumCollapseLevel = () => this.minimumCollapseLevel;
        context.isAutoExpand = () => this.autoExpand;
        context.dataManager = this.dataManager;
        context.e = this.e;
        context.config = this.config;
        context.linkLabelStrategy = this.linkLabelStrategy;
        return context;
    }
    tick() {
    }
    update() {
        if (!this.initialized) {
            return;
        }
        this.layoutUI.restartsimulation();
    }
    invalidateGraph() {
        this.invalidGraph = true;
    }
}
