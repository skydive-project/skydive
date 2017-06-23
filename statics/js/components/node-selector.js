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
    form: {
      type: String,
    }
  },

  mixins: [notificationMixin],

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
          if (self.form == "capture") {
            var allowedTypes = ["ovsbridge", "device", "internal", "veth", "tun", "bridge", "dummy", "gre", "bond", "can", "hsr", "ifb", "macvlan", "macvtap", "vlan", "vxlan", "gretap", "ip6gretap", "geneve", "ipoib", "vcan", "ipip", "ipvlan", "lowpan", "ip6tnl", "ip6gre", "sit"];
            if (allowedTypes.indexOf(e.target.__data__.metadata.Type) > -1) {
              node = value = e.target.__data__;
            } else {
              self.$error({message: "Capture not allowed on this node"});
              return;
            }
          } else {
            node = value = e.target.__data__;
          }
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
