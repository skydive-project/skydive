import * as events from 'events';
import LayoutConfig from '../../config';
import DataManager from '../data_manager';
export default class LayoutContext {
    dataManager: DataManager;
    config: LayoutConfig;
    e: events.EventEmitter;
    linkLabelStrategy: any;
    getCollapseLevel(): number {
        return 1;
    }
    getMinimumCollapseLevel(): number {
        return 1;
    }
    isAutoExpand(): boolean {
        return false;
    }
    get collapseLevel() {
        return Math.max(this.getCollapseLevel(), this.getMinimumCollapseLevel());
    }
    subscribeToEvent(eventName: string, cb: any) {
        this.e.on(eventName, cb);
    }
}
