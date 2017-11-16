/* jshint multistr: true */

var TopologyComponent = {

  name: 'topology',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div class="topology">\
      <div class="col-sm-7 fill content">\
      <div class="topology-d3"></div>\
	        <div class="slider">\
	          <slider v-if="history" :min="timeRange[0]" :max="timeRange[1]" \
	                v-model="time" :info="topologyTimeHuman"></slider>\
	          <div class="form-group input-sm" style="width: 450px">\
	              <label for="topology-filter" style="width: 33px">Filter</label>\
	              <input list="topology-filter-list" placeholder="e.g. g.V().Has(,)" \
	              @click="topologyFilterClear" \
	              id="topology-filter" type="text" style="color: black;width: 350px" \
	              v-model="topologyFilter" @keyup.enter="topologyFilterQuery"></input>\
	              <datalist id="topology-filter-list">\
	              </datalist>\
	              <button type="button" class="btn btn-primary btn-sm pull-right" \
	                      @click="topologyFilterQuery"> \
	                <span class="glyphicon glyphicon-search" aria-hidden="true"></span>\
	              </button>\
	          </div>\
	          <div class="form-group input-sm" style="width: 450px">\
	              <label for="topology-highlight" style="width: 33px">Find   </label>\
	              <input list="topology-filter-list" placeholder="e.g. g.V().Has(,)" \
	              @click="topologyHighlightClear" \
	              id="topology-highlight" type="text" style="color: black;width: 350px" \
	              v-model="topologyHighlight" @keyup.enter="topologyHighlightQuery"></input>\
	              <datalist id="topology-filter-list">\
	              </datalist>\
	              <button type="button" class="btn btn-primary btn-sm pull-right" \
	                      @click="topologyHighlightQuery"> \
	                <span class="glyphicon glyphicon-search" aria-hidden="true"></span>\
	              </button>\
	          </div>\
	        </div> \
         <div class="topology-controls">\
          <button id="zoom-in" type="button" class="btn btn-primary"\
                  title="Zoom In" @click="zoomIn">\
            <span class="glyphicon glyphicon-zoom-in" aria-hidden="true"></span>\
          </button>\
          <button id="zoom-out" type="button" class="btn btn-primary"\
                  title="Zoom Out" @click="zoomOut">\
            <span class="glyphicon glyphicon-zoom-out" aria-hidden="true"></span>\
          </button>\
          <button id="zoom-fit" type="button" class="btn btn-primary"\
                  title="Zoom Fit" @click="zoomFit">\
            <span class="glyphicon glyphicon-fullscreen" aria-hidden="true"></span>\
          </button>\
          <button id="expand" type="button" class="btn btn-primary" \
                  title="Expand" @click="toggleCollapseByLevel(false)">\
            <span class="expand-icon-stack">\
              <i class="glyphicon glyphicon-resize-full icon-main"></i>\
            </span>\
            <span class="expand-icon-stack">\
              <i class="glyphicon glyphicon-plus-sign icon-sub"></i>\
            </span>\
          </button>\
          <button id="collapse" type="button" class="btn btn-primary" \
                  title="Collapse" @click="toggleCollapseByLevel(true)">\
            <span class="expand-icon-stack">\
              <i class="glyphicon glyphicon-resize-small icon-main"></i>\
            </span>\
            <span class="expand-icon-stack">\
              <i class="glyphicon glyphicon-minus-sign icon-sub"></i>\
            </span>\
          </button>\
          <button v-if="currentNode != null" id="expand-all" type="button" class="btn btn-primary" \
                  title="Expand/Collapse Current Node Tree" @click="toggleExpandAll(currentNode)">\
            <span class="expand-icon-stack">\
              <span v-if="currentNode.group != null" class="glyphicon icon-main" \
                 :class="{\'glyphicon-resize-full\': currentNode.group.collapsed, \'glyphicon-resize-small\': !currentNode.group.collapsed}" aria-hidden="true"></span>\
            </span>\
          </button>\
        </div>\
      </div>\
      <div id="right-panel" class="col-sm-5 fill info">\
        <tabs v-if="isAnalyzer">\
          <tab-pane title="Captures">\
            <capture-list></capture-list>\
            <capture-form v-if="time === 0"></capture-form>\
          </tab-pane>\
          <tab-pane title="Generator" v-if="time === 0">\
            <inject-form></inject-form>\
          </tab-pane>\
          <tab-pane title="Flows">\
            <flow-table-control></flow-table-control>\
          </tab-pane>\
        </tabs>\
        <transition name="slide" mode="out-in">\
          <div class="left-panel" v-if="currentNode">\
            <span v-if="time" class="label center-block node-time">\
              Interface state at {{timeHuman}}\
            </span>\
            <h1>Metadata<span class="pull-right">(id: {{currentNode.id}})\
              <i v-if="currentNode.group != null" class="node-action fa"\
	              title="Expand/Collapse Node"\
		            :class="{\'fa-expand\': (currentNode.group && currentNode.group.collapsed), \'fa-compress\': (!currentNode.group || !currentNode.group.collapsed)}"\
		            @click="toggleExpandAll(currentNode)"></i></span>\
	        </h1>\
            <div id="metadata-panel" class="sub-left-panel">\
              <object-detail :object="currentNodeMetadata"></object-detail>\
            </div>\
              <div v-if="currentNodeMetadata.Type == \'ovsbridge\'">\
              <h1>Rules</h1>\
              <rule-detail :bridge="currentNode" :graph="graph"></rule-detail>\
            </div>\
            <div id="interface-metrics" v-show="Object.keys(currentNodeStats).length">\
              <h1>Interface metrics</h1>\
              <statistics-table :object="currentNodeStats"></statistics-table>\
            </div>\
            <div id="last-interface-metrics" v-show="Object.keys(currentNodeLastStats).length && time === 0">\
              <h1>Last metrics</h1>\
              <statistics-table :object="currentNodeLastStats"></statistics-table>\
            </div>\
            <div id="flow-table-panel" v-if="isAnalyzer && currentNodeFlowsQuery">\
              <h1>Flows</h1>\
              <flow-table :value="currentNodeFlowsQuery"></flow-table>\
            </div>\
          </div>\
        </transition>\
      </div>\
    </div>\
  ',

  data: function() {
    return {
      time: 0,
      timeRange: [-120, 0],
      collapsed: false,
      topologyFilter: "",
      topologyHighlight: "",
    };
  },

  mounted: function() {
    var self = this;
    // run d3 layout
    this.graph = new Graph(websocket);

    this.layout = new TopologyGraphLayout(this, ".topology-d3");
    this.collapse();

    websocket.addConnectHandler(function() {
      self.collapse();
    });

    this.graph.addHandler(this.layout);
    this.layout.addHandler(this);

    this.syncTopo = debounce(this.graph.syncRequest.bind(this.graph), 300);

    $(this.$el).find('.content').resizable({
      handles: 'e',
      minWidth: 300,
      resize: function(event, ui){
        var x = ui.element.outerWidth();
        var y = ui.element.outerHeight();
        var ele = ui.element;
        var factor = $(this).parent().width() - x;
        var f2 = $(this).parent().width() * 0.02999;
        $.each(ele.siblings(), function(idx, item) {
          ele.siblings().eq(idx).css('height', y+'px');
          ele.siblings().eq(idx).width((factor-f2)+'px');
        });
      }
    });

    // trigered when some component wants to highlight some nodes
    this.$store.subscribe(function(mutation) {
      if (mutation.type == "highlight")
        self.layout.highlightNodeID(mutation.payload);
      else if (mutation.type == "unhighlight")
        self.layout.unhighlightNodeID(mutation.payload);
    });
  },

  beforeDestroy: function() {
    this.$store.commit('unselected');
    this.unwatch();
  },

  watch: {

    topologyFilter: function() {
       var self = this;
       self.topologyFilterQuery();
    },

    topologyHighlight: function() {
       var self = this;
       self.topologyHighlightQuery();
    },

    time: function() {
      var self = this;
      if (this.timeId) {
        clearTimeout(this.timeId);
        this.timeId = null;
      }
      if (this.time !== 0) {
        this.timeId = setTimeout(function() {
          self.time -= 1;
        }, 1000 * 60);
      }
    },

    topologyTime: function(at) {
      if (this.time === 0) {
        this.syncTopo();
      }
      else {
        this.syncTopo(at);
      }
    },

  },

  computed: {

    history: function() {
      return this.$store.state.history;
    },

    isAnalyzer: function() {
      return this.$store.state.service === 'Analyzer';
    },

    currentNode: function() {
      return this.$store.state.currentNode;
    },

    live: function() {
      return this.time === 0;
    },

    topologyTime: function() {
      var time = new Date();
      time.setMinutes(time.getMinutes() + this.time);
      time.setSeconds(0);
      time.setMilliseconds(0);
      return time.getTime();
    },

    timeHuman: function() {
      return this.$store.getters.timeHuman;
    },

    topologyTimeHuman: function() {
      if (this.live) {
        return "live";
      }
      return -this.time + ' min. ago (' + this.timeHuman + ')';
    },

    currentNodeFlowsQuery: function() {
      if (this.currentNode && this.currentNode.isCaptureAllowed())
        return "G.V('" + this.currentNode.id + "').Flows().Sort().Dedup()";
      return "";
    },

    currentNodeMetadata: function() {
      if (!this.currentNode) return {};
      return this.extractMetadata(this.currentNode.metadata, ['LastMetric', 'Statistics']);
    },

    currentNodeStats: function() {
      if (!this.currentNode) return {};
      return this.extractMetadata(this.currentNode.metadata.Statistics);
    },

    currentNodeLastStats: function() {
      if (!this.currentNode) return {};
      var s = this.extractMetadata(this.currentNode.metadata.LastMetric);
      ['Start', 'Last'].forEach(function(k) {
        if (s[k]) {
          s[k] = new Date(s[k]).toLocaleTimeString();
        }
      });
      return s;
    },

  },

  methods: {

    unwatch: function() {
      clearTimeout(this.timeId);
      this.timeId = null;
    },

    onNodeSelected: function(d) {

      this.$store.commit('selected', d);
    },

    zoomIn: function() {
      this.layout.zoomIn();
    },

    zoomOut: function() {
      this.layout.zoomOut();
    },

    zoomFit: function() {
      this.layout.zoomFit();
    },

    topologyFilterClear: function () {
      this.topologyFilter = '';
     },

    topologyHighlightClear: function () {
      this.topologyHighlight = '';
     },

    endsWith: function (str, suffix) {
       return str.indexOf(suffix, str.length - suffix.length) !== -1;
    },

    topologyFilterQuery: function() {
        if ($("#topology-filter").val() == '' || this.endsWith(this.topologyFilter, ")")) {
            this.$store.commit('topologyFilter', this.topologyFilter + ".SubGraph()");
            this.syncTopo();
        }
    },

    highlightSelectedNodes: function(gremlinExpr, bool) {
      var self = this;

      this.$topologyQuery(gremlinExpr, bool)
        .then(function(nodes) {
          nodes.forEach(function(n) {
            for (var i in n.Nodes) {
                var myNode = n.Nodes[i];
                if (bool) {
                    self.layout.highlightNodeID(myNode.ID);
                } else {
                    self.layout.unhighlightNodeID(myNode.ID);
                }
            }
          });
        });
    },

    topologyHighlightQuery: function() {
        if ($("#topology-highlight").val() == '' || this.endsWith(this.topologyHighlight, ")")) {
            var prevGremlinExpr = this.$store.getters.currTopologyHighlightExpr;
            this.highlightSelectedNodes(prevGremlinExpr, false);
            var newGremlinExpr = this.topologyHighlight + ".SubGraph()"
            this.$store.commit('topologyHighlight', newGremlinExpr);
            this.highlightSelectedNodes(newGremlinExpr, true);
        }
    },

    collapse: function() {
      this.collapsed = true;
      this.layout.collapse(this.collapsed);
    },

    toggleExpandAll: function(node) {
      this.layout.toggleExpandAll(node);
    },

    toggleCollapseByLevel: function(collapse) {
      this.layout.toggleCollapseByLevel(collapse);
    },

    extractMetadata: function(metadata, exclude) {
      var mdata = {};
      for (var key in metadata) {
        if (exclude && exclude.indexOf(key) >= 0) continue;
        mdata[key] = metadata[key];
      }
      return mdata;
    },

  },

};

