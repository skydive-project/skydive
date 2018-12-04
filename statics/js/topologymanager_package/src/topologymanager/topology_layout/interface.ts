import LayoutConfig from '../config';
import * as events from 'events';
import { LayoutBridgeUII } from './base/ui/index';

export default interface TopologyLayoutI {
    uiBridge: LayoutBridgeUII;
    e: events.EventEmitter;
    alias: string;
    active: boolean;
    config: LayoutConfig;
    initializer(): void;
    remove(): void;
    useConfig(config: LayoutConfig): void;
}
