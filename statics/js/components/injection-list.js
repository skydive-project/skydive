/* jshint multistr: true */

var Injector = {
  mixins: [apiMixin, notificationMixin],

  props: {
    injector: {
      type: Object,
      required: true,
    }
  },

  template: '\
    <div class="injector-item">\
      <div class="injector-title">\
        <i class="injector-action injector-delete fa fa-trash"\
          @click="remove(injector)">\
        </i>\
        {{injector.UUID}}\
      </div>\
      <dl class="dl-horizontal">\
        <dt>Src</dt>\
        <dd class="query"\
            @mouseover="highlightNode(injector.Src)"\
            @mouseout="unhighlightNode()">\
          {{injector.Src}}\
        </dd>\
        <dt v-if="injector.Dst">Dst</dt>\
        <dd v-if="injector.Dst" class="query"\
            @mouseover="highlightNode(injector.Dst)"\
            @mouseout="unhighlightNode()">\
          {{injector.Dst}}\
        </dd>\
      </dl>\
    </div>\
  ',

  methods: {
    remove: function(injector) {
      this.$injectorDelete(injector.UUID)
        .always(function() {
          app.$emit("referesh-injector-list");
        });
    },

    highlightNode: function(query, id) {
      var self = this;
      this.$topologyQuery(query)
        .then(function(nodes) {
          nodes.forEach(function(n) {
            self.$store.commit("highlight", n.ID);
          });
        });
    },

    unhighlightNode: function() {
      var ids = this.$store.state.highlightedNodes.slice();
      for (var i in ids) {
        this.$store.commit('unhighlight', ids[i]);
      }
    },
  },
};

Vue.component('injection-list', {
  mixins: [apiMixin, notificationMixin],

  components: {
    'injector': Injector,
  },

  template: '\
    <div class="sub-panel" v-if="count > 0">\
      <ul class="injector-list">\
        <li class="injector-item" v-for="injector in injectors" :id="injector.UUID">\
          <injector :injector="injector"></injector>\
        </li>\
      </ul>\
    </div>\
  ',

  data: function() {
    return {
      injectors: {},
    };
  },

  computed: {
    count: function() {
      return Object.keys(this.injectors).length;
    },
  },

  created: function() {
    var self = this;
    setInterval(this.getInjectorList.bind(this), 30000);
    this.getInjectorList();
    app.$on("referesh-injector-list", function() {
      self.getInjectorList();
    });
  },

  methods: {
    getInjectorList: function() {
      var self = this;
      this.$injectorList()
        .then(function(data) {
          self.injectors = data;
        });
    },
  },

});