var Group = function(owner, type) {
  this.id = owner.id;
  this.owner = owner;
  this.members = {};
  this.parent = null;
  this.children = {};
  this.type = type;
};

Group.prototype = {

  setParent: function(parent) {
    this.parent = parent;
  },

  addMember: function(node) {
    this.members[node.id] = node;
  },

  delMember: function(node) {
    delete this.members[node.id];
  },

};

var Node = function(id, host, metadata) {
  this.id = id;
  this.host = host;
  this.metadata = metadata || {};
  this.edges = {};
  this.group = null;

  this.edge2Parent = null;
};

Node.prototype = {

  isGroupOwner: function(type) {
    return this.group && this.group.owner === this && (!type || type === this.group.type);
  },

  isCaptureOn: function() {
    return "Capture/id" in this.metadata;
  },

  isCaptureAllowed: function() {
    var allowedTypes = ["device", "veth", "ovsbridge", "geneve", "vlan", "bond",
                        "internal", "tun", "bridge", "vxlan", "gre", "gretap", "dpdkport"];
    return allowedTypes.indexOf(this.metadata.Type) >= 0;
  },

};

var Edge = function(id, host, metadata, source, target) {
  this.id = id;
  this.host = host;
  this.source = source;
  this.target = target;
  this.metadata = metadata || {};

  source.edges[id] = this;
  target.edges[id] = this;
};

