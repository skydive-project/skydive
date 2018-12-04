import * as events from 'events';
import LayoutConfig from '../../../config';
export default class LayoutContext {
    config: LayoutConfig;
    e: events.EventEmitter;
    subscribeToEvent(eventName: string, cb: any) {
        this.e.on(eventName, cb);
    }
    unsubscribeFromEvent(eventName: string, cb?: any) {
        if (cb) {
            this.e.removeListener(eventName, cb);
        } else {
            this.e.removeAllListeners(eventName);
        }
    }
}
