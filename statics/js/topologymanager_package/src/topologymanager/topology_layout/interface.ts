import LayoutConfig from './config';
import * as events from 'events';
import { DataManager } from './base/index';
import { LayoutBridgeUII } from './base/ui/index';

export default interface TopologyLayoutI {
    uiBridge: LayoutBridgeUII;
    dataManager: DataManager;
    e: events.EventEmitter;
    alias: string;
    active: boolean;
    config: LayoutConfig;
    initializer(): void;
    remove(): void;
    useConfig(config: LayoutConfig): void;
    reactToTheUiEvent(eventName: string, ...args: Array<any>): void;
}
