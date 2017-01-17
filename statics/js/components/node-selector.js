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
    }
  },

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
      $(".topology-d3").off('click');
      $(".topology-d3").on('click', function(e) {
        var value, node;
        if (! e.target.__data__) {
          return;
        } else {
          node = value = e.target.__data__;
        }

        self.attr.split(".").forEach(function(key) {
          if (! value[key]) {
            return;
          } else {
            value = value[key];
          }
        });

        self.$emit('input', value);
        self.$emit('selected', node);
        e.preventDefault();
        $(".topology-d3").off('click');
      });
    }

  }

});
