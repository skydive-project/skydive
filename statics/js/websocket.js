function WSHandler() {
  this.host = location.host;
  this.conn = null;
  this.connected = null;
  this.disconnected = null;
  this.msgHandlers = {};
  this.discHandlers = [];
  this.connHandlers = [];
}

WSHandler.prototype = {

  connect: function() {
    var self = this;

    this._connect()
      .fail(function() {
        console.error("Failed to connect to WS server, trying again in 1s");
        setTimeout(self.connect.bind(self), 1000);
      })
      .then(function() {
        // once we are connected we can react on disconnections
        self.disconnected
          .then(function() {
            console.error("Disconnected from WS server, reconnecting");
            self.connect();
          });
      });
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

    this.conn = new WebSocket("ws://" + this.host + "/ws");
    this.conn.onopen = function() {
      $.notify({
      	message: 'Connected'
      },{
      	type: 'success'
      });
      self.connecting = false;
      self.connected.resolve(true);
    };
    this.conn.onclose = function() {
      // connection closed after a succesful connection
      if (self.connecting === false) {
        $.notify({
          message: 'Connection lost'
        },{
          type: 'danger'
        });
        self.disconnected.resolve(true);
      // client never succeed to connect in the first place
      } else {
        $.notify({
          message: 'Failed to connect'
        },{
          type: 'danger'
        });
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

    return self.connected;
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
 
  send: function(msg) {
    this.conn.send(JSON.stringify(msg));
  }

};
