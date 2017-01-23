Vue.component('capture-form', {

  template: '\
    <transition name="slide" mode="out-in">\
      <div class="sub-left-panel" v-if="visible">\
        <div class="title-left-panel">New Capture :</div>\
        <form @submit.prevent="start" class="capture-form">\
          <label>Name : </label></br>\
          <input type="text" class="capture-input" v-model="name" /><br/>\
          <label>Description : </label></br>\
          <textarea type="text" class="capture-input" rows="2" v-model="desc"></textarea><br/>\
          <label>Target : </label></br>\
          <label class="radio-inline">\
            <input type="radio" name="capture-target" value="selection" v-model="mode"> Nodes selection\
          </label>\
          <label class="radio-inline">\
            <input type="radio" name="capture-target" value="gremlin" v-model="mode"> Gremlin Expression\
          </label>\
          <div v-if="mode == \'selection\'">\
            <node-selector placeholder="Interface 1" v-model="node1"></node-selector>\
            </br>\
            <node-selector placeholder="Interface 2 (Optionnal)" v-model="node2"></node-selector>\
          </div>\
          <br/>\
          <div v-if="mode == \'gremlin\'">\
            <textarea type="text" class="capture-input" v-model="userQuery"></textarea><br/>\
          </div>\
          <button type="submit" class="btn btn-primary">Start</button>\
          <button type="button" class="btn btn-danger" @click="reset">Cancel</button>\
          <br/>\
        </form>\
      </div>\
      <button type="button"\
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
      userQuery: "",
      mode: "selection",
      visible: false,
    };
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
          CurrentNodeDetails &&
          CurrentNodeDetails.Metadata.TID &&
          CurrentNodeDetails.IsCaptureAllowed()) {
        this.node1 = CurrentNodeDetails.Metadata.TID;
      }
    },

    query: function(newQuery) {
      var self = this;
      if (!newQuery) {
        this.resetQueryNodes();
        return;
      }
      TopologyAPI.query(newQuery)
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
      this.name = this.desc = "";
      this.visible = false;
    },

    start: function() {
      var self = this;
      if (this.queryError) {
        $.notify({message: this.queryError}, {type: 'danger'});
        return;
      }
      CaptureAPI.create(this.query, this.name, this.desc)
        .then(function() {
          self.reset();
        });
    },

    resetQueryNodes: function() {
      this.highlightQueryNodes(false);
      this.queryNodes = [];
    },

    highlightQueryNodes: function(bool) {
      this.queryNodes.forEach(function(n) {
        topologyLayout.SetNodeClass(n.ID, "highlighted", bool);
      });
    }

  }

});
