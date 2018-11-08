import DataSourceI from "./interface";
import * as events from 'events';
import WsHandler from './websocket';

export default class BaseSkydiveDataSource implements DataSourceI {
    sourceType: string = "skydive";
    dataSourceName: string = "base_skydive_data_source";
    e: events.EventEmitter = new events.EventEmitter();
    subscribable: boolean = true;
    time: any;
    filterQuery: string = "";
    websocket: WsHandler;

    constructor() {
        this.onConnected = this.onConnected.bind(this);
        this.processMessage = this.processMessage.bind(this);
        this.websocket = new WsHandler(window.location.host + "/ws/subscriber?x-client-type=webui");
    }

    subscribe() {
        this.websocket.e.on('websocket.messageGraph', this.processMessage);
        this.websocket.e.once('websocket.connected', this.onConnected);
        this.websocket.connect();
    }

    unsubscribe() {
        this.e.removeAllListeners();
        this.websocket.e.removeAllListeners('websocket.messageGraph');
        this.websocket.disconnect();
    }

    onConnected() {
        console.log('Send sync request');
        const obj: any = {};
        if (this.time) {
            obj.Time = this.time;
        }

        obj.GremlinFilter = this.filterQuery;
        const msg = { "Namespace": "Graph", "Type": "SyncRequest", "Obj": obj };
        console.log('send msg', msg);
        this.websocket.send(msg);
    }

    processMessage(msg: any) {
        console.log('Got message from websocket', msg);
        this.e.emit('broadcastMessage', msg.Type, msg)
    }

}
