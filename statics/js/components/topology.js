/* jshint multistr: true */

var TopologyComponent = {

  name: 'topology',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div class="topology">\
      <div class="col-sm-7 fill content">\
        <div class="topology-d3"></div>\
        <slider v-if="history" class="slider" :min="timeRange[0]" :max="timeRange[1]" \
                v-model="time" :info="topologyTimeHuman"></slider>\
        <div class="topology-controls">\
          <button type="button" class="btn btn-primary"\
                  title="Zoom In" @click="zoomIn">\
            <span class="glyphicon glyphicon-zoom-in" aria-hidden="true"></span>\
          </button>\
          <button type="button" class="btn btn-primary"\
                  title="Zoom Out" @click="zoomOut">\
            <span class="glyphicon glyphicon-zoom-out" aria-hidden="true"></span>\
          </button>\
          <button type="button" class="btn btn-primary" \
                  title="Reset" @click="zoomReset">Reset</button>\
          <button type="button" class="btn btn-primary" \
                  @click="collapseAll">{{collapsed ? "Expand" : "Collapse"}}</button>\
        </div>\
      </div>\
      <div class="col-sm-5 fill sidebar">\
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
            <h1>Metadata<span class="pull-right">(ID: {{currentNode.ID}})</span></h1>\
            <div class="sub-left-panel">\
              <object-detail :object="currentNodeMetadata"></object-detail>\
            </div>\
            <div v-show="Object.keys(currentNodeStats).length">\
              <h1>Interface metrics</h1>\
              <statistics-table :object="currentNodeStats"></statistics-table>\
            </div>\
            <div v-show="Object.keys(currentNodeLastStats).length && time === 0">\
              <h1>Last metrics</h1>\
              <statistics-table :object="currentNodeLastStats"></statistics-table>\
            </div>\
            <div v-if="isAnalyzer && currentNodeFlowsQuery">\
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
    };
  },

  mounted: function() {
    var self = this;
    // run d3 layout
    this.layout = new TopologyLayout(this, ".topology-d3");

    this.syncTopo = debounce(this.layout.SyncRequest.bind(this.layout), 300);

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
        self.layout.SetNodeClass(mutation.payload, "highlighted", true);
      else if (mutation.type == "unhighlight")
        self.layout.SetNodeClass(mutation.payload, "highlighted", false);
    });

    // trigered when a node is selected
    this.unwatch = this.$store.watch(
      function() {
        return self.$store.state.currentNode;
      },
      function(newNode, oldNode) {
        if (oldNode) {
          var old = d3.select('#node-' + oldNode.ID);
          if (!old.empty()) {
            old.classed('active', false);
            old.select('circle').attr('r', parseInt(old.select('circle').attr('r')) - 3);
          }
        }
        if (newNode) {
          var current = d3.select('#node-' + newNode.ID);
          current.classed('active', true);
          current.select('circle').attr('r', parseInt(current.select('circle').attr('r')) + 3);
        }
      }
    );
  },

  beforeDestroy: function() {
    this.$store.commit('unselected');
    this.unwatch();
  },

  watch: {

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
      if (this.currentNode && this.currentNode.IsCaptureAllowed())
        return "G.V('" + this.currentNode.ID + "').Flows().Sort().Dedup()";
      return "";
    },

    currentNodeMetadata: function() {
      return this.extractMetadata(this.currentNode.Metadata, null, ['LastMetric', 'Statistics', '__']);
    },

    currentNodeStats: function() {
      return this.currentNode.Metadata.Statistics || {};
    },

    currentNodeLastStats: function() {
      var s = this.currentNode.Metadata.LastMetric || {};
      ['Start', 'Last'].forEach(function(k) {
        if (s[k]) {
          s[k] = new Date(s[k]).toLocaleTimeString();
        }
      });
      return s;
    },

  },

  methods: {

    rescale: function(factor) {
      var width = this.layout.width,
          height = this.layout.height,
          translate = this.layout.zoom.translate(),
          newScale = this.layout.zoom.scale() * factor,
          newTranslate = [width / 2 + (translate[0] - width / 2) * factor,
                          height / 2 + (translate[1] - height / 2) * factor];
      this.layout.zoom
        .scale(newScale)
        .translate(newTranslate)
        .event(this.layout.view);
    },

    zoomIn: function() {
      this.rescale(1.1);
    },

    zoomOut: function() {
      this.rescale(0.9);
    },

    zoomReset: function() {
      this.layout.zoom
        .scale(1)
        .translate([0, 0])
        .event(this.layout.view);
    },

    collapseAll: function() {
      this.collapsed = !this.collapsed;
      var nodes = this.layout.nodes;
      for (var i in nodes) {
        if (nodes[i].Metadata.Type !== "host") {
          continue;
        }
        if (nodes[i].Collapsed !== this.collapsed) {
          this.layout.CollapseHost(nodes[i]);
          this.layout.Redraw();
        }
      }
    },

    extractMetadata: function(metadata, namespace, exclude) {
      return Object.getOwnPropertyNames(metadata).reduce(function(mdata, key) {
        var use = true;
        if (namespace && key.search(namespace) === -1) {
          use = false;
        }
        if (exclude) {
          exclude.forEach(function(e) {
            if (key.search(e) !== -1) {
              use = false;
            }
          });
        }
        if (use) {
          mdata[key] = metadata[key];
        }
        return mdata;
      }, {});
    },

  },

};