var Graph = function(websocket) {
  this.websocket = websocket;

  this.nodes = {};
  this.edges = {};
  this.groups = {};

  this.handlers = [];

  this.synced = false;
  this.live = true;

  this.websocket.addConnectHandler(this.syncRequest.bind(this));
  this.websocket.addDisconnectHandler(this.invalidate.bind(this));
  this.websocket.addMsgHandler('Graph', this.processGraphMessage.bind(this));
};

Graph.prototype = {

  addHandler: function(handler) {
    this.handlers.push(handler);
  },

  removeHandler: function(handler) {
    var index = this.handlers.indexOf(handler);
    if (index > -1) {
      this.handlers.splice(index, 1);
    }
  },

  notifyHandlers: function(ev, v1, v2) {
    var i, h;
    for (i = this.handlers.length - 1; i >= 0; i--) {
      h = this.handlers[i];
      try {
        var callback = h["on"+firstUppercase(ev)];
        if (callback) {
          callback.bind(h)(v1, v2);
        }
      } catch(e) {
        console.log(e);
      }
    }
  },

  addGroup: function(owner, type) {
    var group = new Group(owner, type);
    this.groups[owner.id] = group;

    this.notifyHandlers('groupAdded', group);

    return group;
  },

  addNode: function(id, host, metadata) {
    var node = new Node(id, host, metadata);
    this.nodes[id] = node;

    this.notifyHandlers('nodeAdded', node);
  },

  updateNode: function(id, metadata) {
    this.nodes[id].metadata = metadata;

    this.notifyHandlers('nodeUpdated', this.nodes[id]);
  },

  delNode: function(node) {
    for (var i in node.edges) {
      this.delEdge(this.edges[i]);
    }

    // remove if member of a group
    if (node.group && this.groups[node.group.owner.id]) {
      this.groups[node.group.owner.id].delMember(node);
    }

    if (node.group && node.group.owner === node) {
      this.delGroup(node.group);
    }

    delete this.nodes[node.id];

    if (this.synced && store.state.currentNode && store.state.currentNode.id == node.id) {
      store.commit('unselected');
    }

    this.notifyHandlers('nodeDeleted', node);
  },

  getNeighbor: function(node, type) {
    for (var id in node.edges) {
      var edge = node.edges[id];
      if (edge.source === node && edge.target.metadata.Type === type) return edge.source;
      if (edge.target === node && edge.source.metadata.Type === type) return edge.target;
    }
    return undefined;
  },

  getTargets: function(node) {
    var targets = [];

    for (var i in node.edges) {
      var e = node.edges[i];
      if (e.source === node)
         targets.push(e.target);
      }
    return targets;
  },

  delGroup: function(group) {
    if (group.parent) delete group.parent.children[group.id];

    var members = Object.values(group.members);
    for (var i = members.length - 1; i >= 0; i--) {
      delete members[i];
    }
    group.members = [];

    delete this.groups[group.id];

    this.notifyHandlers('groupDeleted', group);
  },

  addGroupMember: function(group, node) {
    group.addMember(node);
    this.notifyHandlers('groupMemberAdded', group, node);
  },

  delGroupMember: function(group, node) {
    group.delMember(node);
    this.notifyHandlers('groupMemberDeleted', group, node);
  },

  setParentGroup: function(group, parent) {
    if (group.parent) delete group.parent.children[group.id];

    group.setParent(parent);
    parent.children[group.id] = group;
    this.notifyHandlers('parentSet', group);
  },

  addEdge: function(id, host, metadata, source, target) {
    var self = this;

    var edge = new Edge(id, host, metadata, source, target);
    this.edges[id] = edge;

    this.notifyHandlers('edgeAdded', edge);

    // compute group
    if (edge.metadata.RelationType === "ownership" || edge.metadata.Type === "vlan") {
      var group = this.groups[source.id];
      if (!group) {
        var type = edge.metadata.RelationType === "ownership" ? "ownership" : "interface";


        group = this.addGroup(source, type);

        if (source.group) {
          this.delGroupMember(source.group, source);
          this.setParentGroup(group, source.group);
        }
        source.group = group;
        this.addGroupMember(group, source);
      }

      // target is itself is a group then set the source as its parent
      var tg = this.groups[target.id];
      if (tg && tg.parent != group) {
        this.delGroupMember(group, target);
        this.setParentGroup(tg, group);
      }

      // do not change the group of the target as it could be a group owner
      if (!target.isGroupOwner()) {
        target.group = group;
        this.addGroupMember(group, target);
      }
    }
  },

  updateEdge: function(id, metadata) {
    if (id in this.edges) {
      this.edges[id].metadata = metadata;
    }
  },

  delEdge: function(edge) {
    if (edge.metadata.RelationType === "ownership" || edge.metadata.Type === "vlan") {
      var group = edge.source.group;
      if (group) {
        this.delGroupMember(group, edge.target);

        if (Object.values(group.members).length === 1 && group.type === "interface") {
          this.delGroup(group);
        }
      }
    }

    delete edge.source.edges[edge.id];
    delete edge.target.edges[edge.id];
    delete this.edges[edge.id];

    this.notifyHandlers('edgeDeleted', edge);
  },

  invalidate: function() {
    this.synced = false;
  },

  init: function(g) {
    var n, e, i;
    for (i in g.Nodes) {
      n = g.Nodes[i];
      this.addNode(n.ID, n.Host, n.Metadata || {});
    }

    // add first ownership link to respect the original order
    for (i in g.Edges) {
      e = g.Edges[i];
      if (e.Metadata.RelationType === "ownership") {
        this.addEdge(e.ID, e.Host, e.Metadata || {}, this.nodes[e.Parent], this.nodes[e.Child]);
      }
    }

    for (i in g.Edges) {
      e = g.Edges[i];
      if (e.Metadata.RelationType !== "ownership") {
        this.addEdge(e.ID, e.Host, e.Metadata || {}, this.nodes[e.Parent], this.nodes[e.Child]);
      }
    }
  },

  initFromSyncMessage: function(msg) {
    this.notifyHandlers('preInit');
    this.synced = false;

    this.clear();

    if (msg.Status != 200) {
      $.notify({
        message: 'Unable to init topology'
      },{
        type: 'danger'
      });
      return;
    }

    this.init(msg.Obj);

    this.synced = true;
    this.notifyHandlers('postInit');
  },

  hostGraphDeleted: function(host) {
    var n, e, i;
    for (i in this.edges) {
      e = this.edges[i];
      if (e.host === host) {
        this.delEdge(e);
      }
    }

    for (i in this.nodes) {
      n = this.nodes[i];
      if (n.host === host) {
        this.delNode(n);
      }
    }
  },

  sync: function(msg) {
    this.init(msg.Obj);
  },

  syncRequest: function(t) {
    if (t && t === store.state.time) {
      return;
    }
    var obj = {};
    if (t) {
      this.live = false;
      obj.Time = t;
      store.commit('time', t);
    } else {
      this.live = true;
      store.commit('time', 0);
    }

    obj.GremlinFilter=store.state.topologyFilter;
    var msg = {"Namespace": "Graph", "Type": "SyncRequest", "Obj": obj};
    this.websocket.send(msg);
  },

  processGraphMessage: function(msg) {
    if (msg.Type != "SyncReply" && (!this.live || !this.synced) ) {
      console.log("Skipping message " + msg.Type);
      return;
    }

    var node, edge;
    switch(msg.Type) {
      case "SyncReply":
        this.initFromSyncMessage(msg);
        break;

      case "Sync":
        this.sync(msg);
        break;

      case "HostGraphDeleted":
        this.hostGraphDeleted(msg.Obj);
        break;

      case "NodeUpdated":
        this.updateNode(msg.Obj.ID, msg.Obj.Metadata);
        break;

      case "NodeAdded":
        node = this.nodes[msg.Obj.ID];
        if (!node) {
          this.addNode(msg.Obj.ID, msg.Obj.Host, msg.Obj.Metadata || {});
        }
        break;

      case "NodeDeleted":
        node = this.nodes[msg.Obj.ID];
        if (node) {
          this.delNode(node);
        }
        break;

      case "EdgeUpdated":
        this.updateEdge(msg.Obj.ID, msg.Obj.Metadata);
        break;

      case "EdgeAdded":
        edge = this.edges[msg.Obj.ID];
        if (!edge) {
          var parent = this.nodes[msg.Obj.Parent];
          var child = this.nodes[msg.Obj.Child];

          this.addEdge(msg.Obj.ID, msg.Obj.Host, msg.Obj.Metadata || {}, parent, child);
        }
        break;

      case "EdgeDeleted":
        edge = this.edges[msg.Obj.ID];
        if (edge) {
          this.delEdge(edge);
        }
        break;
    }
  },

  clear: function() {
    var id;
    for (id in this.groups) {
      this.delGroup(this.groups[id]);
    }

    for (id in this.edges) {
      this.delEdge(this.edges[id]);
    }

    for (id in this.nodes) {
      this.delNode(this.nodes[id]);
    }
  },

};
