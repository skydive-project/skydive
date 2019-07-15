Vue.component('node-selector', {

  props: {
    value: {
      type: String,
      required: true,
    },
    placeholder: {
      type: String,
    },
    attr: {
      type: String,
      default: "metadata.TID"
    },
    name: {
      type: String,
      default: "metadata.Name"
    },
    form: {
      type: String,
    }
  },

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div style="position:relative">\
      <input v-bind:class="{ \'node-select-active\' : isActive }" class="form-control input-sm has-left-icon"\
             readonly\
             @keyup.delete="clean($event)"\
             @click="select($event)"\
             :placeholder="placeholder"\
             :value="value" />\
      <span class="fa fa-crosshairs form-control-feedback"></span>\
      <span v-if="multiNodes" class="fa fa-plus-circle form-control-feedback" style="top: 8px"></span>\
      <input v-bind:class="{ \'node-select-active\' : isActive }" v-for="node in nodes.slice(1)" class="form-control input-sm has-left-icon"\
             readonly\
             @keyup.delete="clean($event)"\
             :placeholder="placeholder"\
             @focus="$event.target.select()"\
             :value="node" />\
    </div>\
  ',

  data: function () {
    return {
      nodes: [],
      isActive: false,
      multiNodes: false
    }
  },

  mounted: function() {
    var self = this;
    // until the ctrl release we allow to select and append node
    this.$store.subscribe(function(mutation) {
      if (mutation.type === "ctrlPressed" && self.isActive) {
        if (mutation.payload === false) {
          $(".topology-d3").off('click');
          self.isActive = false;
          self.multiNodes = false;
        } else {
          self.multiNodes = true;
        }
      }
    })
  },

  methods: {

    clean: function (event) {
      if (event.target.value === this.value) {
        this.$emit('input', "");
      } else {
        this.nodes = this.nodes.filter(function(e) { return event.target.value !== e})
      }
    },

    getValue: function(value) {
      var attr = value;
      this.attr.split(".").forEach(function (key) {
        if (attr[key]) {
          attr = attr[key];
        } else {
          attr = "";
          return
        }
      });

      var name = value;
      this.name.split(".").forEach(function (key) {
        if (name[key]) {
          name = name[key];
        } else {
          name = "";
          return;
        }
      });
 
      value = attr;
      if (name) {
        value += " - " + name;
      }

      return value;
    },

    select: function (event) {
      var self = this;

      this.isActive = true;

      if (this.value !== "") {
        event.target.select();
      }

      $(".topology-d3").off('click');
      $(".topology-d3").on('click', function (e) {
        // if when selecting ctrl is not pressed then reset the list 
        // so that only one node will be selected
        if (!self.$store.state.isCtrlPressed) {
          self.nodes = [];
          self.isActive = false;
          $(".topology-d3").off('click');
        }

        var value, node;
        if (!e.target.__data__) {
          return;
        } else if (e.target.__data__.metadata) {
          if (self.form == "capture") {
            if (self.$allowedTypes().indexOf(e.target.__data__.metadata.Type) > -1) {
              node = value = e.target.__data__;
            } else {
              self.$error({ message: "Capture not allowed on this node" });
              $(".topology-d3").off('click');
              return;
            }
          } else {
            node = value = e.target.__data__;
          }
        } else {
          return;
        }

        value = self.getValue(value);
        if (value) {
          self.nodes.unshift(value);
          self.$emit('input', value);
          self.$emit('nodeSelected', node);
        } else {
          self.$error({ message: "Can not select, metadata `" + self.attr + "` is missing" });
          $(".topology-d3").off('click');
          self.isActive = false;
        }
        e.preventDefault();
      });
    },

  }

});
