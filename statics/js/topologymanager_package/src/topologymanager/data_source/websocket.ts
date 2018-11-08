import * as events from 'events';

export default class WSHandler {
    protocol: string;
    e: events.EventEmitter = new events.EventEmitter();
    conn: any;
    connectionUrl: string;

    constructor(connectionUrl: string) {
        this.connectionUrl = connectionUrl;
        this.protocol = "ws://";
        if (location.protocol == "https:") {
            this.protocol = "wss://";
        }
    }

    connect() {
        if (this.conn && this.conn.readyState == window.WebSocket.OPEN) {
            return;
        }
        this._connect();
    }

    _connect() {
        this.conn = new WebSocket(this.protocol + this.connectionUrl);
        this.conn.onopen = () => {
            this.e.emit('websocket.connected');
        };
        this.conn.onclose = () => {
            this.e.emit('websocket.disconnected');
        };
        this.conn.onmessage = (r: any) => {
            const msg = JSON.parse(r.data);
            this.e.emit('websocket.message' + msg.Namespace, msg);
        };
        this.conn.onerror = () => {
            this.e.emit('websocket.error');
        };
    }

    disconnect() {
        this.conn && this.conn.close();
    }

    send(msg: any) {
        this.conn.send(JSON.stringify(msg));
    }

}
