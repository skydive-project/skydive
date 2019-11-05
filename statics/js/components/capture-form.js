/* jshint multistr: true */

Vue.component('capture-form', {

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div>\
      <form @submit.prevent="start" class="capture-form" v-if="visible">\
        <div class="form-group">\
          <label for="capture-name">Name</label>\
          <input id="capture-name" type="text" class="form-control input-sm" v-model="name" />\
        </div>\
        <div class="form-group">\
          <label for="capture-desc">Description</label>\
          <textarea id="capture-desc" type="text" class="form-control input-sm" rows="2" v-model="desc"></textarea>\
        </div>\
        <div class="form-group form-inline">\
          <label class="radio-inline">\
            <input type="radio" id="by-node" name="capture-target" value="selection" v-model="mode"/> Nodes selection\
            <span class="checkmark"></span>\
          </label>\
          <label class="radio-inline">\
            <input type="radio" id="by-gremlin" name="capture-target" value="gremlin" v-model="mode"/> Gremlin Expression\
            <span class="checkmark"></span>\
          </label>\
        </div>\
        <div class="form-group" v-if="mode == \'selection\'">\
          <label>Targets</label>\
          <node-selector id="node-selector-1" class="inject-target"\
                         placeholder="Interface 1"\
                         form="capture"\
                         v-model="node1"></node-selector>\
          <node-selector id="node-selector-2" placeholder="Interface 2 (Optional)"\
                         form="capture"\
                         v-model="node2"></node-selector>\
        </div>\
        <div class="form-group" v-if="mode == \'gremlin\'">\
          <label for="capture-query">Query</label>\
          <textarea id="capture-query" type="text" class="form-control input-sm" rows="5" v-model="userQuery"></textarea>\
        </div>\
        <div class="form-group">\
          <label for="capture-bpf">BPF filter</label>\
          <div class="input">\
            <input list="capture-bpf-favorites" placeholder="e.g. port 80 and port 443"\
            id="capture-bpf" type="text" v-model="bpf"\
            class="input-sm form-control"></input>\
            <datalist id="capture-bpf-favorites" class="capture-bpf-favorites">\
            </datalist>\
          </div>\
        </div>\
        <collapse :collapsed="true" class="form-group">\
          <h1 slot="collapse-header" slot-scope="props" :class="{\'closed\': !props.active}">\
            <span class="pull-left">\
              <i class="glyphicon glyphicon-chevron-right rotate" :class="{\'down\': props.active}"></i>\
            </span>\
            <span class="advanced-options">Advanced options</span>\
          </h1>\
          <fieldset slot="collapse-body" class="form-group">\
            <div class="form-group">\
              <label for="capture-type">Capture Type</label>\
              <select id="capture-type" v-model="captureType" class="form-control custom-select" :disabled="!typeAllowed">\
                <option disabled value="">Select capture type</option>\
                <option v-for="option in options" :value="option.type">{{ option.type }} ({{option.desc}})</option>\
              </select>\
            </div>\
            <div class="form-group" v-if="captureType == \'sflow\'">\
              <label for="port">Port</label>\
              <input id="port" type="number" class="form-control input-sm" v-model.number="port" min="0"/>\
            </div>\
            <div class="form-group" v-if="captureType == \'ovssflow\'">\
              <label for="samplingRate">Flow Sampling Rate (0 = No FLows)</label>\
              <input id="samplingRate" type="number" class="form-control input-sm" v-model.number="samplingRate"/>\
              <label for="pollingInterval">Counter Polling Interval (0 = No Counters)</label>\
              <input id="pollingInterval" type="number" class="form-control input-sm" v-model.number="pollingInterval"/>\
            </div>\
            <div class="form-group">\
              <label for="target">Target</label>\
              <input id="target" type="text" class="form-control input-sm" v-model="target"/>\
            </div>\
            <div class="form-group" v-if="captureType != \'ovssflow\' && captureType != \'ovsnetflow\'">\
              <label for="targetType">Target type</label>\
              <select id="targetType" v-model="targetType" class="form-control custom-select">\
                <option disabled value="">Select target type</option >\
                <option value="">None</option>\
                <option value="netflowv5">NetFlow</option>\
                <option value="erspanv1">Erspan</option>\
              </select>\
            </div>\
            <div class="form-group">\
              <div class="form-group">\
                <label for="capture-layer-key-mode">Layers used for Flow Key</label>\
                <select id="capture-layer-key-mode" v-model="layerKeyMode" class="form-control custom-select">\
                  <option disabled value="">Select layers to be used</option>\
                  <option value="" selected>Default</option>\
                  <option value="L2">L2 (uses Layer 2 and beyond)</option>\
                  <option value="L3">L3 (uses layer 3 and beyond)</option>\
                </select>\
              </div>\
              <div class="form-group">\
                <label for="capture-header-size">Header Size</label>\
                <input id="capture-header-size" type="number" class="form-control input-sm" v-model.number="headerSize" min="0" />\
              </div>\
              <div class="form-group">\
                <label class="form-check-label">\
                  <input id="capture-tcp-metric" type="checkbox" class="form-check-input" v-model="extraTCPMetric">\
                  Extra TCP metric\
                  <span class="checkmark"></span>\
                </label>\
                <label class="form-check-label">\
                  <input id="capture-ip-defrag" type="checkbox" class="form-check-input" v-model="ipDefrag">\
                  Defragment IPv4 packets\
                  <span class="checkmark"></span>\
                </label>\
                <label class="form-check-label">\
                  <input id="capture-reassemble-tcp" type="checkbox" class="form-check-input" v-model="reassembleTCP">\
                  Reassemble TCP packets\
                  <span class="checkmark"></span>\
                </label>\
                <div class="form-group" v-if="isPacketCaptureEnabled && !targetType">\
                  <label for="capture-raw-packets">Raw packets limit</label>\
                  <input id="capture-raw-packets" type="number" class="form-control input-sm" v-model.number="rawPackets" min="0" max="10"/>\
                </div>\
              </div>\
            </div>\
          </fieldset>\
        </collapse>\
        <button type="submit" id="start-capture" class="btn btn-primary">Start</button>\
        <button type="button" class="btn btn-danger" @click="reset">Cancel</button>\
      </form>\
      <button type="button"\
              id="create-capture"\
              class="btn btn-primary"\
              v-else\
              @click="toggleForm">\
        Create\
      </button>\
    </div>\
  ',

  data: function() {
    return {
      node1: "",
      node2: "",
      queryNodes: [],
      name: "",
      desc: "",
      bpf: "",
      headerSize: 0,
      rawPackets: 0,
      extraTCPMetric: true,
      ipDefrag: false,
      reassembleTCP: false,
      userQuery: "",
      mode: "selection",
      visible: false,
      captureType: "",
      layerKeyMode: "",
      nodeType: "",
      typeAllowed: false,
      port: 0,
      samplingRate: 1,
      pollingInterval: 10,
      isPacketCaptureEnabled: true,
      target: "",
      targetType: "",
    };
  },

  beforeDestroy: function() {
    this.resetQueryNodes();
  },

  computed: {

    options: function() {
      var options = {};
      for (let t of this.$allowedTypes()) {
        options[t] = [
          {"type": "afpacket", "desc": "MMap'd AF_PACKET socket reading"},
          {"type": "pcap", "desc": "Packet Capture library based probe"},
          {"type": "pcapsocket", "desc": "Socket reading PCAP format data"},
          {"type": "sflow", "desc": "Socket reading sFlow frames"},
          {"type": "ebpf", "desc": "Flow capture within kernel - experimental"},
          {"type": "ovsmirror", "desc": "Leverages mirroring to capture - experimental"}
        ];
      }
      options.ovsbridge = [
        {"type": "ovsnetflow", "desc": "Reading NetFlow from OVS"},
        {"type": "ovssflow", "desc": "Reading sFlow from OVS"},
        {"type": "pcapsocket", "desc": "Socket reading PCAP format data"}
      ];
      options.dpdkport = [
        {"type": "dpdk", "desc": "DPDK based probe - experimental"}
      ];
      return options[this.nodeType];
    },

    queryError: function() {
      if (this.mode == "gremlin" && !this.userQuery) {
          return "Gremlin query can't be empty";
      } else if (this.mode == "selection" && !this.node1) {
          return "At least one interface has to be selected";
      } else {
          return;
      }
    },

    query: function() {
      if (this.queryError) {
        return;
      }
      if (this.mode == "gremlin") {
        return this.userQuery;
      } else {
        var q = "G.V().Has('TID', '" + this.node1 + "')";
        if (this.node2)
          q += ".ShortestPathTo(Metadata('TID', '" + this.node2 + "'), Metadata('RelationType', 'layer2'))";
        return q;
      }
    }

  },

  mounted: function() {
    var self = this;

    self.isPacketCaptureEnabled = app.enforce("capture", "rawpackets");
  },

  watch: {

    visible: function(newValue) {
      if (newValue === true &&
          this.$store.state.currentNode &&
          this.$store.state.currentNode.isCaptureAllowed() &&
          this.$store.state.currentNode.metadata.TID) {
        this.node1 = this.$store.state.currentNode.metadata.TID;
      }
    },

    query: function(newQuery) {
      var self = this;
      if (!newQuery) {
        this.resetQueryNodes();
        return;
      }
      this.$topologyQuery(newQuery)
        .then(function(nodes) {
          self.resetQueryNodes();
          self.queryNodes = nodes;
          self.highlightQueryNodes(true);
          if (nodes.length === 1) {
            self.typeAllowed = true;
            self.nodeType = nodes[0].Metadata.Type;
          } else {
            self.typeAllowed = false;
          }
        })
        .catch(function() {
          self.resetQueryNodes();
          self.typeAllowed = false;
        });
    },

    node1: function(newNode) {
      var node = this.$store.state.currentNode;
      this.nodeType = node.metadata.Type;
    },

    node2: function(newNode) {
      if (newNode) {
        this.typeAllowed = false;
      } else {
        this.typeAllowed = true;
      }
    },

    mode: function(newMode) {
      if (newMode === "selection") {
        if (this.node2) {
          this.typeAllowed = false;
        } else {
          this.typeAllowed = true;
          if (this.node1 !== "")
            this.checkQuery("G.V().Has('TID', '" + this.node1 + "')");
        }
      } else if (newMode === "gremlin") {
        if (!this.query) {
          this.typeAllowed = false;
        } else {
          this.checkQuery(this.query);
        }
      }
    },

    captureType: function(newType) {
      this.targetType = "";
    }
  },

  methods: {

    reset: function() {
      this.node1 = this.node2 = this.userQuery = "";
      this.name = this.desc = this.bpf = "";
      this.headerSize = this.rawPackets = 0;
      this.extraTCPMetric = true;
      this.ipDefrag = false;
      this.reassembleTCP = false;
      this.visible = false;
      this.captureType = "";
      this.layerKeyMode = "";
      this.target = "";
      this.targetType = "";
    },

    checkQuery: function(query) {
      var self = this;
      this.$topologyQuery(query)
        .then(function(nodes) {
          if (nodes.length === 1) {
            self.nodeType = nodes[0].Metadata.Type;
            self.typeAllowed = true;
          } else {
            self.typeAllowed = false;
          }
        })
        .catch(function(e) {
          self.typeAllowed = false;
        });
    },

    start: function() {
      var self = this;
      if (this.queryError) {
        this.$error({message: this.queryError});
        return;
      }
      if (!this.typeAllowed)
        this.captureType = "";

      var capture = new api.Capture();
      capture.GremlinQuery = this.query;
      capture.Name = this.name;
      capture.Description = this.desc;
      capture.BPFFilter = this.bpf;
      capture.HeaderSize = this.headerSize;
      capture.RawPacketLimit = this.rawPackets;
      capture.ExtraTCPMetric = this.extraTCPMetric;
      capture.Type = this.captureType;
      capture.Port = this.port;
      capture.IPDefrag = this.ipDefrag;
      capture.ReassembleTCP = this.reassembleTCP;
      capture.LayerKeyMode = this.layerKeyMode;
      capture.SamplingRate = this.samplingRate;
      capture.PollingInterval = this.pollingInterval;
      capture.Target = this.target;
      capture.TargetType = this.targetType;
      return self.captureAPI.create(capture)
      .then(function(data) {
        self.$success({message: 'Capture created'});
        self.reset();
        return data;
      })
      .catch(function(e) {
        self.$error({message: 'Capture create error: ' + e.responseText});
        return e;
      });
    },

    resetQueryNodes: function() {
      this.highlightQueryNodes(false);
      this.queryNodes = [];
    },

    highlightQueryNodes: function(bool) {
      var self = this;
      this.queryNodes.forEach(function(n) {
        if (bool)
          self.$store.commit('highlight', n.ID);
        else
          self.$store.commit('unhighlight', n.ID);
      });
    },

    setBPFFavoritesFromConfig: function() {
      var self = this;
      var options = $(".capture-bpf-favorites");

      if (typeof(Storage) !== "undefined" && localStorage.preferences) {
        var bpf = JSON.parse(localStorage.preferences).bpf;
        if (bpf) {
          $.each(bpf, function(i, f) {
            options.append($("<option/>").text(f.name).val(f.expression));
          });
        }
      }

      var favorites = app.getConfigValue('bpf.favorites');
      if (!favorites || favorites.length === 0) return;
      $.each(favorites, function(key, value) {
        options.append($("<option/>").text(key).val(value));
      });
    },

    toggleForm: function() {
      this.visible = !this.visible;
      setTimeout(this.setBPFFavoritesFromConfig, 200);
    },

  }

});