var hostImg = 'statics/img/host.png';
var switchImg = 'statics/img/switch.png';
var portImg = 'statics/img/port.png';
var intfImg = 'statics/img/intf.png';
var vethImg = 'statics/img/veth.png';
var nsImg = 'statics/img/ns.png';
var bridgeImg = 'statics/img/bridge.png';
var dockerImg = 'statics/img/docker.png';
var neutronImg = 'statics/img/openstack.png';
var minusImg = 'statics/img/minus-outline-16.png';
var plusImg = 'statics/img/plus-16.png';
var probeIndicatorImg = 'statics/img/media-record.png';
var pinIndicatorImg = 'statics/img/pin.png';

var Group = function(ID, type) {
  this.ID = ID;
  this.Type = type;
  this.Nodes = {};
  this.Hulls = [];
};

var Node = function(ID) {
  this.ID = ID;
  this.Host = '';
  this.Metadata = {};
  this.Edges = {};
  this.Visible = true;
  this.Collapsed = false;
  this.Highlighted = false;
  this.Group = '';
};

Node.prototype = {

  IsCaptureOn: function() {
    return "Capture" in this.Metadata && "ID" in this.Metadata.Capture;
  },

  IsCaptureAllowed: function() {
    var allowedTypes = ["device", "veth", "ovsbridge",
                        "internal", "tun", "bridge"];
    return allowedTypes.indexOf(this.Metadata.Type) >= 0;
  }

};

var Edge = function(ID) {
  this.ID = ID;
  this.Host = '';
  this.Parent = '';
  this.Child = '';
  this.Metadata = {};
  this.Visible = true;
};

var Graph = function(ID) {
  this.Nodes = {};
  this.Edges = {};
  this.Groups = {};
};

Graph.prototype.NewNode = function(ID, host) {
  var node = new Node(ID);
  node.Graph = this;
  node.Host = host;

  this.Nodes[ID] = node;

  return node;
};

Graph.prototype.GetNode = function(ID) {
  return this.Nodes[ID];
};

Graph.prototype.GetNeighbors = function(node) {
  var neighbors = [];

  for (var i in node.Edges) {
    neighbors.push(node.Edges[i]);
  }

  return neighbors;
};

Graph.prototype.GetChildren = function(node) {
  var children = [];

  for (var i in node.Edges) {
    var e = node.Edges[i];
    if (e.Parent == node)
      children.push(e.Child);
  }

  return children;
};

Graph.prototype.GetParents = function(node) {
  var parents = [];

  for (var i in node.Edges) {
    var e = node.Edges[i];
    if (e.Child == node)
      parents.push(e.Child);
  }

  return parents;
};

Graph.prototype.GetEdge = function(ID) {
  return this.Edges[ID];
};

Graph.prototype.NewEdge = function(ID, parent, child, host) {
  var edge = new Edge(ID);
  edge.Parent = parent;
  edge.Child = child;
  edge.Graph = this;
  edge.Host = host;

  this.Edges[ID] = edge;

  parent.Edges[ID] = edge;
  child.Edges[ID] = edge;

  return edge;
};

Graph.prototype.DelNode = function(node) {
  for (var i in node.Edges) {
    this.DelEdge(this.Edges[i]);
  }

  delete this.Nodes[node.ID];
};

Graph.prototype.DelEdge = function(edge) {
  delete edge.Parent.Edges[edge.ID];
  delete edge.Child.Edges[edge.ID];
  delete this.Edges[edge.ID];
};

Graph.prototype.InitFromSyncMessage = function(msg) {
  var g = msg.Obj;

  var i;
  for (i in g.Nodes || []) {
    var n = g.Nodes[i];

    var node = this.NewNode(n.ID);
    if ("Metadata" in n)
      node.Metadata = n.Metadata;
    node.Host = n.Host;
  }

  for (i in g.Edges || []) {
    var e = g.Edges[i];

    var parent = this.GetNode(e.Parent);
    var child = this.GetNode(e.Child);

    if (!parent || !child)
      continue;

    var edge = this.NewEdge(e.ID, parent, child);

    if ("Metadata" in e)
      edge.Metadata = e.Metadata;
    edge.Host = e.Host;
  }
};

