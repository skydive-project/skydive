/* jshint multistr: true */

Vue.component('fabric-form', {

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div v-if="visible">\
      <form @submit.prevent="create" class="fabric-form">\
        <div class="form-group">\
          <label for="fabric-node-name">Name</label>\
          <input id="fabric-node-name" type="text" class="form-control input-sm" v-model="name" />\
        </div>\
        <div class="form-group">\
          <label for="metadata">Metadata</label>\
          <textarea id="metadata" type="text" class="form-control input-sm" rows="2" v-model="metadata"></textarea>\
        </div>\
        <button type="submit" id="create-fabric" class="btn btn-primary">Create</button>\
        <button type="button" class="btn btn-danger" @click="cancel">Cancel</button>\
      </form>\
    </div>\
  ',

  data: function() {
    return {
      visible: false,
      name: "",
      metadata: "",
      subType: "",
      parentID: "",
    };
  },

  created: function() {
    var self = this;
    app.$on("show-fabric-form", function() {
      self.visible = true;
    });
    app.$on("node-host", function() {
      self.subType = "host";
    });
    app.$on("node-port", function() {
      self.subType = "port";
    });
    app.$on("parent-id", function(id) {
      self.parentID = id;
    });
  },

  methods: {
    create: function() {
      var self = this;
      this.$createFabricNode(this.name, this.metadata, "node", this.subType, this.parentID)
        .then(function() {
          self.visible = false;
        });
    },

    cancel: function() {
      this.visible = false;
    }
  }
});

Vue.component('fabric-link', {

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div>\
      <form @submit.prevent="create" v-if="visible">\
        <div class="form-group">\
          <label>Target Node</label>\
          <node-selector id="node-selector" class="inject-target"\
                         placeholder="target node"\
                         form="fabric-link"\
                         attr="id"\
                         v-model="target"></node-selector>\
        </div>\
        <div class="form-group">\
          <label for="link-type">Link Type</label>\
          <select id="link-type" v-model="subType" class="form-control input-sm">\
            <option disabled value="">select link type</option>\
            <option value="ownership">Ownership</option>\
            <option value="layer2">Layer 2</option>\
            <option value="both">Both</option>\
          </select>\
        </div>\
        <div class="form-group">\
          <label for="metadata">Metadata</label>\
          <textarea id="metadata" type="text" class="form-control input-sm" rows="2" v-model="metadata"></textarea>\
        </div>\
        <button type="submit" id="create-fabric-link" class="btn btn-primary">Create</button>\
        <button type="button" class="btn btn-danger" @click="cancel">Cancel</button>\
      </form>\
    </div>\
  ',

  data: function() {
    return {
      visible: false,
      metadata: "",
      source: "",
      target: "",
      subType: "",
    };
  },

  created: function() {
    var self = this;
    app.$on("src-id", function(id) {
      self.source = id;
    });
    app.$on("show-fabric-link-form", function() {
      self.visible = true;
    });
  },

  methods: {
    create: function() {
      var self = this;
      this.$createFabricLink(this.source, this.target, this.subType, this.metadata)
        .then(function() {
          self.visible = false;
        });
    },

    cancel: function() {
      this.visible = false;
    }
  }
});
