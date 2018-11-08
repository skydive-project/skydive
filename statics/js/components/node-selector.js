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
      default: "Metadata.TID"
    },
    form: {
      type: String,
    }
  },

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div style="position:relative">\
      <input class="form-control input-sm has-left-icon"\
             readonly\
             @focus="select"\
             :placeholder="placeholder"\
             :value="value" />\
      <span class="fa fa-crosshairs form-control-feedback"></span>\
    </div>\
  ',

  methods: {

    select: function() {
      var self = this;
      globalEventHandler.onUiEvent('node.select', function(node) {
        if (!node) {
	  return;
	}
        if (self.form == "capture") {
	  if (self.$allowedTypes().indexOf(node.Metadata.Type) === -1) {
            self.$error({message: "Capture not allowed on this node"});
	    return;
	  }
	}
        var found = true;
	var value = node;
        self.attr.split(".").forEach(function(key) {
          if (! value[key]) {
            found = false;
            return;
          } else {
            value = value[key];
          }
        });

        if (found) {
          self.$emit('input', value);
          self.$emit('nodeSelected', node);
        } else {
          self.$error({message: "Capture not allowed, required metadata missing `" + self.attr + "`"});
        }
      }, true);
    }

  }

});
