/* jshint multistr: true */

var Capture = {

  mixins: [apiMixin, notificationMixin],

  props: {

    capture: {
      type: Object,
      required: true,
    }

  },

  template: '\
    <div class="capture-item">\
      <div class="capture-title">\
        <i v-if="canWriteCaptures" class="capture-action capture-delete fa fa-trash"\
           @click="remove(capture)">\
        </i>\
        <i class="capture-action fa"\
           title="Monitor flows"\
           v-if="canShowFlows"\
           :class="{\'fa-expand\': !showFlows, \'fa-compress\': showFlows}"\
           @click="showFlows = !showFlows"></i>\
        <i v-if="!capture.Count" class="capture-action capture-warning fa fa-exclamation-triangle" \
          v-tooltip.bottom-left="{content: warningMessage()}"></i>\
        {{capture.UUID}}\
      </div>\
      <dl class="dl-horizontal">\
        <dt v-if="capture.Name">Name</dt>\
        <dd v-if="capture.Name">{{capture.Name}}</dd>\
        <dt>Query</dt>\
        <dd class="query"\
            @mouseover="highlightNodes(capture)"\
            @mouseout="unhighlightNodes(capture)"\
            @click="(canShowFlows ? showFlows = !showFlows : null)">\
          {{capture.GremlinQuery}}\
        </dd>\
        <dt v-if="capture.Description">Desc</dt>\
        <dd v-if="capture.Description">{{capture.Description}}</dd>\
        <dt v-if="capture.Type">Type</dt>\
        <dd v-if="capture.Type">{{capture.Type}}</dd>\
        <dt v-if="capture.LayerKeyMode">Layer mode</dt>\
        <dd v-if="capture.LayerKeyMode">{{capture.LayerKeyMode}}</dd>\
        <dt v-if="capture.BPFFilter">BPF</dt>\
        <dd v-if="capture.BPFFilter">{{capture.BPFFilter}}</dd>\
        <dt v-if="capture.HeaderSize">Header</dt>\
        <dd v-if="capture.HeaderSize">{{capture.HeaderSize}}</dd>\
        <dt v-if="capture.RawPacketLimit">R. Pkts</dt>\
        <dd v-if="capture.RawPacketLimit">{{capture.RawPacketLimit}}</dd>\
        <dt v-if="capture.ExtraLayers">Extra layers</dt>\
        <dd v-if="capture.ExtraLayers">{{extraLayers}}</dd>\
        <dt v-if="capture.Target">Target</dt>\
        <dd v-if="capture.Target">{{capture.Target}}</dd>\
        <dt v-if="showFlows">Flows</dt>\
        <dd v-if="showFlows">\
          <flow-table :value="\'G.Flows().Has(\\\'CaptureID\\\', \\\'\' + capture.UUID + \'\\\').Dedup()\'"></flow-table>\
        </dd>\
      </dl>\
    </div>\
  ',

  data: function() {
    return {
      showFlows: false,
      deleting: false,
    };
  },

  computed: {

    // https://github.com/skydive-project/skydive/issues/202
    canShowFlows: function() {
      return this.capture.GremlinQuery.search('ShortestPathTo') === -1;
    },

    canWriteCaptures: function() {
      return app.enforce("capture", "write");
    },

    extraLayers: function() {
      return this.capture.ExtraLayers.join(', ');
    }

  },

  methods: {

    warningMessage: function() {
      return '\
        <b>This capture doesn\'t currently match any node</b><br><br>\
        Please check there is a Layer 2 link in case of 2 nodes capture.\
      ';
    },

    remove: function(capture) {
      var self = this;
      this.deleting = true;
      this.captureAPI.delete(capture.UUID)
        .catch(function (e) {
          self.$error({message: 'Capture delete error: ' + e.responseText});
          return e;
        })
        .finally(function() {
          self.deleting = false;
        });
    },

    waitForHighlight: function(uuid) {
      var self = this;
      setTimeout(function() {
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

    unhighlightNodes: function(capture) {
      this.waitForHighlight(capture.UUID);
    },

    highlightNodes: function(capture) {
      var self = this;
      // Avoid highlighting the nodes while the capture
      // is being deleted
      if (this.deleting) {
        return;
      }
      this.$store.commit('highlightStart', capture.UUID);
      this.$topologyQuery(capture.GremlinQuery)
        .then(function(nodes) {
          nodes.forEach(function(n) {
            self.$store.commit("highlight", n.ID);
          });
          self.$store.commit('highlightEnd', capture.UUID);
        });
    }

  }

};


Vue.component('capture-list', {

  mixins: [apiMixin, notificationMixin],

  components: {
    'capture': Capture,
  },

  template: '\
    <div class="sub-panel" v-if="count > 0">\
      <ul class="capture-list">\
        <li class="capture-item"\
            v-for="capture in captures"\
            :id="capture.UUID">\
          <capture :capture="capture"></capture>\
        </li>\
      </ul>\
    </div>\
  ',

  data: function() {
    return {
      captures: {},
      deleting: [],
      timer: null,
    };
  },

  created: function() {
    websocket.addMsgHandler('OnDemandCaptureNotification', this.onMsg.bind(this));
    websocket.addConnectHandler(this.init.bind(this));
  },

  beforeDestroy: function() {
    websocket.delConnectHandler(this.init.bind(this));
  },

  computed: {
    count: function() {
      return Object.keys(this.captures).length;
    }
  },

  methods: {

    init: function() {
      var self = this;
      self.captureAPI.list()
        .then(function(data) {
          self.captures = data;
        })
        .catch(function (e) {
          console.log("Error while listing captures: " + e);
          if (e.status === 405) { // not allowed
            return $.Deferred().promise([]);
          }
          self.$error({message: 'Capture list error: ' + e.responseText});
          return e;
        });
    },

    onMsg: function(msg) {
      var self = this;
      switch(msg.Type) {
        case "Deleted":
          Vue.delete(this.captures, msg.Obj.UUID);
          break;
        case "Added":
          Vue.set(this.captures, msg.Obj.UUID, msg.Obj);
          break;
        case "NodeUpdated":
          this.captureAPI.get(msg.Obj.UUID)
            .then(function(data) {
              Vue.set(self.captures, data.UUID, data);
            })
            .catch(function (e) {
              self.$error({message: 'Capture get error: ' + e.responseText});
              return e;
            });
      }
    }

  }

});
