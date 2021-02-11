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
            @mouseover="highlightNode(injector.UUID, injector.Src)"\
            @mouseout="unhighlightNode(injector.UUID)">\
          {{injector.Src}}\
        </dd>\
        <dt v-if="injector.Dst">Dst</dt>\
        <dd v-if="injector.Dst" class="query"\
            @mouseover="highlightNode(injector.UUID, injector.Dst)"\
            @mouseout="unhighlightNode(injector.UUID)">\
          {{injector.Dst}}\
        </dd>\
      </dl>\
    </div>\
  ',

  methods: {
    remove: function(injector) {
      var self = this;
      this.injectAPI.delete(injector.UUID)
        .catch(function (e) {
          self.$error({message: 'Packet injector delete error: ' + e.responseText});
          return e;
        })
    },

    highlightNode: function(uuid, query) {
      var self = this;
      self.$store.commit("highlightStart", uuid);
      this.$topologyQuery(query)
        .then(function(nodes) {
          nodes.forEach(function(n) {
            self.$store.commit("highlight", n.ID);
          });
          self.$store.commit("highlightEnd", uuid);
        });
    },

    waitForHighlight: function(uuid) {
      var self = this;
      setTimeout(function(){
        var status = self.$store.state.highlightInprogress.get(uuid);
        if (status) {
          self.waitForHighlight(uuid);
          return;
        }
        var ids = self.$store.state.highlightedNodes.slice();
        for (var i in ids) {
          self.$store.commit('unhighlight', ids[i]);
        }
        self.$store.commit('highlightDelete', uuid);
      }, 100);
    },

    unhighlightNode: function(uuid) {
      this.waitForHighlight(uuid);
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
    websocket.addMsgHandler('OnDemandPacketInjectionNotification', this.onMsg.bind(this));
    websocket.addConnectHandler(this.init.bind(this));
  },

  beforeDestroy: function() {
    websocket.delConnectHandler(this.init.bind(this));
  },

  methods: {

    init: function() {
      var self = this;
      this.injectAPI.list()
        .then(function(data) {
          self.injectors = data;
        })
        .catch(function (e) {
          self.$error({message: 'Packet injector list error: ' + e.responseText});
          return e;
        });
    },

    onMsg: function(msg) {
      var self = this;
      switch(msg.Type) {
        case "Deleted":
          Vue.delete(this.injectors, msg.Obj.UUID);
          break;
        case "Added":
          Vue.set(this.injectors, msg.Obj.UUID, msg.Obj);
          break;
        case "NodeUpdated":
          this.injectAPI.get(msg.Obj.UUID)
            .then(function(data) {
              Vue.set(self.injectors, data.UUID, data);
            })
            .catch(function (e) {
              self.$error({message: 'Injection get error: ' + e.responseText});
              return e;
            });
      }
    }

  },

});