var TopologyLayout = function(vm, selector) {
  var self = this;
  this.vm = vm;
  this.graph = new Graph();
  this.selector = selector;
  this.elements = {};
  this.groups = {};
  this.synced = false;
  this.lscachetimeout = 60 * 24 * 7;
  this.keeplayout = false;
  this.alerts = {};

  setInterval(function() {
    // keep track of position once one drag occured
    if (self.keeplayout) {
      for (var i in self.nodes) {
        var node = self.nodes[i];
        lscache.set(self.nodes[i].Metadata.TID, {x: node.x, y: node.y, fixed: node.fixed}, self.lscachetimeout);
      }
    }
  }, 30000);

  websocket.addConnectHandler(this.SyncRequest.bind(this));
  websocket.addDisconnectHandler(this.Invalidate.bind(this));
  websocket.addMsgHandler('Graph', this.ProcessGraphMessage.bind(this));
  websocket.addMsgHandler('Alert', this.ProcessAlertMessage.bind(this));

  this.width = $(selector).width() - 8;
  this.height = $(selector).height();

  this.svg = d3.select(selector).append("svg")
    .attr("width", this.width)
    .attr("height", this.height)
    .attr("y", 60)
    .attr('viewBox', -this.width/2 + ' ' + -this.height/2 + ' ' + this.width * 2 + ' ' + this.height * 2)
    .attr('preserveAspectRatio', 'xMidYMid meet');

  var _this = this;

  this.zoom = d3.behavior.zoom()
    .on("zoom", function() { _this.Rescale(); });

  this.force = d3.layout.force()
    .size([this.width, this.height])
    .charge(-400)
    .gravity(0.02)
    .linkStrength(0.5)
    .friction(0.8)
    .linkDistance(function(d, i) {
      return _this.LinkDistance(d, i);
    })
    .on("tick", function(e) {
      _this.Tick(e);
    });

  this.view = this.svg.append('g');

  this.svg.call(this.zoom)
    .on("dblclick.zoom", null);

  this.drag = this.force.stop().drag()
    .on("dragstart", function(d) {
      _this.keeplayout = true;
      d3.event.sourceEvent.stopPropagation();
    });

  this.groupsG = this.view.append("g")
    .attr("class", "groups")
    .on("click", function() {
      d3.event.preventDefault();
    });

  this.deferredActions = [];
  this.links = this.force.links();
  this.nodes = this.force.nodes();

  var linksG = this.view.append("g").attr("class", "links");
  this.link = linksG.selectAll(".link");

  var nodesG = this.view.append("g").attr("class", "nodes");
  this.node = nodesG.selectAll(".node");

  // un-comment to debug relationships
  /*this.svg.append("svg:defs").selectAll("marker")
    .data(["end"])      // Different link/path types can be defined here
    .enter().append("svg:marker")    // This section adds in the arrows
    .attr("id", String)
    .attr("viewBox", "0 -5 10 10")
    .attr("refX", 25)
    .attr("refY", -1.5)
    .attr("markerWidth", 6)
    .attr("markerHeight", 6)
    .attr("orient", "auto")
    .append("svg:path")
    .attr("d", "M0,-5L10,0L0,5");*/
  this.bandwidth = {source: 'netlink',
    bandwidth_threshold: 'absolute',
    updatePeriod: 3000,
    active: 5,
    warning: 100,
    alert: 1000,
    outstandingAjax: 0};

    this.getBandwidthConfiguration()
      .then(function() {
        self.refreshLinksInterval = setInterval(self.RefreshLinks.bind(self), self.bandwidth.updatePeriod);
      });
};

TopologyLayout.prototype.LinkDistance = function(d, i) {
  var distance = 60;

  if (d.source.Group == d.target.Group) {
    if (d.source.Metadata.Type == "host") {
      for (var property in d.source.Edges)
        distance += 2;
      return distance;
    }
  }

  // local to fabric
  if ((d.source.Metadata.Probe == "fabric" && !d.target.Metadata.Probe) ||
      (!d.source.Metadata.Probe && d.target.Metadata.Probe == "fabric")) {
    return distance + 100;
  }
  return 80;
};

TopologyLayout.prototype.InitFromSyncMessage = function(msg) {
  if (msg.Status != 200) {
    this.vm.$error({message: 'Unable to init topology'});
    return;
  }

  this.graph.InitFromSyncMessage(msg);

  for (var ID in this.graph.Nodes) {
    this.AddNode(this.graph.Nodes[ID]);
  }

  for (var ID in this.graph.Edges)
    this.AddEdge(this.graph.Edges[ID]);

  if (store.state.currentNode) {
    var id = store.state.currentNode.ID;
    if (id in this.elements) {
      store.commit('selected', this.elements[id]);
    } else {
      store.commit('unselected');
    }
  }

};

TopologyLayout.prototype.Invalidate = function() {
  this.synced = false;
};

TopologyLayout.prototype.Clear = function() {
  var ID;

  for (ID in this.graph.Edges)
    this.DelEdge(this.graph.Edges[ID]);

  for (ID in this.graph.Nodes)
    this.DelNode(this.graph.Nodes[ID]);

  for (ID in this.graph.Edges)
    this.graph.DelEdge(this.graph.Edges[ID]);

  for (ID in this.graph.Nodes)
    this.graph.DelNode(this.graph.Nodes[ID]);
};

TopologyLayout.prototype.Rescale = function() {
  var trans = d3.event.translate;
  var scale = d3.event.scale;

  this.view.attr("transform", "translate(" + trans + ")" + " scale(" + scale + ")");
};

TopologyLayout.prototype.SetPosition = function(x, y) {
  this.view.attr("x", x).attr("y", y);
};

TopologyLayout.prototype.SetNodeClass = function(ID, clazz, active) {
  d3.select("#node-" + ID).classed(clazz, active);
};

TopologyLayout.prototype.Hash = function(str) {
  var chars = str.split('');

  var hash = 2342;
  for (var i in chars) {
    var c = chars[i].charCodeAt(0);
    hash = ((c << 5) + hash) + c;
  }

  return hash;
};

TopologyLayout.prototype.AddNode = function(node) {
  if (node.ID in this.elements)
    return;

  this.elements[node.ID] = node;

  // get postion for cache otherwise distribute node on a circle depending on the host
  var data = lscache.get(node.Metadata.TID);
  if (data) {
    node.x = data.x;
    node.y = data.y;
    node.fixed = data.fixed;
  } else {
    var place = this.Hash(node.Host) % 100;
    node.x = Math.cos(place / 100 * 2 * Math.PI) * 500 + this.width / 2 + Math.random();
    node.y = Math.sin(place / 100 * 2 * Math.PI) * 500 + this.height / 2 + Math.random();
  }

  this.nodes.push(node);

  this.Redraw();
};

TopologyLayout.prototype.UpdateNode = function(node, metadata) {
  node.Metadata = metadata;
};

