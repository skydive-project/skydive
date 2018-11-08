import * as events from 'events';

export default class WSHandler {
    protocol: string;
    e: events.EventEmitter = new events.EventEmitter();
    conn: any;
    connectionUrl: string;
    connecting: boolean = false;
    connected: boolean = false;
    reconnect: boolean = true;

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
        if (this.connecting || this.connected) {
            return;
        }
        this._connect();
    }

    _connect() {
        this.connecting = true;
        this.connected = false;
        this.conn = new WebSocket(this.protocol + this.connectionUrl);
        this.conn.onopen = () => {
            this.connecting = false;
            this.connected = true;
            this.e.emit('websocket.connected');
        };
        this.conn.onclose = () => {
            this.e.emit('websocket.disconnected');
            this.connected = false;
            this.tryToReconnect();
        };
        this.conn.onmessage = (r: any) => {
            const msg = JSON.parse(r.data);
            this.e.emit('websocket.message' + msg.Namespace, msg);
        };
        this.conn.onerror = () => {
            this.e.emit('websocket.error');
            this.connected = false;
            this.tryToReconnect();
        };
    }

    disconnect() {
        this.connected = false;
        this.conn && this.conn.close();
    }

    send(msg: any) {
        this.conn.send(JSON.stringify(msg));
    }

    tryToReconnect() {
        if (!this.reconnect) {
            return;
        }
        this.conn.onopen = function() { }
        this.conn.onclose = function() { }
        this.conn.onmessage = function() { }
        this.conn.onerror = function() { }
        window.setTimeout(() => {
            this.connecting = false;
            this.connect();
        }, 200);
    }

    toggleReconnectMode(status: boolean) {
        this.reconnect = status;
    }

}
