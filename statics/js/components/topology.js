/* jshint multistr: true */

var TopologyComponent = {

  name: 'topology',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div class="topology">\
      <div class="col-sm-7 fill content">\
        <div class="topology-d3">\
          <div class="topology-legend">\
            <strong>Topology view</strong></br>\
            <span v-if="currTopologyFilter">{{currTopologyFilter}}</span>\
            <span v-else>Full</span>\
            <div v-if="topologyHumanTimeContext">{{topologyHumanTimeContext}}</div>\
            <span v-else>Live</span>\
          </div>\
        </div>\
        <div id="topology-options">\
          <div id="topology-options-panel">\
            <div v-if="history" class="form-group input-sm">\
              <div class="form-group row">\
                <label class="control-label" for="topology-datepicker">Date/Time</label>\
                <div class="input">\
                  <div class="switch switch--vertical">\
                    <input id="radio-live" name="topology-mode" type="radio" value="live" v-model="topologyMode" checked="checked"/>\
                    <label for="radio-live">Live</label>\
                    <input id="radio-history" name="topology-mode" type="radio" value="history" v-model="topologyMode"/>\
                    <label for="radio-history">History</label><span class="toggle-outside">\
                    <span class="toggle-inside"></span></span>\
                  </div>\
                  <datepicker id="topology-datepicker" :calendar-class="\'topology-datepicker\'" \
                    :format="\'dd/MM/yyyy\'" placeholder="dd/MM/yyyy" \
                    v-on:opened="topologyOptionsExpanded" \
                    v-on:closed="topologyOptionsCollapsed" \
                    v-model="topologyDate" input-class="input-sm form-control"\
                    :disabled-picker="topologyMode === \'live\'"></datepicker> \
                  <input id="topology-timepicker" placeholder="HH:mm:ss" \
                    v-model="topologyTime" class="input-sm form-control" \
                    :disabled="topologyMode === \'live\'" style="margin-left: 10px" \
                    @keyup.enter="topologyTimeTravel"></input>\
                  <button type="button" class="btn btn-primary" \
                          :disabled="topologyMode === \'live\'" @click="topologyTimeTravel"> \
                    <span class="glyphicon glyphicon-time" aria-hidden="true"></span>\
                  </button>\
                </div>\
              </div>\
            </div>\
            <div class="form-group input-sm">\
                <div class="form-group row">\
                  <label class="control-label" for="topology-filter">Filter</label>\
                  <div class="input">\
                    <input list="topology-filter-list" placeholder="e.g. g.V().Has(,)" \
                    id="topology-filter" type="text" style="width: 400px" \
                    v-model="topologyFilter" @keyup.enter="topologyFilterQuery" \
                    v-on:input="onFilterDatalistSelect" \
                    class="input-sm form-control"></input>\
                    <span class="clear-btn" @click.stop="topologyFilterClear">&times;</span>\
                    <datalist id="topology-filter-list" class="topology-filter-list">\
                    </datalist>\
                  </div>\
                </div>\
            </div>\
            <div class="form-group input-sm">\
                <div class="form-group row">\
                  <label class="control-label" for="topology-filter">Highlight</label>\
                  <div class="input">\
                    <input list="topology-highlight-list" placeholder="e.g. g.V().Has(,)" \
                    id="topology-highlight" type="text" style="width: 400px" \
                    v-model="topologyHighlight" @keyup.enter="topologyHighlightQuery" \
                    v-on:input="onFilterDatalistSelect" \
                    class="input-sm form-control"></input>\
                    <span class="clear-btn" @click.stop="topologyHighlightClear">&times;</span>\
                    <datalist id="topology-highlight-list" class="topology-filter-list">\
                    </datalist>\
                  </div>\
                </div>\
            </div>\
          </div>\
          <div style="margin-top: 10px">\
            <div class="trigger">\
              <button @mouseenter="showTopologyOptions" @click="hideTopologyOptions">\
                <span class="glyphicon glyphicon-align-justify" aria-hidden="true"></span>\
              </button>\
            </div>\
          </div>\
        </div>\
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
            <capture-form v-if="topologyMode ===  \'live\'"></capture-form>\
          </tab-pane>\
          <tab-pane title="Generator" v-if="topologyMode ===  \'live\'">\
            <inject-form></inject-form>\
          </tab-pane>\
          <tab-pane title="Flows">\
            <flow-table-control></flow-table-control>\
          </tab-pane>\
        </tabs>\
        <transition name="slide" mode="out-in">\
          <div class="left-panel" v-if="currentNode || currentEdge">\
            <div v-if="currentNode">\
              <span v-if="time" class="label center-block node-time">\
                Interface state at {{timeHuman}}\
              </span>\
              <h1>Node Metadata<span class="pull-right">(id: {{currentNode.id}})\
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
            <div v-if="currentEdge">\
              <h1> Edge Metadata<span class="pull-right">(id: {{currentEdge.id}})</h1>\
              <div id="metadata-panel" class="sub-left-panel">\
                <object-detail :object="currentEdge.metadata"></object-detail>\
              </div>\
            </div>\
          </div>\
        </transition>\
      </div>\
    </div>\
  ',

  data: function() {
    return {
      topologyTimeContext: 0,
      topologyTime: null,
      topologyDate: '',
      collapsed: false,
      topologyFilter: "",
      currTopologyFilter: "",
      topologyHighlight: "",
      topologyMode: "live",
      topologyHumanTimeContext: ""
    };
  },

  mounted: function() {
    var self = this;

    // run d3 layout
    this.graph = new Graph(websocket, function(err) {
      this.$error({
        message: err
      });
    }.bind(this));

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

    this.setFilterFromConfig();
  },

  beforeDestroy: function() {
    this.$store.commit('unselected');
    this.$store.commit('edgeUnselected');
    this.unwatch();
  },

  watch: {

    topologyMode: function (val) {
      if (val === 'live') {
        this.topologyTimeTravelClear();
      } else {
        this.graph.pauseLive();

        var dt = new Date();
        this.topologyTimeContext = dt.getTime();
      }
    },
    topologyTimeContext: function(val) {
      if (!this.topologyTimeContext) {
        this.topologyHumanTimeContext = '';
      } else {
        var dt = new Date(this.topologyTimeContext);
        this.topologyHumanTimeContext =  ' at ' + dt.toLocaleString();
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

    currentEdge: function() {
      return this.$store.state.currentEdge;
    },

    currentNodeFlowsQuery: function() {
      if (this.currentNode && this.currentNode.isCaptureAllowed())
        return "G.V('" + this.currentNode.id + "').Flows().Dedup().Sort()";
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

    onEdgeSelected: function(d) {
      this.$store.commit('edgeSelected', d);
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
      this.topologyFilterQuery();
     },

    topologyHighlightClear: function () {
      this.topologyHighlight = '';
      this.topologyHighlightQuery();
     },

    endsWith: function (str, suffix) {
       return str.indexOf(suffix, str.length - suffix.length) !== -1;
    },

    setFilterFromConfig: function() {
      return $.when(this.$getConfigValue('analyzer.topology_filter'))
       .then(function(filter) {
         if (!filter) return;
         var options = $(".topology-filter-list");
         $.each(filter, function(key, value) {
           options.append($("<option/>").text(key).val(value));
         });
       });
    },

    onFilterDatalistSelect: function(e) {
      var val = e.target.value;
      var listId = e.target.getAttribute("list");
      var opts = document.getElementById(listId).childNodes;
      for (var i = 0; i < opts.length; i++) {
        if (opts[i].value === val) {
          if (e.target.id === "topology-filter") {
            this.topologyFilterQuery();
          } else if (e.target.id === "topology-highlight") {
            this.topologyHighlightQuery();
          }
        }
      }
    },

    topologyTimeTravelClear: function() {
      this.topologyDate = '';
      this.topologyTime = '';
      this.topologyTimeContext = 0;
      this.$store.commit('topologyTimeContext', this.topologyTimeContext);
      this.syncTopo(this.topologyTimeContext, this.topologyFilter);
    },

    topologyTimeTravel: function() {
      var time = new Date(), live = true;
      if (this.topologyDate) {
        time = new Date(this.topologyDate);
        live = false;
      }

      if (this.topologyTime) {
        var tl = this.topologyTime.split(':');
        time.setHours(tl[0]);
        if (tl.length > 1) {
          time.setMinutes(tl[1]);
        }
        if (tl.length > 2) {
          time.setSeconds(tl[2]);
        }
        live = false;
      }

      if (live) {
        this.topologyTimeContext = '';
      } else {
        this.topologyTimeContext = time.getTime();
      }

      this.$store.commit('topologyTimeContext', this.topologyTimeContext);
      this.syncTopo(this.topologyTimeContext, this.topologyFilter);
    },

    topologyFilterQuery: function() {
      var self = this;

      if (!this.topologyFilter || this.endsWith(this.topologyFilter, ")")) {
        var filter = this.topologyFilter;
        $("#topology-filter-list").find('option').filter(function() {
          if (this.value === self.topologyFilter) {
            filter = this.text;
          }
        });
        this.currTopologyFilter = filter;

        this.$store.commit('topologyFilter', this.topologyFilter);
        this.syncTopo(this.topologyTimeContext, this.topologyFilter);
      }
    },

    showTopologyOptions: function() {
      if (this.topologyOptionsTimeoutID) {
        clearTimeout(this.topologyOptionsTimeoutID);
      }
      $("#topology-options").css("left", "0");
    },

    hideTopologyOptions: function() {
      this.topologyOptionsTimeoutID = setTimeout(function() {
        $('input').blur();
        $("#topology-options").css("left", "-543px");
      }, 100);
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
      if (!this.topologyHighlight || this.endsWith(this.topologyHighlight, ")")) {
        var prevGremlinExpr = this.$store.getters.currTopologyHighlightExpr;
        this.highlightSelectedNodes(prevGremlinExpr, false);

        if (this.topologyHighlight) {
          var expr = this.topologyHighlight;
          if (this.topologyTimeContext !== 0) {
            expr = expr.replace(/g/i, "g.at(" + this.topologyTimeContext + ")");
          }

          var newGremlinExpr = expr + ".SubGraph()";
          this.$store.commit('topologyHighlight', newGremlinExpr);
          this.highlightSelectedNodes(newGremlinExpr, true);
        }
      }
    },

    topologyOptionsExpanded: function() {
      if (this.topologyOptionsExpandedTimeoutID) {
        clearTimeout(this.topologyOptionsExpandedTimeoutID);
      }
      $("#topology-options").addClass("topology-options-expanded");
    },

    topologyOptionsCollapsed: function() {
      // wait 2 sec
      this.topologyOptionsExpandedTimeoutID = setTimeout(function() {
        $("#topology-options").removeClass("topology-options-expanded");
      }, 2000);
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

var Graph = function(websocket, onErrorCallback) {
  this.websocket = websocket;
  this.onErrorCallback = onErrorCallback;

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

    store.commit('edgeUnselected');
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
        if (!this.nodes[e.Parent] || !this.nodes[e.Child])
          continue;

        this.addEdge(e.ID, e.Host, e.Metadata || {}, this.nodes[e.Parent], this.nodes[e.Child]);
      }
    }

    for (i in g.Edges) {
      e = g.Edges[i];
      if (e.Metadata.RelationType !== "ownership") {
        if (!this.nodes[e.Parent] || !this.nodes[e.Child])
          continue;

        this.addEdge(e.ID, e.Host, e.Metadata || {}, this.nodes[e.Parent], this.nodes[e.Child]);
      }
    }
  },

  initFromSyncMessage: function(msg) {
    this.notifyHandlers('preInit');
    this.synced = false;

    this.clear();

    if (msg.Status != 200) {
      if (this.onErrorCallback) {
        this.onErrorCallback('Unable to get the topology, please check the filter');
      }
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

  pauseLive: function() {
    this.live = false;
  },

  syncRequest: function(time, filter) {
    var obj = {};
    if (time) {
      this.live = false;
      obj.Time = time;
    } else {
      this.live = true;
    }

    if (filter) {
      obj.GremlinFilter = filter + ".SubGraph()";
    }
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