TopologyLayout.prototype.DelNode = function(node) {
  if (this.synced && store.state.currentNode &&
      store.state.currentNode.ID == node.ID) {
    store.commit('unselected');
  }

  if (!(node.ID in this.elements))
    return;

  for (var i in this.nodes) {
    if (this.nodes[i].ID == node.ID) {
      this.nodes.splice(i, 1);
      break;
    }
  }
  delete this.elements[node.ID];

  this.Redraw();
};

TopologyLayout.prototype.AddEdge = function(edge) {
  if (edge.ID in this.elements)
    return;

  this.elements[edge.ID] = edge;

  // ignore layer 3 for now
  if (edge.Metadata.RelationType == "layer3")
    return;

  // specific to link to host
  var i, e, nparents;
  if (edge.Parent.Metadata.Type == "host") {
    if (edge.Child.Metadata.Type == "ovsbridge" ||
        edge.Child.Metadata.Type == "netns")
      return;

    if (edge.Child.Metadata.Type == "bridge" && this.graph.GetNeighbors(edge.Child).length > 1)
      return;

    nparents = this.graph.GetParents(edge.Child).length;
    if (nparents > 2 || (nparents > 1 && this.graph.GetChildren(edge.Child).length !== 0))
      return;
  } else {
    var nodes = [edge.Parent, edge.Child];
    for (var n in nodes) {
      var node = nodes[n];
      for (i in node.Edges) {
        e = node.Edges[i];
        if (e.Parent.Metadata.Type == "host") {

          if (node.Metadata.Type == "bridge" && this.graph.GetNeighbors(node).length > 1) {
            this.DelEdge(e);
            break;
          }

          nparents = this.graph.GetParents(node).length;
          if (nparents > 2 || (nparents > 1 && this.graph.GetChildren(node).length !== 0)) {
            this.DelEdge(e);
            break;
          }
        }
      }
    }
  }

  this.links.push({source: edge.Parent, target: edge.Child, edge: edge});

  this.Redraw();
};

TopologyLayout.prototype.DelEdge = function(edge) {
  if (!(edge.ID in this.elements))
    return;

  for (var i in this.links) {
    if (this.links[i].source.ID == edge.Parent.ID &&
        this.links[i].target.ID == edge.Child.ID) {

      var nodes = [edge.Parent, edge.Child];
      for (var n in nodes) {
        var node = nodes[n];

        if (node.Metadata.Type == "bridge" && this.graph.GetNeighbors(node).length < 2) {
          for (var e in node.Edges) {
            if (node.Edges[e].Parent.Metadata.Type == "host" || node.Edges[e].Child.Metadata.Type == "host") {
              this.AddEdge(node.Edges[e]);
            }
          }
        }
      }

      this.links.splice(i, 1);
    }
  }
  delete this.elements[edge.ID];

  this.Redraw();
};

TopologyLayout.prototype.Tick = function(e) {

  var _this = this;

  this.link.select(".linkpath").attr("d", function(d) { return _this.linkArc(d, false, 0.1);});
  this.link.select(".textpath").attr("d", function(d) { return _this.linkArc(d, d.source.x < d.target.x, 0.2);});

  this.node.attr("cx", function(d) { return d.x; })
  .attr("cy", function(d) { return d.y; });

  this.node.attr("transform", function(d) { return "translate(" + d.x + "," + d.y + ")"; });

  var _this = this;
  if (this.group.length > 0)
    this.group.data(this.Groups()).attr("d", function(d) {
      return _this.DrawCluster(d);
    });
};

TopologyLayout.prototype.linkArc = function(d, leftHand,arcDelta) {
  var start = leftHand ? d.source : d.target,
      end = leftHand ? d.target : d.source,
      dx = end.x - start.x,
      dy = end.y - start.y,
      drx = Math.sqrt(dx * dx + dy * dy) * 1 + arcDelta,
      dry = Math.sqrt(dx * dx + dy * dy) * 1 - arcDelta,
      sweep = leftHand ? 0 : 1;
      return "M" + start.x + "," + start.y + "A" + drx + "," + dry + " 0 0," + sweep + " " + end.x + "," + end.y;
};

TopologyLayout.prototype.CircleSize = function(d) {
  var size;
  switch(d.Metadata.Type) {
    case "host":
      size = 22;
      break;
    case "port":
    case "ovsport":
      size = 18;
      break;
    case "switch":
    case "ovsbridge":
      size = 20;
      break;
    default:
      size = 16;
      break;
  }

  if (store.state.currentNode && store.state.currentNode.ID === d.ID) {
    size += 3;
  }

  return size;
};

TopologyLayout.prototype.GroupClass = function(d) {
  return "group " + d.Type;
};

TopologyLayout.prototype.NodeClass = function(d) {
  var clazz = "node " + d.Metadata.Type;

  if (d.ID in this.alerts)
    clazz += " alert";

  if (d.Metadata.State == "DOWN")
    clazz += " down";

  if (d.Highlighted)
    clazz = "highlighted " + clazz;

  if (store.state.currentNode && store.state.currentNode.ID === d.ID)
    clazz = "active " + clazz;

  return clazz;
};

TopologyLayout.prototype.EdgeClass = function(d) {
  if (d.edge.Metadata.Type == "fabric") {
    if ((d.edge.Parent.Metadata.Probe == "fabric" && !d.edge.Child.Metadata.Probe) ||
      (!d.edge.Parent.Metadata.Probe && d.edge.Child.Metadata.Probe == "fabric")) {
        return "link local2fabric";
      }
  }

  return "link " + (d.edge.Metadata.Type || '')  + " " + (d.edge.Metadata.RelationType || '');
};

