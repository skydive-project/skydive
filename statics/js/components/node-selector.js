Vue.component('node-selector', {

  props: {
    value: {
      type: String,
      required: true,
    },
    placeholder: {
      type: String,
    },
  },

  template: '\
    <div style="position:relative">\
      <input class="form-control input-sm has-left-icon"\
             readonly\
             @focus="select" \
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
        if (e.target.__data__ && e.target.__data__.Metadata && e.target.__data__.Metadata.TID) {
          self.$emit('input', e.target.__data__.Metadata.TID);
          self.$emit('selected', e.target.__data__);
          e.preventDefault();
          $(".topology-d3").off('click');
        }
      });
    }

  }

});
