import * as events from 'events';

import DataManager from '../data_manager';
import LayoutConfig from '../../config';

export interface LayoutBridgeUII {
    e: events.EventEmitter;
    config: LayoutConfig;
    useEventEmitter(e: events.EventEmitter): void;
    selector: string;
    dataManager: DataManager;
    linkLabelStrategy: any;
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
    config: LayoutConfig;
    initialized: boolean = false;
    collapseLevel: number = 1;
    minimumCollapseLevel: number = 1;
    autoExpand: boolean = false;
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
    start() {
        this.initialized = false;
        this.initialized = true;
    }
    remove() {
    }
}
