/* jshint multistr: true */

Vue.component('noderule-form', {

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div>\
      <form @submit.prevent="create" class="noderule-form" v-if="visible">\
        <div class="form-group">\
          <label for="rule-name">Name</label>\
          <input id="rule-name" type="text" class="form-control input-sm" v-model="name" />\
        </div>\
        <div class="form-group">\
          <label for="rule-desc">Description</label>\
          <textarea id="rule-desc" type="text" class="form-control input-sm" rows="2" v-model="desc"></textarea>\
        </div>\
        <div class="form-group">\
          <label>Action</label></br>\
          <label class="radio-inline">\
            <input type="radio" id="create" value="create" v-model="action"> Create\
            <span class="checkmark"></span>\
          </label>\
          <label class="radio-inline">\
            <input type="radio" id="update" value="update" v-model="action"> Update\
            <span class="checkmark"></span>\
          </label>\
        </div>\
        <div v-if="action === \'create\'">\
          <div class="form-group">\
            <label for="node-name">Node Name</label>\
            <input id="node-name" type="text" class="form-control input-sm" v-model="nodename" />\
          </div>\
          <div class="form-group">\
            <label for="node-type">Node Type</label>\
            <input id="node-type" type="text" class="form-control input-sm" v-model="nodetype" />\
          </div>\
        </div>\
        <div v-if="action === \'update\'">\
          <div class="form-group">\
            <label for="query">Gremlin Query</label>\
            <textarea id="query" type="text" class="form-control input-sm" rows="2" v-model="query"></textarea>\
          </div>\
        </div>\
        <div class="form-group">\
          <label for="metadata">Metadata</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="comma(,) separated key=value pair. ex: key1= value1, key2=value2"></i></a>\
          <textarea id="metadata" type="text" class="form-control input-sm" rows="2" v-model="metadata"></textarea>\
        </div>\
        <button type="submit" id="create-noderule" class="btn btn-primary">Create</button>\
        <button type="button" class="btn btn-danger" @click="reset">Cancel</button>\
      </form>\
      <button type="button"\
              id="create-noderule"\
              class="btn btn-primary"\
              v-else\
              @click="visible = !visible">\
        Create\
      </button>\
    </div>\
  ',

  data: function() {
    return {
      visible: false,
      name: "",
      desc: "",
      action: "create",
      nodename: "",
      nodetype: "",
      query: "",
      metadata: "",
    };
  },

  methods: {
    reset: function() {
      this.visible = false;
      this.name = this.desc = this.nodetype = this.nodename = this.query = this.methods = "";
      this.action = "create";
    },

    parseMetadata: function(data) {
      md = {};
      if ((data = data.trim()) === "") return md;
      datas = data.split(",");
      for (i = 0; i < datas.length; i++) {
        e = datas[i].split("=");
        if (e.length != 2) throw "Invalid Metadata format";
        md[e[0]] = e[1];
      }
      return md;
    },

    create: function() {
      var self = this;
      var noderule = new api.NodeRule();
      noderule.Name = this.name;
      noderule.Description = this.desc;
      noderule.Action = this.action;

      try {
        noderule.Metadata = this.parseMetadata(this.metadata);
      }
      catch(err) {
        this.$error({message: err});
        return;
      }

      if (this.action === "create") {
        if (this.nodename === "" || this.nodetype === "") {
          this.$error({message: "'Node Name' and 'Node Type' are mandatory"});
          return;
        }
        noderule.Metadata["Name"] = this.nodename;
        noderule.Metadata["Type"] = this.nodetype;
      } else {
        if (this.query === "" || this.metadata === "") {
          this.$error({message: "'Query' and 'Metadata' are mandatory"});
          return;
        }
        noderule.Query = this.query;
      }

      return self.noderuleAPI.create(noderule)
        .then(function(data) {
          self.$success({message: "Node Rule created"});
          self.reset();
          app.$emit("refresh-noderule-list");
          return data;
        })
        .catch(function(e) {
          self.$error({message: 'Node Rule create error: ' + e.responseText});
          return e;
        });
    }
  }
});
