/* jshint multistr: true */

Vue.component('inject-form', {

  mixins: [apiMixin, notificationMixin],

  template: '\
    <form @submit.prevent="inject">\
      <div class="form-group">\
        <label for="inject-type">Type</label>\
        <select id="inject-type" v-model="type" class="form-control custom-select">\
          <option value="icmp4">ICMPv4/Echo request</option>\
          <option value="icmp6">ICMPv6/Echo request</option>\
          <option value="tcp4">TCP/IPv4</option>\
          <option value="tcp6">TCP/IPv6</option>\
          <option value="udp4">UDP/IPv4</option>\
          <option value="udp6">UDP/IPv6</option>\
        </select>\
      </div>\
      <div class="form-group">\
        <label>Source</label>\
        <node-selector class="inject-target"\
                       placeholder="From"\
                       id="inject-src"\
                       v-model="node1"></node-selector>\
        <div class="input-group">\
          <label for="inject-src-ip" class="input-group-addon inject-label">IP: </label>\
          <input id="inject-src-ip" class="form-control" v-model="srcIP" placeholder="Auto"/>\
        </div>\
        <div class="input-group">\
          <label for="inject-src-mac" class="input-group-addon inject-label">MAC: </label>\
          <input id="inject-src-mac" class="form-control" v-model="srcMAC" placeholder="Auto"/>\
        </div>\
      </div>\
      <div class="form-group">\
        <label>Destination</label>\
        <node-selector placeholder="To"\
                       class="inject-target"\
                       id="inject-dst"\
                       v-model="node2"></node-selector>\
        <div class="input-group">\
          <label for="inject-dst-ip" class="input-group-addon inject-label">IP: </label>\
          <input id="inject-dst-ip" class="form-control" v-model="dstIP" placeholder="Auto"/>\
        </div>\
        <div class="input-group">\
          <span for="inject-dst-mac" class="input-group-addon inject-label">MAC: </span>\
          <input id="inject-dst-mac" class="form-control" v-model="dstMAC" placeholder="Auto"/>\
        </div>\
      </div>\
      <div class="form-group">\
        <label for="inject-count">Nb. of packets</label>\
        <input id="inject-count" type="number" class="form-control input-sm" v-model.number="count" min="1" />\
      </div>\
      <div v-if="type === \'icmp4\' || type === \'icmp6\'">\
        <div class="form-group">\
          <label for="inject-count">ICMP ID</label>\
          <input id="inject-id" type="number" class="form-control input-sm" v-model.number="id" min="0" />\
        </div>\
        <div class="form-group">\
          <label class="form-check-label">\
            <input id="inject-increment" type="checkbox" class="form-check-input" v-model="increment">\
            Increment ID at every packet\
            <span class="checkmark"></span>\
          </label>\
        </div>\
      </div>\
      <div class="form-group form-inline">\
        <label class="radio-inline">\
          <input type="radio" id="random" value="random" v-model="mode"> Payload Length\
          <span class="checkmark"></span>\
        </label>\
        <label class="radio-inline">\
          <input type="radio" id="custom" value="custom" v-model="mode"> Payload\
          <span class="checkmark"></span>\
        </label>\
      </div>\
      <div class="form-group" v-if="mode == \'random\'">\
        <label for="payload-length">Payload length</label>\
        <input id="payload-length" type="number" class="form-control input-sm" v-model.number="payloadlength" min="0" />\
      </div>\
      <div class="form-group" v-if="mode == \'custom\'">\
        <label for="payld">Payload</label>\
        <input id="payld" type="text" class="form-control input-sm" v-model="payload"/>\
      </div>\
      <div v-if="type === \'tcp4\' || type === \'tcp6\' || type === \'udp4\' || type === \'udp6\'">\
        <div class="form-group">\
          <label for="src-port">Src Port</label>\
          <input id="src-port" type="number" class="form-control input-sm" v-model.number="port1" min="0" />\
        </div>\
        <div class="form-group">\
          <label for="dst-port">Dst Port</label>\
          <input id="dst-port" type="number" class="form-control input-sm" v-model.number="port2" min="0" />\
        </div>\
      </div>\
      <div class="form-group">\
        <label for="inject-interval">Interval in milliseconds</label>\
        <input id="inject-interval" type="number" class="form-control input-sm" v-model.number="interval" min="0" />\
      </div>\
      <button type="submit" id="inject" class="btn btn-primary">Inject</button>\
      <button type="button" class="btn btn-danger" @click="reset">Reset</button>\
    </form>\
  ',

  data: function() {

    return {
      node1: "",
      node2: "",
      count: 1,
      type: "icmp4",
      id: 0,
      interval: 0,
      increment: false,
      port1: 0,
      port2: 0,
      payloadlength: 0,
      srcNode: null,
      dstNode: null,
      srcIP: "",
      dstIP: "",
      srcMAC: "",
      dstMAC: "",
      mode: "random",
      payload: "",
    };
  },

  created: function() {
    if (this.$store.state.currentNode) {
      this.node1 = this.$store.state.currentNode.ID;
    }
  },

  beforeDestroy: function() {
    // FIXME: we should just call reset() here,
    // but the watchers are not being evaluated :/
    if (this.node1) {
      this.highlightNode(this.node1, false);
    }
    if (this.node2) {
      this.highlightNode(this.node2, false);
    }
  },

  computed: {

    error: function() {
      if (!this.node1 || !this.node2) {
        return "Source and destination interfaces must be selected";
      } else if (this.srcIP === "" && this.dstIP === "") {
        return "Source and Destination IPs need to be given by user";
      } else if (this.srcIP === "") {
        return "Source IP need to be given by user";
      } else if (this.dstIP === "") {
        return "Destination IP need to be given by user";
      } else if (this.srcMAC === "" && this.dstMAC === "") {
        return "Source and Destination MACs need to be given by user";
      } else if (this.srcMAC === "") {
        return "Source MAC need to be given by user";
      } else if (this.dstMAC === "") {
        return "Destination MAC need to be given by user";
      } else {
        return;
      }
    },

  },

  watch: {

    node1: function(newVal, oldVal) {
      if (oldVal) {
        this.highlightNode(oldVal, false);
      }
      if (newVal) {
        this.highlightNode(newVal, true);
        this.srcIP = this.getIP(this.srcNode = this.$store.state.currentNode);
        this.srcMAC = this.getMAC(this.srcNode);
      }
    },

    node2: function(newVal, oldVal) {
      if (oldVal) {
        this.highlightNode(oldVal, false);
      }
      if (newVal) {
        this.highlightNode(newVal, true);
        this.dstIP = this.getIP(this.dstNode = this.$store.state.currentNode);
        this.dstMAC = this.getMAC(this.dstNode);
      }
    },

    type: function(newVal, oldVal) {
      if(newVal.slice(-1) != oldVal.slice(-1)) {
        if(this.srcNode !== null) this.srcIP = this.getIP(this.srcNode);
        if(this.dstNode !== null) this.dstIP = this.getIP(this.dstNode);
      }
    }

  },

  methods: {

    highlightNode: function(id, bool) {
      if (!id) return;
      if (bool)
        this.$store.commit('highlight', id);
      else
        this.$store.commit('unhighlight', id);
    },

    getIP: function(node) {
      md = node.metadata;
      ipFamily = this.type.slice(-1);
      if (ipFamily == "4" && "IPV4" in md) {
        return md.IPV4[0];
      } else if (ipFamily == "6" && "IPV6" in md) {
        return md.IPV6[0];
      } else {
        return "";
      }
    },

    getMAC: function(node) {
      return node.metadata.MAC || "";
    },

    reset: function() {
      var self = this;
      this.node1 = this.node2 = "";
      this.count = 1;
      this.type = "icmp4";
      this.payloadlength = 0;
      this.srcNode = this.dstNode = null;
      this.srcIP = this.dstIP = "";
      this.srcMAC = this.dstMAC = "";
      this.mode = "random";
      this.payload = "";
    },

    inject: function() {
      var self = this;
      if (this.error) {
        this.$error({message: this.error});
        return;
      }
      if (this.mode == "random") {
        this.payload = (new Array(this.payloadlength)).join("x").toString();
      }
      $.ajax({
        dataType: "json",
        url: '/api/injectpacket',
        data: JSON.stringify({
          "Src": "G.V().Has('TID', '" + this.node1 + "')",
          "Dst": "G.V().Has('TID', '" + this.node2 + "')",
          "SrcPort": this.port1,
          "DstPort": this.port2,
          "SrcIP": this.srcIP,
          "DstIP": this.dstIP,
          "SrcMAC": this.srcMAC,
          "DstMAC": this.dstMAC,
          "Type": this.type,
          "Count": this.count,
          "ICMPID": this.id,
          "Increment": this.increment,
          "Interval": this.interval,
          "Payload": this.payload,
        }),
        contentType: "application/json; charset=utf-8",
        method: 'POST',
      })
      .then(function() {
        self.$success({message: 'Packet injected'});
        app.$emit("refresh-injector-list");
      })
      .fail(function(e) {
        self.$error({message: 'Packet injection error: ' + e.responseText});
      });

      self.highlightNode(self.node1, false);
      self.highlightNode(self.node2, false);
    },

  }

});