TopologyLayout.prototype.EdgeTextClass = function(d) {
  return "linklabel";
};

TopologyLayout.prototype.CircleOpacity = function(d) {
  if (d.Metadata.Type == "netns" && d.Metadata.Manager === null)
    return 0.0;
  return 1.0;
};

TopologyLayout.prototype.EdgeOpacity = function(d) {
  if (d.edge.Visible == false) {
    return 0.0;
  }

  return 1.0;
};

TopologyLayout.prototype.NodeManagerPicto = function(d) {
  switch(d.Metadata.Manager) {
    case "docker":
      return dockerImg;
    case "neutron":
      return neutronImg;
  }
};

TopologyLayout.prototype.NodeManagerStyle = function(d) {
  switch(d.Metadata.Manager) {
    case "docker":
      return "";
    case "neutron":
      return "";
  }

  return "visibility: hidden";
};

TopologyLayout.prototype.NodePicto = function(d) {
  switch(d.Metadata.Type) {
    case "host":
      return hostImg;
    case "port":
    case "ovsport":
      return portImg;
    case "bridge":
      return bridgeImg;
    case "switch":
    case "ovsbridge":
      return switchImg;
    case "netns":
      return nsImg;
    case "veth":
      return vethImg;
    case "bond":
      return portImg;
    case "container":
      return dockerImg;
    default:
      return intfImg;
  }
};

TopologyLayout.prototype.NodeProbeStatePicto = function(d) {
  if (d.IsCaptureOn())
    return probeIndicatorImg;
  return "";
};

TopologyLayout.prototype.NodePinStatePicto = function(d) {
  if (d.fixed)
    return pinIndicatorImg;
  return "";
};

TopologyLayout.prototype.NodeStatePicto = function(d) {
  if (d.Metadata.Type !== "netns" && d.Metadata.Type !== "host")
    return "";

  if (d.Collapsed)
    return plusImg;
  return minusImg;
};

// return the parent for a give node as a node can have mutliple parent
// return the best one. For ex an ovsport is not considered as a parent,
// host node will be a better candiate.
TopologyLayout.prototype.ParentNodeForGroup = function(node) {
  var parent;
  for (var i in node.Edges) {
    var edge = node.Edges[i];
    if (edge.Parent == node)
      continue;

    if (edge.Parent.Metadata.Probe == "fabric")
      continue;

    switch (edge.Parent.Metadata.Type) {
      case "ovsport":
        if (node.Metadata.IfIndex)
          break;
        return edge.Parent;
      case "ovsbridge":
      case "netns":
        return edge.Parent;
      default:
        parent = edge.Parent;
    }
  }

  return parent;
};

TopologyLayout.prototype.AddNodeToGroup = function(ID, type, node, groups) {
  var group = groups[ID] || (groups[ID] = new Group(ID, type));
  if (node.ID in group.Nodes)
    return;

  group.Nodes[node.ID] = node;
  if (node.Group === '')
    node.Group = ID;

  if (isNaN(parseFloat(node.x)))
    return;

  if (!node.Visible)
    return;

  // padding around group path
  var pad = 24;
  if (group.Type == "host" || group.Type == "vm")
    pad = 48;
  if (group.Type == "fabric")
    pad = 60;

  group.Hulls.push([node.x - pad, node.y - pad]);
  group.Hulls.push([node.x - pad, node.y + pad]);
  group.Hulls.push([node.x + pad, node.y - pad]);
  group.Hulls.push([node.x + pad, node.y + pad]);
};

// add node to parent group until parent is of type host
// this means a node can be in multiple group
TopologyLayout.prototype.addNodeToParentGroup = function(parent, node, groups) {
  if (parent) {
    groupID = parent.ID;

    // parent group exist so add node to it
    if (groupID in groups)
      this.AddNodeToGroup(groupID, '', node, groups);

    if (parent.Metadata.Type != "host") {
      parent = this.ParentNodeForGroup(parent);
      this.addNodeToParentGroup(parent, node, groups);
    }
  }
};

TopologyLayout.prototype.UpdateGroups = function() {
  var node;
  var i;

  this.groups = {};

  for (i in this.graph.Nodes) {
    node = this.graph.Nodes[i];

    // present in graph but not in d3
    if (!(node.ID in this.elements))
      continue;

    // reset node group
    node.Group = '';

    var groupID;
    if (node.Metadata.Probe == "fabric") {
      if ("Group" in node.Metadata && node.Metadata.Group !== "") {
        groupID = node.Metadata.Group;
      } else {
        groupID = "fabric";
      }
      this.AddNodeToGroup(groupID, "fabric", node, this.groups);
    } else {
      // these node a group holder
      switch (node.Metadata.Type) {
        case "host":
          if ("InstanceID" in node.Metadata) {
            this.AddNodeToGroup(node.ID, "vm", node, this.groups);
            break;
          }
        case "ovsbridge":
        case "netns":
          this.AddNodeToGroup(node.ID, node.Metadata.Type, node, this.groups);
      }
    }
  }

  // place nodes in groups
  for (i in this.graph.Nodes) {
    node = this.graph.Nodes[i];

    if (!(node.ID in this.elements))
      continue;

    var parent = this.ParentNodeForGroup(node);
    this.addNodeToParentGroup(parent, node, this.groups);
  }
};

TopologyLayout.prototype.Groups = function() {
  var groupArray = [];

  this.UpdateGroups();
  for (var ID in this.groups) {
    groupArray.push({Group: ID, Type: this.groups[ID].Type, path: d3.geom.hull(this.groups[ID].Hulls)});
  }

  return groupArray;
};

