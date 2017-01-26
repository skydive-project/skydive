Vue.component('capture-list', {

  template: '\
    <div class="sub-left-panel" v-if="count > 0">\
      <ul class="capture-list">\
        <li class="capture-item"\
            v-for="capture in captures"\
            :id="capture.UUID">\
          <div class="capture-title">\
            <i class="capture-delete fa fa-trash"\
               @click="remove(capture)">\
            </i>\
            {{capture.UUID}}\
          </div>\
          <dl class="dl-horizontal">\
            <dt v-if="capture.Name">Name</dt>\
            <dd v-if="capture.Name">{{capture.Name}}</dd>\
            <dt>Query</dt>\
            <dd class="query"\
                @mouseover="highlightCaptureNodes(capture, true)"\
                @mouseout="highlightCaptureNodes(capture, false)">\
              {{capture.GremlinQuery}}\
            </dd>\
            <dt v-if="capture.Description">Desc</dt>\
            <dd v-if="capture.Description">{{capture.Description}}</dd>\
          </dl>\
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
      CaptureAPI.list()
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
    },

    remove: function(capture) {
      var self = this,
          uuid = capture.UUID;
      self.deleting.push(uuid);
      CaptureAPI.delete(uuid)
        .always(function() {
          self.deleting.splice(self.deleting.indexOf(uuid), 1);
        });
    },

    highlightCaptureNodes: function(capture, bool) {
      // Avoid highlighting the nodes while the capture
      // is being deleted
      if (this.deleting.indexOf(capture.UUID) !== -1) {
        return;
      }
      TopologyAPI.query(capture.GremlinQuery)
        .then(function(nodes) {
          nodes.forEach(function(n) {
            topologyLayout.SetNodeClass(n.ID, "highlighted", bool);
          });
        });
    }

  }

});
