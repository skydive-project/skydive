/* jshint multistr: true */

var Capture = {

  mixins: [apiMixin],

  props: {

    capture: {
      type: Object,
      required: true,
    }

  },

  template: '\
    <div class="capture-item">\
      <div class="capture-title">\
        <i class="capture-action capture-delete fa fa-trash"\
           @click="remove(capture)">\
        </i>\
        <i class="capture-action fa"\
           title="Monitor flows"\
           v-if="canShowFlows"\
           :class="{\'fa-expand\': !showFlows, \'fa-compress\': showFlows}"\
           @click="showFlows = !showFlows"></i>\
        {{capture.UUID}}\
      </div>\
      <dl class="dl-horizontal">\
        <dt v-if="capture.Name">Name</dt>\
        <dd v-if="capture.Name">{{capture.Name}}</dd>\
        <dt>Query</dt>\
        <dd class="query"\
            @mouseover="highlightCaptureNodes(capture, true)"\
            @mouseout="highlightCaptureNodes(capture, false)"\
            @click="(canShowFlows ? showFlows = !showFlows : null)">\
          {{capture.GremlinQuery}}\
        </dd>\
        <dt v-if="capture.Description">Desc</dt>\
        <dd v-if="capture.Description">{{capture.Description}}</dd>\
        <dt v-if="capture.BPFFilter">BPF</dt>\
        <dd v-if="capture.BPFFilter">{{capture.BPFFilter}}</dd>\
        <dt v-if="showFlows">Flows</dt>\
        <dd v-if="showFlows">\
          <flow-table :value="capture.GremlinQuery + \'.Flows().Dedup()\'"></flow-table>\
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

  },

  methods: {

    remove: function(capture) {
      var self = this,
          uuid = capture.UUID;
      this.deleting = true;
      this.$captureDelete(uuid)
        .always(function() {
          self.deleting = false;
        });
    },

    highlightCaptureNodes: function(capture, bool) {
      var self = this;
      // Avoid highlighting the nodes while the capture
      // is being deleted
      if (this.deleting) {
        return;
      }
      this.$topologyQuery(capture.GremlinQuery)
        .then(function(nodes) {
          nodes.forEach(function(n) {
            if (bool)
              self.$store.commit("highlight", n.ID);
            else
              self.$store.commit("unhighlight", n.ID);
          });
        });
    }

  }

};


Vue.component('capture-list', {

  mixins: [apiMixin],

  components: {
    'capture': Capture,
  },

  template: '\
    <div class="sub-left-panel" v-if="count > 0">\
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
    websocket.addMsgHandler('OnDemand', this.onMsg.bind(this));
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
      this.$captureList()
        .then(function(data) {
          self.captures = data;
        });
    },

    onMsg: function(msg) {
      switch(msg.Type) {
        case "CaptureDeleted":
          Vue.delete(this.captures, msg.Obj.UUID);
          break;
        case "CaptureAdded":
          Vue.set(this.captures, msg.Obj.UUID, msg.Obj);
          break;
      }
    }

  }

});
