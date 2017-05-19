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
            <node-selector class="inject-target"\
                           placeholder="Interface 1"\
                           form="capture"\
                           v-model="node1"></node-selector>\
            <node-selector placeholder="Interface 2 (Optionnal)"\
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
      this.visible = false;
    },

    start: function() {
      var self = this;
      if (this.queryError) {
        this.$error({message: this.queryError});
        return;
      }
      this.$captureCreate(this.query, this.name, this.desc, this.bpf)
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
