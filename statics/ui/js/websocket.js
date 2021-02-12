function WSHandler() {
  this.host = location.host;
  this.conn = null;
  this.connected = null;
  this.disconnected = null;
  this.msgHandlers = {};
  this.discHandlers = [];
  this.connHandlers = [];
  this.errorHandlers = [];
}

WSHandler.prototype = {

  connect: function() {
    var self = this;

    if (this.conn && this.conn.readyState == WebSocket.OPEN) {
      return;
    }

    this._connect();
  },

  _connect: function() {
    var self = this;

    this.connected = $.Deferred();
    this.connected.then(function() {
      self.connHandlers.forEach(function(callback) {
        callback();
      });
    });
    this.disconnected = $.Deferred();
    this.disconnected.then(function() {
      self.discHandlers.forEach(function(callback) {
        callback();
      });
    });
    this.connecting = true;

    this.protocol = "ws://";
    if (location.protocol == "https:") {
      this.protocol = "wss://";
    }
    this.conn = new WebSocket(this.protocol + this.host + "/ws/subscriber?x-client-type=webui&x-update-policy=partial");
    this.conn.onopen = function() {
      self.connecting = false;
      self.connected.resolve(true);
    };
    this.conn.onclose = function() {
      // connection closed after a successful connection
      if (self.connecting === false) {
        self.disconnected.resolve(true);
        // client never succeed to connect in the first place
      } else {
        self.connecting = false;
        self.connected.reject(false);
      }
    };
    this.conn.onmessage = function(r) {
      var msg = JSON.parse(r.data);
      if (self.msgHandlers[msg.Namespace]) {
        self.msgHandlers[msg.Namespace].forEach(function(callback) {
          callback(msg);
        });
      }
    };
    this.conn.onerror = function(r) {
      self.errorHandlers.forEach(function(callback) {
        callback();
      });
    };
  },

  disconnect: function() {
    this.conn.close();
  },

  addMsgHandler: function(namespace, callback) {
    if (! this.msgHandlers[namespace]) {
      this.msgHandlers[namespace] = [];
    }
    this.msgHandlers[namespace].push(callback);
  },

  addConnectHandler: function(callback) {
    this.connHandlers.push(callback);
    if (this.connected !== null) {
      this.connected.then(function() {
        callback();
      });
    }
  },

  delConnectHandler: function(callback) {
    this.connHandlers.splice(
      this.connHandlers.indexOf(callback), 1);
  },

  addDisconnectHandler: function(callback) {
    this.discHandlers.push(callback);
    if (this.disconnected !== null) {
      this.disconnected.then(function() {
        callback();
      });
    }
  },

  addErrorHandler: function(callback) {
    this.errorHandlers.push(callback);
  },

  send: function(msg) {
    this.conn.send(JSON.stringify(msg));
  }

};
