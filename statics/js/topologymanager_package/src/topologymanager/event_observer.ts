const upperCaseFirst = require('upper-case-first');

export default class EventObserver {
    private handlers: Array<any> = [];

    addHandler(handler: any) {
        this.handlers.push(handler);
    }

    removeHandler(handler: any) {
        var index = this.handlers.indexOf(handler);
        if (index > -1) {
            this.handlers.splice(index, 1);
        }
    }

    notifyHandlers(obj: any, ev: string, v1: any, v2: any) {
        var i, h;
        for (i = this.handlers.length - 1; i >= 0; i--) {
            h = this.handlers[i];
            try {
                var callback = h["on" + upperCaseFirst(ev)];
                if (callback) {
                    callback.bind(h)(v1, v2);
                }
            } catch (e) {
                console.log(e);
            }
        }
    }
}

