import DataSourceI from "./interface";
import * as events from 'events';

export default class InfraTopologyDataSource implements DataSourceI {
    sourceType: string = "skydive";
    dataSourceName: string = "infra_topology";
    e: events.EventEmitter = new events.EventEmitter();
    subscribable: boolean = true;
    time: any;
    filterQuery: string = "G.V().Has('Type', 'host')";

    constructor() {
        this.onConnected = this.onConnected.bind(this);
        this.processMessage = this.processMessage.bind(this);
    }

    subscribe() {
        window.websocket.removeMsgHandlers('Graph');
        window.websocket.addMsgHandler('Graph', this.processMessage);
        window.websocket.addOneTimeConnectHandler(this.onConnected);
    }

    unsubscribe() {
        this.e.removeAllListeners();
        window.websocket.removeMsgHandlers('Graph');
    }

    onConnected() {
        console.log('Send sync request');
        const obj: any = {};
        if (this.time) {
            obj.Time = this.time;
        }

        obj.GremlinFilter = this.filterQuery + ".SubGraph()";
        const msg = { "Namespace": "Graph", "Type": "SyncRequest", "Obj": obj };
        window.websocket.send(msg);
    }

    processMessage(msg: any) {
        console.log('Got message from websocket', msg);
        this.e.emit('broadcastMessage', msg.Type, msg)
    }

}
