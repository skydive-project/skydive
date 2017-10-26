/* jshint multistr: true */

Vue.component('capture-form', {

  mixins: [apiMixin, notificationMixin],

  template: '\
    <transition name="slide" mode="out-in">\
      <div class="sub-left-panel" v-if="visible">\
        <form @submit.prevent="start" class="capture-form">\
          <div class="form-group">\
            <label for="capture-name">Name</label>\
            <input id="capture-name" type="text" class="form-control input-sm" v-model="name" />\
          </div>\
          <div class="form-group">\
            <label for="capture-desc">Description</label>\
            <textarea id="capture-desc" type="text" class="form-control input-sm" rows="2" v-model="desc"></textarea>\
          </div>\
          <div class="form-group">\
            <label class="radio-inline">\
              <input type="radio" id="by-node" name="capture-target" value="selection" v-model="mode"> Nodes selection\
            </label>\
            <label class="radio-inline">\
              <input type="radio" id="by-gremlin" name="capture-target" value="gremlin" v-model="mode"> Gremlin Expression\
            </label>\
          </div>\
          <div class="form-group" v-if="mode == \'selection\'">\
            <label>Targets</label>\
            <node-selector id="node-selector-1" class="inject-target"\
                           placeholder="Interface 1"\
                           form="capture"\
                           v-model="node1"></node-selector>\
            <node-selector id="node-selector-2" placeholder="Interface 2 (Optionnal)"\
                           form="capture"\
                           v-model="node2"></node-selector>\
          </div>\
          <div class="form-group" v-if="mode == \'gremlin\'">\
            <label for="capture-query">Query</label>\
            <textarea id="capture-query" type="text" class="form-control input-sm" rows="5" v-model="userQuery"></textarea>\
          </div>\
          <div class="form-group">\
            <label for="capture-bpf">BPF filter</label>\
            <input id="capture-bpf" type="text" class="form-control input-sm" v-model="bpf" />\
          </div>\
          <div class="capture-advanced">\
            <div class="panel-heading">\
              <h4 class="panel-title">\
                <a data-toggle="collapse" data-parent="#" href="#one">\
                  Advanced options\
                <i class="indicator glyphicon glyphicon-chevron-down pull-right"></i>\
                </a>\
              </h4>\
            </div>\
            <div id="one" class="panel-collapse collapse">\
              <fieldset class="form-group">\
                <div class="form-group">\
                  <label for="capture-header-size">Header Size</label>\
                  <input id="capture-header-size" type="number" class="form-control input-sm" v-model="headerSize" min="0" />\
                </div>\
                <div class="form-group">\
                  <label for="capture-raw-packets">Raw packets limit</label>\
                  <input id="capture-raw-packets" type="number" class="form-control input-sm" v-model="rawPackets" min="0" max="10"/>\
                </div>\
                <div class="form-group">\
                  <label class="form-check-label">\
                    <input id="capture-tcp-metric" type="checkbox" class="form-check-input" v-model="tcpMetric">\
                    Extra TCP metric\
                  </label>\
                </div>\
                <div class="form-group">\
                  <label class="form-check-label">\
                    <input id="capture-socket-info" type="checkbox" class="form-check-input" v-model="socketInfo">\
                    TCP Socket Info\
                  </label>\
                </div>\
              </fieldset>\
            </div>\
          </div>\
          <button type="submit" id="start-capture" class="btn btn-primary">Start</button>\
          <button type="button" class="btn btn-danger" @click="reset">Cancel</button>\
        </form>\
      </div>\
      <button type="button"\
              id="create-capture"\
              class="btn btn-primary"\
              v-else\
              @click="visible = !visible">\
        Create\
      </button>\
    </transition>\
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
      tcpMetric: true,
      socketInfo: false,
      userQuery: "",
      mode: "selection",
      visible: false,
    };
  },

  beforeDestroy: function() {
    this.resetQueryNodes();
  },

  computed: {

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
        })
        .fail(function() {
          self.resetQueryNodes();
        });
    }

  },

  methods: {

    reset: function() {
      this.node1 = this.node2 = this.userQuery = "";
      this.name = this.desc = this.bpf = "";
      this.headerSize = this.rawPackets = 0;
      this.tcpMetric = true;
      this.socketInfo = false;
      this.visible = false;
    },

    start: function() {
      var self = this;
      if (this.queryError) {
        this.$error({message: this.queryError});
        return;
      }
      this.$captureCreate(this.query, this.name, this.desc, this.bpf, this.headerSize, this.rawPackets, this.tcpMetric, this.socketInfo)
        .then(function() {
          self.reset();
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
    }

  }

});