TopologyLayout.prototype.DrawCluster = function(d) {
  var curve = d3.svg.line()
  .interpolate("cardinal-closed")
  .tension(0.90);

  return curve(d.path);
};

TopologyLayout.prototype.GetNodeText = function(d) {
  var name = this.graph.GetNode(d.ID).Metadata.Name;
  if (name.length > 10)
    name = name.substr(0, 8) + ".";

  return name;
};

TopologyLayout.prototype.CollapseNetNS = function(node) {
  for (var i in node.Edges) {
    var edge = node.Edges[i];

    if (edge.Child == node)
      continue;

    if (Object.keys(edge.Child.Edges).length == 1) {
      edge.Child.Visible = edge.Child.Visible ? false : true;
      edge.Visible = edge.Visible ? false : true;

      node.Collapsed = edge.Child.Visible ? false : true;
    }
  }
};

TopologyLayout.prototype.CollapseHost = function(hostNode) {
  var fabricNode;
  var isCollapsed = hostNode.Collapsed ? false : true;

  // All nodes in the group
  for (var i in this.nodes) {
    var node = this.nodes[i];

    if (node.Host != hostNode.Host)
      continue;

    if (node == hostNode)
      continue;

    // All edges (connected to all nodes in the group)
    for (var j in node.Edges) {
      var edge = node.Edges[j];

      if (edge.Metadata.Type == "fabric") {
        fabricNode = node;
        continue;
      }

      if ((edge.Parent == hostNode) || (edge.Child == hostNode)) {
        child = edge.Child;
        var found = false;
        for (var n in child.Edges) {
          nEdge = edge.Child.Edges[n];
          if (nEdge.Metadata.Type == "fabric")
            found = true;
            continue;
        }

        if (found)
          continue;
      }

      edge.Visible = isCollapsed ? false : true;
    }

    if (node == fabricNode)
      continue;

    node.Visible = isCollapsed ? false : true;
  }

  hostNode.Collapsed = isCollapsed;
};

TopologyLayout.prototype.CollapseNode = function(d) {
  if (d3.event.defaultPrevented)
    return;

  switch(d.Metadata.Type) {
    case "netns":
      this.CollapseNetNS(d);
      break;
    case "host":
      this.CollapseHost(d);
      break;
    default:
      return;
  }

  this.Redraw();
};

TopologyLayout.prototype.Redraw = function() {
  var self = this;

  if (typeof this.redrawTimeout == "undefined")
    this.redrawTimeout = setTimeout(function() {
      for (var i in self.deferredActions)
      {
        var action = self.deferredActions[i];
        action.fn.apply(self, action.params);
      }
      self.deferredActions = [];

      self.redraw();

      clearTimeout(self.redrawTimeout);
      self.redrawTimeout = undefined;
    }, 100);
};

TopologyLayout.prototype.redraw = function() {
  var _this = this;

  this.link = this.link.data(this.links, function(d) { return d.edge.ID; });
  this.link.exit().remove();

  this.link.select('path').style("opacity", function(d) {
    return _this.EdgeOpacity(d);
  });
  this.link.select("textPath").style("opacity", function(d) {
    return _this.EdgeOpacity(d);
  });

  var linkEnter = this.link.enter().append("g");

  linkEnter.append("path")
    .attr("class", function(d) {
      return "linkpath " + _this.EdgeClass(d);
    })
    .attr("id",function(d) {
      return "linkpath_" + d.edge.ID;
    })
    .attr("marker-end", "url(#end)")
    .style("opacity", function(d) {
      return _this.EdgeOpacity(d);
    });

  linkEnter.append('path')
           .attr("class","textpath")
           .style("opacity", "0")
           .attr("id", function(d) { return "path_" + d.edge.ID; });

  linkEnter.append('text').append('textPath')
           .attr("id",function(d) { return "pathlabel_" + d.edge.ID;})
           .attr("xlink:href", function(d) { return "#path_" + d.edge.ID; })
           .attr("class", function(d) {
             return "labelpath " + _this.EdgeTextClass(d);
            })
           .style("text-anchor","middle")
           .attr("startOffset", "50%")
           .text(function(d,i){return "";});

  this.node = this.node.data(this.nodes, function(d) { return d.ID; })
      .attr("class", function(d) {
        return _this.NodeClass(d);
    })
    .style("display", function(d) {
      return !d.Visible ? "none" : "block";
    });
  this.node.exit().remove();

  var nodeEnter = this.node.enter().append("g")
    .attr("id", function(d) { return "node-" + d.ID; })
    .attr("tid", function(d) {return d.Metadata["TID"]})
    .attr("class", function(d) {
      return _this.NodeClass(d);
    })
    .on("click", function(d) {
      if (d3.event.shiftKey) {
        if (d.fixed)
          d.fixed = false;
        else
          d.fixed = true;
        _this.redraw();
        return;
      }
      store.commit('selected', d);
    })
    .on("dblclick", function(d) {
      return _this.CollapseNode(d);
    })
    .call(this.drag);

  nodeEnter.append("circle")
    .attr("r", this.CircleSize)
    .attr("class", "circle")
    .style("opacity", function(d) {
      return _this.CircleOpacity(d);
    });

  nodeEnter.append("image")
    .attr("class", "picto")
    .attr("xlink:href", function(d) {
      return _this.NodePicto(d);
    })
    .attr("x", -10)
    .attr("y", -10)
    .attr("width", 20)
    .attr("height", 20);

  nodeEnter.append("image")
    .attr("class", "probe")
    .attr("x", -25)
    .attr("y", 5)
    .attr("width", 20)
    .attr("height", 20);

  nodeEnter.append("image")
    .attr("class", "pin")
    .attr("x", 10)
    .attr("y", -23)
    .attr("width", 16)
    .attr("height", 16);

  nodeEnter.append("image")
    .attr("class", "state")
    .attr("x", -20)
    .attr("y", -20)
    .attr("width", 12)
    .attr("height", 12);

  nodeEnter.append("circle")
    .attr("class", "manager")
    .attr("r", 12)
    .attr("cx", 14)
    .attr("cy", 16);

  nodeEnter.append("image")
    .attr("class", "manager")
    .attr("x", 4)
    .attr("y", 6)
    .attr("width", 20)
    .attr("height", 20);

  nodeEnter.append("text")
    .attr("dx", 22)
    .attr("dy", ".35em")
    .text(function(d) {
      return _this.GetNodeText(d);
    });

  // bounding boxes for groups
  this.groupsG.selectAll("path.group").remove();
  this.group = this.groupsG.selectAll("path.group")
    .data(this.Groups())
    .enter().append("path")
    .attr("class", function(d) {
      return _this.GroupClass(d);
    })
    .attr("id", function(d) {
      return d.group;
    })
    .attr("d", function(d) {
      return _this.DrawCluster(d);
    });

  this.node.select('text')
    .text(function(d){
        return _this.GetNodeText(d);
    });

  this.node.select('image.state').attr("xlink:href", function(d) {
    return _this.NodeStatePicto(d);
  });

  this.node.select('image.probe').attr("xlink:href", function(d) {
    return _this.NodeProbeStatePicto(d);
  });

  this.node.select('image.pin').attr("xlink:href", function(d) {
    return _this.NodePinStatePicto(d);
  });

  this.node.select('image.manager').attr("xlink:href", function(d) {
    return _this.NodeManagerPicto(d);
  });

  this.node.select('circle.manager').attr("style", function(d) {
    return _this.NodeManagerStyle(d);
  });

  this.force.start();
};

