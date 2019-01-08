/* jshint multistr: true */

Vue.component('edgerule-form', {

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div>\
      <form @submit.prevent="create" class="edgerule-form" v-if="visible">\
        <div class="form-group">\
          <label for="rule-name">Name</label>\
          <input id="rule-name" type="text" class="form-control input-sm" v-model="name" />\
        </div>\
        <div class="form-group">\
          <label for="rule-desc">Description</label>\
          <textarea id="rule-desc" type="text" class="form-control input-sm" rows="2" v-model="desc"></textarea>\
        </div>\
        <div class="form-group">\
          <label>Source Node</label>\
          <node-selector id="node-selector-src" class="inject-target"\
                         placeholder="src interface"\
                         form="edgerule"\
                         v-model="src"></node-selector>\
          <label>Destination Node</label>\
          <node-selector id="node-selector-dst" placeholder="dst interface"\
                         form="edgerule"\
                         v-model="dst"></node-selector>\
        </div>\
        <div class="form-group">\
          <label for="type">Relationship type</label>\
          <input list="relation-types" id="type" type="text" v-model="type" class="form-control input-sm">\
          </input>\
          <datalist id="relation-types">\
            <option value="layer2">\
            <option value="ownership">\
          </datalist>\
        </div>\
        <div class="form-group">\
          <label for="metadata">Metadata</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="comma(,) separated key=value pair. ex: key1= value1, key2=value2"></i></a>\
          <textarea id="metadata" type="text" class="form-control input-sm" rows="2" v-model="metadata"></textarea>\
        </div>\
        <button type="submit" id="create-edgerule" class="btn btn-primary">Create</button>\
        <button type="button" class="btn btn-danger" @click="reset">Cancel</button>\
      </form>\
      <button type="button"\
              id="create-edgerule"\
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
      src: "",
      dst: "",
      type: "",
      metadata: "",
    };
  },

  methods: {
    reset: function() {
      this.visible = false;
      this.name = this.desc = this.metadata = "";
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
      if (this.src === "" || this.dst === "") {
        this.$error({message: "Source and destination nodes are mandatory."});
        return;
      }
      if (this.type === "") {
        this.$error({message: "Relationship Type is mandatory."});
        return;
      }
      var edgerule = new api.EdgeRule();
      edgerule.Name = this.name;
      edgerule.Description = this.desc;
      edgerule.Src = "G.V().Has('TID', '" + this.src + "')";
      edgerule.Dst = "G.V().Has('TID', '" + this.dst + "')";

      try {
        edgerule.Metadata = this.parseMetadata(this.metadata);
      }
      catch(err) {
        this.$error({message: err});
        return;
      }
      edgerule.Metadata["RelationType"] = this.type;

      return self.edgeruleAPI.create(edgerule)
        .then(function(data) {
          self.$success({message: "Edge Rule created"});
          self.reset();
          app.$emit("refresh-edgerule-list");
          return data;
        })
        .catch(function(e) {
          self.$error({message: 'Edge Rule create error: ' + e.responseText});
          return e;
        });
    }
  }
});