TopologyLayout.prototype.ProcessGraphMessage = function(msg) {
 if (msg.Type != "SyncReply" && (!this.vm.live || !this.synced) ) {
    console.log("Skipping message " + msg.Type);
    return;
  }

  var node;
  var edge;
  switch(msg.Type) {
    case "SyncReply":
      this.synced = false;
      this.Clear();
      this.InitFromSyncMessage(msg);
      this.synced = true;
      break;

    case "NodeUpdated":
      node = this.graph.GetNode(msg.Obj.ID);
      var redraw = (msg.Obj.Metadata.State !== node.Metadata.State) ||
                   (JSON.stringify(msg.Obj.Metadata.Capture) !== JSON.stringify(node.Metadata.Capture));

      if (redraw) {
        this.deferredActions.push({fn: this.UpdateNode, params: [node, msg.Obj.Metadata]});
        this.Redraw();
      } else {
        this.UpdateNode(node, msg.Obj.Metadata);
      }
      break;

    case "NodeAdded":
      node = this.graph.NewNode(msg.Obj.ID, msg.Obj.Host);
      if ("Metadata" in msg.Obj)
        node.Metadata = msg.Obj.Metadata;

      this.deferredActions.push({fn: this.AddNode, params: [node]});
      this.Redraw();
      break;

    case "NodeDeleted":
      node = this.graph.GetNode(msg.Obj.ID);
      if (typeof node == "undefined")
        return;

      this.graph.DelNode(node);
      this.deferredActions.push({fn: this.DelNode, params: [node]});
      this.Redraw();
      break;

    case "EdgeUpdated":
      edge = this.graph.GetEdge(msg.Obj.ID);
      edge.Metadata = msg.Obj.Metadata;

      this.Redraw();
      break;

    case "EdgeAdded":
      var parent = this.graph.GetNode(msg.Obj.Parent);
      var child = this.graph.GetNode(msg.Obj.Child);

      edge = this.graph.NewEdge(msg.Obj.ID, parent, child, msg.Obj.Host);
      if ("Metadata" in msg.Obj)
        edge.Metadata = msg.Obj.Metadata;

      this.deferredActions.push({fn: this.AddEdge, params: [edge]});
      this.Redraw();
      break;

    case "EdgeDeleted":
      edge = this.graph.GetEdge(msg.Obj.ID);
      if (typeof edge == "undefined")
        break;

      this.graph.DelEdge(edge);
      this.deferredActions.push({fn: this.DelEdge, params: [edge]});
      this.Redraw();
      break;
  }
};

TopologyLayout.prototype.ProcessAlertMessage = function(msg) {
  var _this = this;

  var ID  = msg.Obj.ReasonData.ID;
  this.alerts[ID] = msg.Obj;
  this.Redraw();

  setTimeout(function() { delete this.alerts[ID]; _this.Redraw(); }, 1000);
};

TopologyLayout.prototype.SyncRequest = function(t) {
  if (t && t === store.state.time) {
    return;
  }
  var obj = {};
  if (t) {
    obj.Time = t;
    store.commit('time', t);
  } else {
    store.commit('time', 0);
  }
  var msg = {Namespace: "Graph", Type: "SyncRequest", Obj: obj};
  websocket.send(msg);
};

// edge speed for some interfaces are not reported by netlink, defining 10Gps
const DefaultInterfaceSpeed = 1048576;

function isActive(kbps, bandwidth, speed) {
  return (kbps > bandwidth.active*speed) && (kbps < bandwidth.warning*speed);
}

function isWarning(kbps, bandwidth, speed) {
  return (kbps >= bandwidth.warning*speed) && (kbps < bandwidth.alert*speed);
}

function isAlert(kbps, bandwidth, speed) {
  return kbps >= bandwidth.alert*speed;
}

function getBandwidth(res, source) {
  var totalKbit = 0;
  var totalByte = 0;
  var deltaTime = 0;

  if (res === null || typeof res === 'undefined' || res.LastMetric === undefined)
    return 0;

  if (source == "netlink") {
    totalByte = res.LastMetric.RxBytes + res.LastMetric.TxBytes;

    if (typeof totalByte === 'undefined')
      return 0;

    deltaTime = res.LastMetric.Last - res.LastMetric.Start;
  } else if (source == "flows") {
    deltaTime = res.Last - res.Start;
    totalByte = res.ABBytes + res.BABytes;
  }

  deltaTime = Math.floor(deltaTime / 1000); // ms to sec
  if (deltaTime >=1 && totalByte >= 1) {
    totalKbit = Math.floor(8*totalByte / (1024*deltaTime)); // to kbit per sec
  }
  return totalKbit;
}

TopologyLayout.prototype.getBandwidthConfiguration = function() {
  var vm = this.vm;
  var b = this.bandwidth;
  var cfgNames = {};

  cfgNames.relative = ['analyzer.bandwidth_relative_active',
    'analyzer.bandwidth_relative_warning',
    'analyzer.bandwidth_relative_alert'];
  cfgNames.absolute = ['analyzer.bandwidth_absolute_active',
    'analyzer.bandwidth_absolute_warning',
    'analyzer.bandwidth_absolute_alert'];


  return $.when(
      vm.$getConfigValue('analyzer.bandwidth_update_rate'),
      vm.$getConfigValue('analyzer.bandwidth_source'),
      vm.$getConfigValue('analyzer.bandwidth_threshold'))
    .then(function(period, src, threshold) {
      b.updatePeriod = period[0]*1000; // in millisec
      b.bandwidth_source = src[0];
      b.bandwidth_threshold = threshold[0];
      return b.bandwidth_threshold;
    })
  .then(function(t) {
    return $.when(
      vm.$getConfigValue(cfgNames[t][0]),
      vm.$getConfigValue(cfgNames[t][1]),
      vm.$getConfigValue(cfgNames[t][2]))
      .then(function(active, warning, alert) {
        b.active = active[0];
        b.warning = warning[0];
        b.alert = alert[0];
      });
  });
};

function displayBandwidth(ID, bandwidth, speed, totalKb) {
  if (totalKb > bandwidth.active*speed) {
    d3.select("#pathlabel_" + ID)
      .classed ("path-link-active",  isActive(totalKb, bandwidth, speed))
      .classed ("path-link-warning", isWarning(totalKb, bandwidth, speed))
      .classed ("path-link-alert",   isAlert(totalKb, bandwidth, speed))
      .text(bandwidthToString(totalKb));

    d3.select("#linkpath_" + ID)
      .style("stroke", (isActive(totalKb, bandwidth, speed) ? "green" :
            (isWarning(totalKb, bandwidth, speed) ? "yellow" :
             (isAlert(totalKb, bandwidth, speed) ? "red" : ""))));
  } else {
    d3.select("#pathlabel_" + ID)
      .classed ("link-active",  false)
      .classed ("link-warning", false)
      .classed ("link-alert",  false)
      .text("");

    d3.select("#linkpath_" + ID)
      .style("stroke", "");
  }
}

function bandwidthCompletion(res) {
  var totalKb = getBandwidth(res, this.bandwidth.source, this.link.target);
  var speed = (this.bandwidth.bandwidth_threshold == 'relative') ? this.link.target.Metadata.Speed || DefaultInterfaceSpeed : 1;
  displayBandwidth(this.ID, this.bandwidth, speed, totalKbit);
  this.bandwidth.outstandingAjax--;
}

TopologyLayout.prototype.RefreshLinks = function() {
  var query ='';
  var bandwidth = this.bandwidth;
  var netLinkBandwidth = 0;

  if (this.bandwidth.outstandingAjax > 0) {
    return;
  }
  for (var i in this.links) {
    var link = this.links[i];

    if (link.edge.Metadata.RelationType != "layer2") {
      continue;
    }
    var speed = (this.bandwidth.bandwidth_threshold == 'relative') ?
      link.target.Metadata.Speed || DefaultInterfaceSpeed : 1;

    if (bandwidth.source == "netlink" && typeof link.target.Metadata.TID != "undefined") {
      netLinkBandwidth = getBandwidth(this.vm.extractMetadata(this.graph.Nodes[link.target.ID].Metadata, "LastMetric"),
          bandwidth.source);
      displayBandwidth(link.edge.ID, bandwidth, speed, netLinkBandwidth);
      continue;
    } else { // source == "flows"
      if (!link.target.IsCaptureOn()) {
        d3.select("#pathlabel_" + ID)
          .classed ("link-active",  false)
          .classed ("link-warning", false)
          .classed ("link-alert",  false)
          .text("");
        continue;
      }
      query = "G.Flows().Has('NodeTID', '" + link.target.Metadata.TID +"').Metrics().Sum()";
    }

    var ID =  link.edge.ID;
    bandwidth.outstandingAjax++;
    this.vm.$topologyQuery(query, {bandwidth:bandwidth, link:link, ID:ID})
      .fail(function() { this.bandwidth.outstandingAjax--; })
      .then(bandwidthCompletion);
  }
};
