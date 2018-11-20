function WSHandler() {
  this.host = location.host;
  this.conn = null;
  this.connected = null;
  this.disconnected = null;
  this.msgHandlers = {};
  this.discHandlers = [];
  this.connHandlers = [];
  this.errorHandlers = [];
}

WSHandler.prototype = {

  connect: function() {
    var self = this;

    if (this.conn && this.conn.readyState == WebSocket.OPEN) {
      return;
    }

    this._connect();
  },

  _connect: function() {
    var self = this;

    this.connected = $.Deferred();
    this.connected.then(function() {
      self.connHandlers.forEach(function(callback) {
        callback();
      });
    });
    this.disconnected = $.Deferred();
    this.disconnected.then(function() {
      self.discHandlers.forEach(function(callback) {
        callback();
      });
    });
    this.connecting = true;

    this.protocol = "ws://";
    if (location.protocol == "https:") {
      this.protocol = "wss://";
    }
    this.conn = new WebSocket(this.protocol + this.host + "/ws/subscriber?x-client-type=webui");
    this.conn.onopen = function() {
      self.connecting = false;
      self.connected.resolve(true);
    };
    this.conn.onclose = function() {
      // connection closed after a succesful connection
      if (self.connecting === false) {
        self.disconnected.resolve(true);
        // client never succeed to connect in the first place
      } else {
        self.connecting = false;
        self.connected.reject(false);
      }
    };
    this.conn.onmessage = function(r) {
      var msg = JSON.parse(r.data);
      if (self.msgHandlers[msg.Namespace]) {
        self.msgHandlers[msg.Namespace].forEach(function(callback) {
          callback(msg);
        });
      }
    };
    this.conn.onerror = function(r) {
      self.errorHandlers.forEach(function(callback) {
        callback();
      });
    };
  },

  disconnect: function() {
    this.conn && this.conn.close();
  },

  addMsgHandler: function(namespace, callback) {
    if (! this.msgHandlers[namespace]) {
      this.msgHandlers[namespace] = [];
    }
    this.msgHandlers[namespace].push(callback);
  },

  addConnectHandler: function(callback) {
    this.connHandlers.push(callback);
    if (this.connected !== null) {
      this.connected.then(function() {
        callback();
      });
    }
  },

  delConnectHandler: function(callback) {
    this.connHandlers.splice(
      this.connHandlers.indexOf(callback), 1);
  },

  addDisconnectHandler: function(callback) {
    this.discHandlers.push(callback);
    if (this.disconnected !== null) {
      this.disconnected.then(function() {
        callback();
      });
    }
  },

  addErrorHandler: function(callback) {
    this.errorHandlers.push(callback);
  },

  send: function(msg) {
    this.conn.send(JSON.stringify(msg));
  }

};

if (!window.layoutConfig.getValue('useNewUi')) {
  var websocket = new WSHandler();
}



var getImagePath = function(label) {
	return 'statics/img/'+label+'.png';
};

var minusImg = getImagePath('minus-outline-16');
var plusImg = getImagePath('plus-16');
var captureIndicatorImg = getImagePath('media-record');
var pinIndicatorImg = getImagePath('pin');

var setupFixedImages = function(labelMap) {
  imgMap = {};
  Object.keys(labelMap).forEach(function(key) {
    imgMap[key] = getImagePath(labelMap[key]);
  });
  return imgMap;
};

var nodeImgMap = setupFixedImages({
  "host": "host",
  "port": "port",
  "ovsport": "port",
  "bridge": "bridge",
  "switch": "switch",
  "ovsbridge": "switch",
  "netns": "ns",
  "veth": "veth",
  "bond": "port",
  "default": "intf",
  // k8s
  "cluster": "cluster",
  "container": "container",
  "cronjob": "cronjob",
  "daemonset": "daemonset",
  "deployment": "deployment",
  "endpoints": "endpoints",
  "ingress": "ingress",
  "job": "job",
  "node": "host",
  "persistentvolume": "persistentvolume",
  "persistentvolumeclaim": "persistentvolumeclaim",
  "pod": "pod",
  "networkpolicy": "networkpolicy",
  "namespace": "ns",
  "replicaset": "replicaset",
  "replicationcontroller": "replicationcontroller",
  "service": "service",
  "statefulset": "statefulset",
  "storageclass": "storageclass",
  // istio
  "destinationrule": "destinationrule",
  "gateway": "gateway",
  "quotaspec": "quotaspec",
  "quotaspecbinding": "quotaspecbinding",
  "serviceentry": "serviceentry",
  "virtualservice": "virtualservice",
});

var managerImgMap = setupFixedImages({
  "docker": "docker",
  "lxd": "lxd",
  "neutron": "openstack",
  "k8s": "k8s",
  "istio": "istio",
});



const maxClockSkewMillis = 5 * 60 * 1000; // 5 minutes

var LinkLabelBandwidth = Vue.extend({
  mixins: [apiMixin],

  methods: {

    bandwidthFromMetrics: function(metrics) {
      if (!metrics) {
        return 0;
      }
      if (!metrics.Last) {
        return 0;
      }
      if (!metrics.Start) {
        return 0;
      }

      const totalByte = (metrics.RxBytes || 0) + (metrics.TxBytes || 0);
      const deltaMillis = metrics.Last - metrics.Start;
      const elapsedMillis = Date.now() - new Date(metrics.Last);

      if (deltaMillis === 0) {
        return 0;
      }
      if (elapsedMillis > maxClockSkewMillis) {
        return 0;
      }

      return Math.floor(8 * totalByte * 1000 / deltaMillis); // bits-per-second
    },

    setup: function(topology) {
      this.topology = topology;
    },

    updateData: function(link) {
      var metadata;
      if (link.target.Metadata.LastUpdateMetric) {
        metadata = link.target.Metadata;
      } else if (link.source.Metadata.LastUpdateMetric) {
        metadata = link.source.Metadata;
      } else {
        return;
      }

      const defaultBandwidthBaseline = 1024 * 1024 * 1024; // 1 gbps

      link.bandwidthBaseline = (this.topology.bandwidth.bandwidthThreshold === 'relative') ?
        metadata.Speed || defaultBandwidthBaseline : 1;

      link.bandwidthAbsolute = this.bandwidthFromMetrics(metadata.LastUpdateMetric);
      link.bandwidth = link.bandwidthAbsolute / link.bandwidthBaseline;
    },

    hasData: function(link) {
      if (!link.target.Metadata.LastUpdateMetric && !link.source.Metadata.LastUpdateMetric) {
        return false;
      }

      if (!link.bandwidth) {
        return false;
      }
      return link.bandwidth > this.topology.bandwidth.active;
    },

    getText: function(link) {
      return bandwidthToString(link.bandwidthAbsolute);
    },

    isActive: function(link) {
      return (link.bandwidth > this.topology.bandwidth.active) && (link.bandwidth < this.topology.bandwidth.warning);
    },

    isWarning: function(link) {
      return (link.bandwidth >= this.topology.bandwidth.warning) && (link.bandwidth < this.topology.bandwidth.alert);
    },

    isAlert: function(link) {
      return link.bandwidth >= this.topology.bandwidth.alert;
    },
  },
});

var LinkLabelLatency = Vue.extend({
  mixins: [apiMixin],

  methods: {
    setup: function(topology) {
      this.active = 0;
      this.warning = 10
      this.alert = 100;
    },

    updateLatency: function(link, a, b) {
      link.latencyTimestamp = Math.max(a.Last, b.Last);
      link.latency = Math.abs(a.RTT - b.RTT) / 1000000;
    },

    flowQuery: function(nodeTID, trackingID, limit) {
      let has = `"NodeTID", ${nodeTID}`;
      if (typeof trackingID !== 'undefined') {
        has += `"TrackingID", ${trackingID}`;
      }
      has += `"RTT", NE(0)`;
      let query = `G.Flows().Has(${has}).Sort().Limit(${limit})`;
      return this.$topologyQuery(query)
    },

    flowQueryByNodeTID: function(nodeTID, limit) {
      return this.flowQuery(`"${nodeTID}"`, undefined, limit);
    },

    flowQueryByNodeTIDandTrackingID: function(nodeTID, flows) {
      let anyTrackingID = 'Within(';
      for (let i in flows) {
        const flow = flows[i];
        if (i != 0) {
          anyTrackingID += ', ';
        }
        anyTrackingID += `"${flow.TrackingID}"`;
      }
      anyTrackingID += ')';
      return this.flowQuery(`"${nodeTID}"`, anyTrackingID, 1);
    },

    flowCategoryKey: function(flow) {
      return `a=${flow.Link.A} b=${flow.Link.B} app=${flow.Application}`;
    },

    uniqueFlows: function(inFlows, count) {
      let outFlows = [];
      let hasCategory = {};
      for (let i in inFlows) {
        if (count <= 0) {
          break;
        }
        const flow = inFlows[i];
        const key = this.flowCategoryKey(flow);
        if (key in hasCategory) {
          continue;
        }
        hasCategory[key] = true;
        outFlows.push(flow);
        count--;
      }
      return outFlows;
    },

    mapFlowByTrackingID: function(flows) {
      let map = {};
      for (let i in flows) {
        const flow = flows[i];
        map[flow.TrackingID] = flow;
      }
      return map;
    },

    updateData: function(link) {
      var self = this;

      const a = link.source.Metadata;
      const b = link.target.Metadata;

      if (!a.Capture) {
        return;
      }
      if (!b.Capture) {
        return;
      }

      const maxFlows = 1000;
      this.flowQueryByNodeTID(a.TID, maxFlows)
        .then(function(aFlows) {
          if (aFlows.length === 0) {
            return;
          }
          const maxUniqueFlows = 100;
          aFlows = self.uniqueFlows(aFlows, maxUniqueFlows);
          const aFlowMap = self.mapFlowByTrackingID(aFlows);
          self.flowQueryByNodeTIDandTrackingID(b.TID, aFlows)
            .then(function(bFlows) {
              if (bFlows.length === 0) {
                return;
              }
              const bFlow = bFlows[0];
              const aFlow = aFlowMap[bFlow.TrackingID];
              self.updateLatency(link, aFlow, bFlow);
            })
            .catch(function(error) {
              console.log(error);
            });
        })
        .catch(function(error) {
          console.log(error);
        });
    },

    hasData: function(link) {
      if (!link.latencyTimestamp) {
        return false;
      }
      const elapsedMillis = Date.now() - new Date(link.latencyTimestamp);
      return elapsedMillis <= maxClockSkewMillis;
    },

    getText: function(link) {
      return `${link.latency} ms`;
    },

    isActive: function(link) {
      return (link.latency >= this.active) && (link.latency < this.warning);
    },

    isWarning: function(link) {
      return (link.latency >= this.warning) && (link.latency < this.alert);
    },

    isAlert: function(link) {
      return (link.latency >= this.alert);
    },
  },
});

var TopologyGraphLayout = function(vm, selector) {
  var self = this;

  this.linkLabelType = "bandwidth";

  this.vm = vm;

  this.initD3Data();

  this.handlers = [];

  this.queue = new Queue();
  this.queue.await(function() {
    if (self.invalid) {
      if (self._autoExpand) {
        while (self.toggleCollapseByLevel(false));
        setTimeout(function() {
          self.zoomFit();
        }, 5000);
      }
      self.update();
    }
    self.invalid = false;
  }).start(100);

  this.width = $(selector).width() - 20;
  this.height = $(selector).height();

  this.simulation = d3.forceSimulation(Object.values(this.nodes))
    .force("charge", d3.forceManyBody().strength(-500))
    .force("link", d3.forceLink(Object.values(this.links)).distance(this.linkDistance).strength(0.9).iterations(2))
    .force("collide", d3.forceCollide().radius(80).strength(0.1).iterations(1))
    .force("center", d3.forceCenter(this.width / 2, this.height / 2))
    .force("x", d3.forceX(0).strength(0.01))
    .force("y", d3.forceY(0).strength(0.01))
    .alphaDecay(0.0090);

  this.zoom = d3.zoom()
    .on("zoom", this.zoomed.bind(this));

  this.svg = d3.select(selector).append("svg")
    .attr("width", this.width)
    .attr("height", this.height)
    .call(this.zoom)
    .on("dblclick.zoom", null);

  var defsMarker = function(type, target, point) {
    let id = "arrowhead-"+type+"-"+target+"-"+point;

    let refX = 1.65
    let refY = 0.15
    let pathD = "M0,0 L0,0.3 L0.5,0.15 Z"
    if (type === "egress" || point === "end") {
      pathD = "M0.5,0 L0.5,0.3 L0,0.15 Z"
    }

    if (target === "deny") {
      refX = 1.85
      refY = 0.3
      a = "M0.1,0 L0.6,0.5 L0.5,0.6 L0,0.1 Z"
      b = "M0,0.5 L0.1,0.6 L0.6,0.1 L0.5,0 Z"
      pathD = a + " " + b
    }

    let color = "rgb(0, 128, 0, 0.8)"
    if (target === "deny") {
      color = "rgba(255, 0, 0, 0.8)"
    }

    self.svg.append("defs").append("marker")
      .attr("id", id)
      .attr("refX", refX)
      .attr("refY", refY)
      .attr("markerWidth", 1)
      .attr("markerHeight", 1)
      .attr("orient", "auto")
      .attr("markerUnits", "strokeWidth")
      .append("path")
        .attr("fill", color)
        .attr("d", pathD);
  }

  defsMarker("ingress", "deny", "begin");
  defsMarker("ingress", "deny", "end");
  defsMarker("ingress", "allow", "begin");
  defsMarker("ingress", "allow", "end");
  defsMarker("egress", "deny", "begin");
  defsMarker("egress", "deny", "end");
  defsMarker("egress", "allow", "begin");
  defsMarker("egress", "allow", "end");

  this.g = this.svg.append("g");

  this.group = this.g.append("g").attr('class', 'groups').selectAll(".group");
  this.linkWrap = this.g.append("g").attr('class', 'link-wraps').selectAll(".link-wrap");
  this.link = this.g.append("g").attr('class', 'links').selectAll(".link");
  this.linkLabel = this.g.append("g").attr('class', 'link-labels').selectAll(".link-label");
  this.node = this.g.append("g").attr('class', 'nodes').selectAll(".node");

  this.simulation
    .on("tick", this.tick.bind(this));

  this.bandwidth = {
    bandwidthThreshold: 'absolute',
    updatePeriod: 3000,
    active: 5,
    warning: 100,
    alert: 1000,
    intervalID: null,
  };

  this.loadBandwidthConfig()
  self.bandwidth.intervalID = setInterval(self.updateLinkLabelHandler.bind(self), self.bandwidth.updatePeriod);
};

TopologyGraphLayout.prototype = {

  linkLabelFactory: function(link) {
    let type, driver;
    switch (this.linkLabelType) {
      case "latency":
        type = "latency";
        driver = new LinkLabelLatency();
        break;
      default:
        type = "bandwidth";
        driver = new LinkLabelBandwidth();
        break;
    }
    driver.setup(this);
    return driver;
  },

  notifyHandlers: function(ev, v1) {
    var self = this;

    this.handlers.forEach(function(h) {
      switch (ev) {
        case 'nodeSelected': h.onNodeSelected(v1); break;
        case 'edgeSelected': h.onEdgeSelected(v1); break;
      }
    });
  },

  addHandler: function(handler) {
    this.handlers.push(handler);
  },

  removeHandler: function(handler) {
    var index = this.handlers.indexOf(handler);
    if (index > -1) {
      this.handlers.splice(index, 1);
    }
  },

  zoomIn: function() {
    this.svg.transition().duration(500).call(this.zoom.scaleBy, 1.1);
  },

  zoomOut: function() {
    this.svg.transition().duration(500).call(this.zoom.scaleBy, 0.9);
  },

  zoomFit: function() {
    var bounds = this.g.node().getBBox();
    var parent = this.g.node().parentElement;
    var fullWidth = parent.clientWidth, fullHeight = parent.clientHeight;
    var width = bounds.width, height = bounds.height;
    var midX = bounds.x + width / 2, midY = bounds.y + height / 2;
    if (width === 0 || height === 0) return;
    var scale = 0.75 / Math.max(width / fullWidth, height / fullHeight);
    var translate = [fullWidth / 2 - midX * scale, fullHeight / 2 - midY * scale];

    var t = d3.zoomIdentity
      .translate(translate[0] + 30, translate[1])
      .scale(scale);
      this.svg.transition().duration(500).call(this.zoom.transform, t);
  },

  initD3Data: function() {
    this.nodes = {};
    this._nodes = {};

    this.links = {};
    this._links = {};

    this.groups = [];
    this.collapseLevel = 0;

    this.linkLabelData = {};

    this.collapsed = this.defaultCollpsed || false;
    this.selectedNode = null;
    this.selectedEdge = null;
    this.invalid = false;
  },

  onPreInit: function() {
    this.queue.stop();
    this.queue.clear();

    this.initD3Data();
    this.update();
  },

  onPostInit: function() {
    var self = this;
    setTimeout(function() {
      self.queue.start(100);
    }, 1000);
    self.update();
  },

  linkStrength: function(e) {
    var strength = 0.9;

    if ((e.source.Metadata.Type === "netns") && (e.target.Metadata.Type === "netns"))
      return 0.01;

     return strength;
 },

  linkDistance: function(e) {
    var distance = 100, coeff;

    // application
    if ((e.source.Metadata.Type === "netns") && (e.target.Metadata.Type === "netns"))
      return 1800;

    if (e.source.group !== e.target.group) {
      if (e.source.isGroupOwner()) {
        coeff = e.source.group.collapsed ? 40 : 60;
        if (e.source.group.memberArray) {
          distance += coeff * (e.source.group.memberArray.length + e.source.group._memberArray.length) / 10;
        }
      }
      if (e.target.isGroupOwner()) {
        coeff = e.target.group.collapsed ? 40 : 60;
        if (e.target.group.memberArray) {
          distance += coeff * (e.target.group.memberArray.length + e.target.group._memberArray.length) / 10;
        }
      }
    }
    return distance;
  },

  hideNode: function(d) {
    if (this.hidden(d)) return;
    d.visible = false;

    delete this.nodes[d.id];
    this._nodes[d.id] = d;

    // remove links with neighbors
    if (d.links) {
      for (var i in d.links) {
        var link = d.links[i];

        if (this.links[link.id]) {
          delete this.links[link.id];
          this._links[link.id] = link;

          this.delLinkLabel(link);
        }
      }
    }

    var group = d.group, match = function(n) { return n !== d; };
    while(group && group.memberArray) {
      group.memberArray = group.memberArray.filter(match);
      if (group._memberArray.indexOf(d) < 0) group._memberArray.push(d);
      group = group.parent;
    }
  },

  hidden: function(d) {
    return d.Metadata.Type === "ofrule";
  },

  showNode: function(d) {
    if (this.hidden(d)) return;
    d.visible = true;

    delete this._nodes[d.id];
    this.nodes[d.id] = d;

    var i, links = d.links;
    for (i in links) {
      var link = links[i];

      if (this._links[link.id] && link.source.visible && link.target.visible) {
        delete this._links[link.id];
        this.links[link.id] = link;
      }
    }

    var group = d.group, match = function(n) { return n !== d; };
    while(group && group.memberArray) {
      group._memberArray = group._memberArray.filter(match);
      if (group.memberArray.indexOf(d) < 0) group.memberArray.push(d);
      group = group.parent;
    }

    this.update();
  },

  onGroupAdded: function(group) {
    this.queue.defer(this._onGroupAdded.bind(this), group);
  },

  _onGroupAdded: function(group) {
    group.ownerType = group.owner.Metadata.Type;
    group.level = 1;
    group.depth = 1;
    group.collapsed = this.collapsed;

    // list of all group and sub group members
    group.memberArray = [];
    group._memberArray = [];
    group.collapseLinks = [];

    this.groups.push(group);

    this.groupOwnerSet(group.owner);
  },

  delGroup: function(group) {

    var self = this;

    this.delCollapseLinks(group);

    this.groups = this.groups.filter(function(g) { return g.id !== group.id; });

    this.groupOwnerUnset(group.owner);
  },

  onGroupDeleted: function(group) {
    this.queue.defer(this._onGroupDeleted.bind(this), group);
  },

  _onGroupDeleted: function(group) {
    if(group) this.delGroup(group);
  },

  addGroupMember: function(group, node) {
    if (this.hidden(node)) return;

    while(group && group.memberArray) {
      if (node === group.owner) {
        if (group.memberArray.indexOf(node) < 0) group.memberArray.push(node);
      } else {
        var members = group.collapsed ? group._memberArray : group.memberArray;
        if (members.indexOf(node) < 0) members.push(node);
        if (node.group && group.collapsed) this.collapseNode(node, group);
      }
      group = group.parent;
    }
  },

  onGroupMemberAdded: function(group, node) {
    if (this.hidden(node)) return;
    this.queue.defer(this._onGroupMemberAdded.bind(this), group, node);
  },

  _onGroupMemberAdded: function(group, node) {
    this.addGroupMember(group, node);
  },

  delGroupMember: function(group, node) {
    var match = function(n) { return n !== node; };
    while(group && group.memberArray) {
      group.memberArray = group.memberArray.filter(match);
      group._memberArray = group._memberArray.filter(match);
      group = group.parent;
    }
  },

  onGroupMemberDeleted: function(group, node) {
    this.queue.defer(this._onGroupMemberDeleted.bind(this), group, node);
  },

  _onGroupMemberDeleted: function(group, node) {
    this.delGroupMember(group, node);
  },

  setGroupLevel: function(group) {
    var level = 0, g = group;
    while (g) {
      if (level > g.depth) g.depth = level;
      level++;

      g = g.parent;
    }
    group.level = level;
  },

  onParentSet: function(group) {
    this.queue.defer(this._onParentSet.bind(this), group);
  },

  _onParentSet: function(group) {
    var i;
    for (i = this.groups.length - 1; i >= 0; i--) {
      this.setGroupLevel(this.groups[i]);
    }
    this.groups.sort(function(a, b) { return a.level - b.level; });

    var members = Object.values(group.members);
    for (i = members.length - 1; i >= 0; i--) {
      this.addGroupMember(group.parent, members[i]);
    }
  },

  delLink: function(link) {
    delete this.links[link.id];
    delete this._links[link.id];

    // reattache is no outside link
    var i, l, els = [link.source, link.target];
    for (var j in els) {
      var e = els[j];

      if (!e.isGroupOwner() && e.group && !this.hasOutsideLink(e.group)) {
        for (i in e.group.owner.links) {
          l = e.group.owner.links[i];
          if (l.Metadata.RelationType === "ownership" && l.source.group !== l.target.group && l.source.visible && l.target.visible) {
            this.links[l.id] = l;
            delete this._links[l.id];
          }
        }
      }
    }

    this.delLinkLabel(link);
  },

  onEdgeAdded: function(link) {
    if (this.hidden(link.target) || this.hidden(link.source)) return;
    this.queue.defer(this._onEdgeAdded.bind(this), link);
  },

  hasOutsideLink: function(group) {
    var members = group.members;
    for (var i in members) {
      var d = members[i], links = d.links;
      for (var j in links) {
        var e = links[j];
        if (e.Metadata.RelationType !== "ownership" && e.source.group !== e.source.target) return true;
      }
    }

    return false;
  },

  _onEdgeAdded: function(link) {
    link.source.links[link.id] = link;
    link.target.links[link.id] = link;

    if (link.Metadata.RelationType === "ownership") {
      if (this.isNeutronRelatedVMNode(link.target)) return;

      if (link.target.Metadata.Driver === "openvswitch" &&
          ["patch", "vxlan", "gre", "geneve"].indexOf(link.target.Metadata.Type) >= 0) return;

      link.target.linkToParent = link;

      // do not add ownership link for groups having outside link
      if (link.target.isGroupOwner("ownership") && this.hasOutsideLink(link.target.group)) return;
    }

    var sourceGroup = link.source.group, targetGroup = link.target.group;
    if (targetGroup && targetGroup.type === "ownership" && this.hasOutsideLink(targetGroup) &&
        targetGroup.owner.linkToParent) this.delLink(targetGroup.owner.linkToParent);
    if (sourceGroup && sourceGroup.type === "ownership" && this.hasOutsideLink(sourceGroup) &&
        sourceGroup.owner.linkToParent) this.delLink(sourceGroup.owner.linkToParent);

    var i, noc, edges, metadata, source = link.source, target = link.target;
    if (Object.values(target.edges).length >= 2 && target.linkToParent) {
      noc = 0; edges = target.edges;
      for (i in edges) {
        metadata = edges[i].Metadata;
        if (metadata.RelationType !== "ownership" && target.Metadata.Type !== "bridge" && metadata.Type !== "vlan" && ++noc >= 2) this.delLink(target.linkToParent);
      }
    }
    if (Object.keys(source.edges).length >= 2 && source.linkToParent) {
      noc = 0; edges = link.source.edges;
      for (i in edges) {
        metadata = edges[i].Metadata;
        if (metadata.RelationType !== "ownership" && source.Metadata.Type !== "bridge" && metadata.Type !== "vlan" && ++noc >= 2) this.delLink(source.linkToParent);
      }
    }

    if (!source.visible && !target.visible) {
      this._links[link.id] = link;
      if (source.group !== target.group && source.group.owner.visible && target.group.owner.visible) {
        this.addCollapseLink(source.group, source.group.owner, target.group.owner, link.Metadata);
      }
    } else if (!source.visible) {
      this._links[link.id] = link;
      if (source.group && source.group.collapsed && source.group != target.group) {
        this.addCollapseLink(source.group, source.group.owner, target, link.Metadata);
      }
    } else if (!target.visible) {
      this._links[link.id] = link;
      if (target.group && target.group.collapsed && source.group != target.group) {
        this.addCollapseLink(target.group, target.group.owner, source, link.Metadata);
      }
    } else {
      this.links[link.id] = link;
    }

    // invalid the current graph
    this.invalid = true;
  },

  onEdgeDeleted: function(link) {
    if (this.hidden(link.target) || this.hidden(link.source)) return;
    this.queue.defer(this._onEdgeDeleted.bind(this), link);
  },

  _onEdgeDeleted: function(link) {
    this.delLink(link);

    this.invalid = true;
  },

  onNodeAdded: function(node) {
    if (this.hidden(node)) return;
    this.queue.defer(this._onNodeAdded.bind(this), node);
  },

  _onNodeAdded: function(node) {
    node.visible = true;
    if (!node.links) node.links = {};
    node._Metadata = node.Metadata;

    this.nodes[node.id] = node;

    this.invalid = true;
  },

  delNode: function(node) {
    node.visible = false;

    if (this.selectedNode === node) this.selectedNode = null;

    delete this.nodes[node.id];
    delete this._nodes[node.id];

    if (node.group) this.delGroupMember(node.group, node);

    for (var i in node.links) {
      var link = node.links[i];

      delete this.links[link.id];
      delete this._links[link.id];

      if (link.collapse) this.delCollapseLinks(link.collapse.group, node);
    }
  },

  onNodeDeleted: function(node) {
    if (this.hidden(node)) return;
    this.queue.defer(this._onNodeDeleted.bind(this), node);
  },

  _onNodeDeleted: function(node) {
    this.delNode(node);

    this.invalid = true;
  },

  onNodeUpdated: function(node) {
    if (this.hidden(node)) return;
    this.queue.defer(this._onNodeUpdated.bind(this), node);
  },

  _onNodeUpdated: function(node) {
    if (this.isNeutronRelatedVMNode(node)) {
      for (var i in node.links) {
        var link = node.links[i];
        if (link.Metadata.RelationType === "ownership" && this.links[link.id]) {
          delete this.links[link.id];
          this.invalid = true;
        }
      }
    }

    if (node.Metadata.Capture && node.Metadata.Capture.State === "active" && 
        (!node._Metadata.Capture || node._Metadata.Capture.State !== "active")) {
      this.captureStarted(node);
    } else if (!node.Metadata.Capture && node._Metadata.Capture) {
      this.captureStopped(node);
    }
    if (node.Metadata.Manager && !node._Metadata.Manager) {
      this.managerSet(node);
    }
    if (node.Metadata.State !== node._Metadata.State) {
       this.stateSet(node);
    }
    node._Metadata = node.Metadata;
  },

  zoomed: function() {
    this.g.attr("transform", d3.event.transform);
  },

  cross: function(a, b, c)  {
    return (b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0]);
  },

  computeUpperHullIndexes: function(points) {
    var i, n = points.length, indexes = [0, 1], size = 2;

    for (i = 2; i < n; ++i) {
      while (size > 1 && this.cross(points[indexes[size - 2]], points[indexes[size - 1]], points[i]) <= 0) --size;
      indexes[size++] = i;
    }

    return indexes.slice(0, size);
  },

  // see original d3 implementation
  convexHull: function(group) {
    var members = group.memberArray, n = members.length;
    if (n < 1) return null;

    if (n == 1) {
      return members[0].x && members[0].y ? [[members[0].x, members[0].y], [members[0].x + 1, members[0].y + 1]] : null;
    }

    var i, node, sortedPoints = [], flippedPoints = [];
    for (i = 0; i < n; ++i) {
      node = members[i];
      if (node.x && node.y) sortedPoints.push([node.x, node.y, i]);
    }
    sortedPoints.sort(function(a, b) {
      return a[0] - b[0] || a[1] - b[1];
    });
    for (i = 0; i < sortedPoints.length; ++i) {
      flippedPoints[i] = [sortedPoints[i][0], -sortedPoints[i][1]];
    }

    var upperIndexes = this.computeUpperHullIndexes(sortedPoints),
        lowerIndexes = this.computeUpperHullIndexes(flippedPoints);

    var skipLeft = lowerIndexes[0] === upperIndexes[0],
        skipRight = lowerIndexes[lowerIndexes.length - 1] === upperIndexes[upperIndexes.length - 1],
        hull = [];

    for (i = upperIndexes.length - 1; i >= 0; --i) {
        node = members[sortedPoints[upperIndexes[i]][2]];
        hull.push([node.x, node.y]);
    }
    for (i = +skipLeft; i < lowerIndexes.length - skipRight; ++i) {
      node = members[sortedPoints[lowerIndexes[i]][2]];
      hull.push([node.x, node.y]);
    }

    return hull;
  },

  tick: function() {
    var self = this;

    this.link.attr("d", function(d) { if (d.source.x && d.target.x) return 'M ' + d.source.x + " " + d.source.y + " L " + d.target.x + " " + d.target.y; });
    this.linkLabel.attr("transform", function(d, i){
        if (d.link.target.x < d.link.source.x){
          var bbox = this.getBBox();
          var rx = bbox.x + bbox.width / 2;
          var ry = bbox.y + bbox.height / 2;
          return "rotate(180 " + rx + " " + ry +")";
        }
        else {
          return "rotate(0)";
        }
    });

    this.linkWrap.attr('x1', function(d) { return d.link.source.x; })
      .attr('y1', function(d) { return d.link.source.y; })
      .attr('x2', function(d) { return d.link.target.x; })
      .attr('y2', function(d) { return d.link.target.y; });

    this.node.attr("transform", function(d) { return "translate(" + d.x + "," + d.y + ")"; });

    this.group.attrs(function(d) {
      if (d.type !== "ownership") return;

      var hull = self.convexHull(d);

      if (hull && hull.length) {
        return {
          'd': hull ? "M" + hull.join("L") + "Z" : d.d,
          'stroke-width': 64 + d.depth * 50,
        };
      } else {
        return { 'd': '' };
      }
    });
  },

  onNodeDragStart: function(d) {
    if (!d3.event.active) this.simulation.alphaTarget(0.05).restart();

    if (d3.event.sourceEvent.shiftKey && d.isGroupOwner()) {
      var i, members = d.group.memberArray.concat(d.group._memberArray);
      for (i = members.length - 1; i >= 0; i--) {
        members[i].fx = members[i].x;
        members[i].fy = members[i].y;
      }
    } else {
      d.fx = d.x;
      d.fy = d.y;
    }
  },

  onNodeDrag: function(d) {
    var dx = d3.event.x - d.fx, dy = d3.event.y - d.fy;

    if (d3.event.sourceEvent.shiftKey && d.isGroupOwner()) {
      var i, members = d.group.memberArray.concat(d.group._memberArray);
      for (i = members.length - 1; i >= 0; i--) {
        members[i].fx += dx;
        members[i].fy += dy;
      }
    } else {
      d.fx += dx;
      d.fy += dy;
    }
  },

  onNodeDragEnd: function(d) {
    if (!d3.event.active) this.simulation.alphaTarget(0);

    if (d.isGroupOwner()) {
      var i, members = d.group.memberArray.concat(d.group._memberArray);
      for (i = members.length - 1; i >= 0; i--) {
        if (!members[i].fixed) {
          members[i].fx = null;
          members[i].fy = null;
        }
      }
    } else {
      if (!d.fixed) {
        d.fx = null;
        d.fy = null;
      }
    }
  },

  stateSet: function(d) {
    this.g.select("#node-" + d.id).attr("class", this.nodeClass);
  },

  managerSet: function(d) {
    var size = this.nodeSize(d);
    var node = this.g.select("#node-" + d.id);

    node.append("circle")
    .attr("class", "manager")
    .attr("r", 12)
    .attr("cx", size - 2)
    .attr("cy", size - 2);

    node.append("image")
      .attr("class", "manager")
      .attr("x", size - 12)
      .attr("y", size - 12)
      .attr("width", 20)
      .attr("height", 20)
      .attr("xlink:href", this.managerImg(d));
  },

  isNeutronRelatedVMNode: function(d) {
    return d.Metadata.Manager === "neutron" && ["tun", "veth", "bridge"].includes(d.Metadata.Driver);
  },

  captureStarted: function(d) {
    var size = this.nodeSize(d);
    this.g.select("#node-" + d.id).append("image")
      .attr("class", "capture")
      .attr("x", -size - 8)
      .attr("y", size - 8)
      .attr("width", 16)
      .attr("height", 16)
      .attr("xlink:href", captureIndicatorImg);
  },

  captureStopped: function(d) {
    this.g.select("#node-" + d.id).select('image.capture').remove();
  },

  groupOwnerSet: function(d) {
    var self = this;

    var o = this.g.select("#node-" + d.id);

    o.append("image")
    .attr("class", "collapsexpand")
    .attr("width", 16)
    .attr("height", 16)
    .attr("x", function(d) { return -self.nodeSize(d) - 4; })
    .attr("y", function(d) { return -self.nodeSize(d) - 4; })
    .attr("xlink:href", this.collapseImg);
    o.select('circle').attr("r", this.nodeSize);
  },

  groupOwnerUnset: function(d) {
    var o = this.g.select("#node-" + d.id);
    o.select('image.collapsexpand').remove();
    o.select('circle').attr("r", this.nodeSize);
  },

  pinNode: function(d) {
    var size = this.nodeSize(d);
    this.g.select("#node-" + d.id).append("image")
      .attr("class", "pin")
      .attr("x", size - 12)
      .attr("y", -size - 4)
      .attr("width", 16)
      .attr("height", 16)
      .attr("xlink:href", pinIndicatorImg);
    d.fixed = true;
    d.fx = d.x;
    d.fy = d.y;
  },

  unpinNode: function(d) {
    this.g.select("#node-" + d.id).select('image.pin').remove();
    d.fixed = false;
    d.fx = null;
    d.fy = null;
  },

  onNodeShiftClick: function(d) {
    if (!d.fixed) {
      this.pinNode(d);
    } else {
      this.unpinNode(d);
    }
  },

  selectNode: function(d) {
    var circle = this.g.select("#node-" + d.id)
      .classed('selected', true)
      .select('circle');
    circle.transition().duration(500).attr('r', +circle.attr('r') + 3);
    d.selected = true;
    this.selectedNode = d;
  },

  unselectNode: function(d) {
    var circle = this.g.select("#node-" + d.id)
      .classed('selected', false)
      .select('circle');
    if (!circle) return;
    circle.transition().duration(500).attr('r', circle ? +circle.attr('r') - 3 : 0);
    d.selected = false;
    this.selectedNode = null;
  },

  onNodeClick: function(d) {
    if (this.selectedEdge) {
      this.selectedEdge = null;
      this.notifyHandlers('edgeSelected', this.selectedEdge);
    }

    if (d3.event.shiftKey) return this.onNodeShiftClick(d);
    if (d3.event.altKey) return this.collapseByNode(d);

    if (this.selectedNode === d) return;

    if (this.selectedNode) this.unselectNode(this.selectedNode);
    this.selectNode(d);

    this.notifyHandlers('nodeSelected', d);
  },

  onEdgeClick: function(e) {
    if (this.selectedNode) {
      this.unselectNode(this.selectedNode);
      this.notifyHandlers('nodeSelected', this.selectedNode);
    }

    if(e.collapse) return;
    if(this.selectedEdge === e) return;
    this.selectedEdge = e;
    this.notifyHandlers('edgeSelected', e);
  },

  addCollapseLink: function(group, source, target, metadata) {
    var id = source.id < target.id ? source.id + '-' + target.id : target.id + '-' + source.id;
    if (!this.links[id]) {
      var link = {
        id: id,
        source: source,
        target: target,
        Metadata: metadata,
        collapse: {
          group: group
        }
      };
      group.collapseLinks.push(link);

      if (!source.links) source.links = {};
      source.links[id] = link;

      if (!target.links) target.links = {};
      target.links[id] = link;

      this.links[id] = link;
    }
  },

  delCollapseLinks: function(group, node) {
    var i, e, cl = [];
    if(!group.collapseLinks) return;
    for (i = group.collapseLinks.length - 1; i >= 0; i--) {
      e = group.collapseLinks[i];
      if (!node || e.source === node || e.target === node) {
        this.delLink(e);
      } else {
        cl.push(e);
      }
    }
    group.collapseLinks = cl;
  },

  collapseNode: function(n, group) {
    if (n === group.owner) return;

    var i, e, source, target, edges = n.edges,
        members = group.memberArray.concat(group._memberArray);
    for (i in edges) {
      e = edges[i];

      if (e.Metadata.RelationType === "ownership") continue;

      if (members.indexOf(e.source) < 0 || members.indexOf(e.target) < 0) {
        source = e.source; target = e.target;
        if (e.source.group === group) {
          // group already collapsed, link owners together, delete old collapse links
          // that were present between these two groups
          if (e.target.group && e.target.group.collapsed) {
            this.delCollapseLinks(e.target.group, source);
            target = e.target.group.owner;
          }
          source = group.owner;
        } else {
          // group already collapsed, link owners together, delete old collapse links
          // that were present between these two groups
          if (e.source.group && e.source.group.collapsed) {
            this.delCollapseLinks(e.source.group, target);
            source = e.source.group.owner;
          }
          target = group.owner;
        }

        if (!source.group || !target.group || (source.group.owner.visible && target.group.owner.visible)) {
          this.addCollapseLink(group, source, target, e.Metadata);
        }
      }
    }

    this.hideNode(n);
  },

  collapseGroup: function(group) {
    var i, children = group.children;
    for (i in children) {
      if (children[i].collapsed) this.uncollapseGroup(children[i]);
    }

    for (i = group.memberArray.length - 1; i >= 0; i--) {
      this.collapseNode(group.memberArray[i], group);
    }
    group.collapsed = true;

    this.g.select("#node-" + group.owner.id)
      .attr('collapsed', group.collapsed)
      .select('image.collapsexpand')
      .attr('xlink:href', this.collapseImg);
  },

  uncollapseNode: function(n, group) {
    if (n === group.owner) return;

    var i, e, link, source, target, edges = n.edges;
        members = group.memberArray.concat(group._memberArray);
    for (i in edges) {
      e = edges[i];

      if (e.Metadata.RelationType === "ownership") continue;

      if (members.indexOf(e.source) < 0 || members.indexOf(e.target) < 0) {
        source = e.source; target = e.target;
        if (source.group === group && target.group) {
          this.delCollapseLinks(target.group, group.owner);
          if (target.group.collapsed) {
            this.addCollapseLink(group, source, target.group.owner, e.Metadata);
          }
        } else if (source.group) {
          this.delCollapseLinks(source.group, group.owner);
          if (source.group.collapsed) {
            this.addCollapseLink(group, source.group.owner, target, e.Metadata);
          }
        }
      }
    }

    this.showNode(n);
  },

  uncollapseGroupTree: function(group) {
    this.delCollapseLinks(group);

    var i;
    for (i = group._memberArray.length -1; i >= 0; i--) {
      this.uncollapseNode(group._memberArray[i], group);
    }
    group.collapsed = false;

    var children = group.children;
    for (i in children) {
      this.uncollapseGroupTree(children[i]);
    }
    this.g.select("#node-" + group.owner.id)
      .attr('collapsed', group.collapsed)
      .select('image.collapsexpand')
      .attr('xlink:href', this.collapseImg);
  },

  collapseGroupTree: function(group) {
    var i, children = group.children;
    for (i in children) {
      if (children[i].collapsed) this.collapseGroupTree(children[i]);
    }

    for (i = group.memberArray.length - 1; i >= 0; i--) {
      this.collapseNode(group.memberArray[i], group);
    }
    group.collapsed = true;

    this.g.select("#node-" + group.owner.id)
      .attr('collapsed', group.collapsed)
      .select('image.collapsexpand')
      .attr('xlink:href', this.collapseImg);
  },

  uncollapseGroup: function(group) {
    this.delCollapseLinks(group);

    var i;
    for (i = group._memberArray.length - 1; i >= 0; i--) {
      this.uncollapseNode(group._memberArray[i], group);
    }
    group.collapsed = false;

    // collapse children
    var children = group.children;
    for (i in children) {
      this.collapseGroup(children[i]);
    }

    this.g.select("#node-" + group.owner.id)
      .attr('collapsed', group.collapsed)
      .select('image.collapsexpand')
      .attr('xlink:href', this.collapseImg);
  },

  toggleExpandAll: function(d) {
    if (d.isGroupOwner()) {
      if(!d.group.collapsed) {
        this.collapseGroupTree(d.group);
      } else {
        this.uncollapseGroupTree(d.group);
      }
    }
    this.update();
  },

  collapseByNode: function(d) {
    if (d.isGroupOwner()) {
      if(d.group){
        if (!d.group.collapsed) {
          this.collapseGroup(d.group);
        } else {
          this.uncollapseGroup(d.group);
        }
      }
    }

    this.update();
  },

  autoExpand: function(auto) {
    this._autoExpand = auto;
    this.invalid = true;
  },

  collapse: function(collapse) {
    this.collapsed = collapse;
    this.defaultCollpsed = collapse;

    var i;
    for (i = this.groups.length - 1; i >= 0; i--) {
      if (collapse) {
        this.collapseGroup(this.groups[i]);
      } else if (!this.groups[i].parent) {
        this.uncollapseGroup(this.groups[i]);
      }
    }

    this.update();
  },

  toggleCollapseByLevel: function(collapse) {
    if (collapse) {
      if (this.collapseLevel === 0) {
        return false;
      } else {
        this.collapseLevel--;
      }
      this.collapseByLevel(this.collapseLevel, collapse, this.groups);
    } else {
      var maxLevel = 0;
      for (var i in this.groups) {
        var group = this.groups[i];
        if (group.level > maxLevel) maxLevel = group.level;
      }
      if (maxLevel === 0) {
        return false;
      }
      if ((this.collapseLevel) >= maxLevel) {
        return false;
      }

      this.collapseByLevel(this.collapseLevel, collapse, this.groups);
      this.collapseLevel++;
    }
    return true;
  },

  collapseByLevel: function(level, collapse, groups) {
    var i;
    if (level === 0) {
      for (i = groups.length - 1; i >= 0; i--) {
        if (collapse) {
          this.collapseGroup(groups[i]);
        } else {
          this.uncollapseGroup(groups[i]);
        }
      }
      this.update();
    } else {
      for (i = groups.length - 1; i >= 0; i--) {
        this.collapseByLevel((level-1), collapse, Object.values(groups[i].children));
      }
    }
  },

  loadBandwidthConfig: function() {
    var b = this.bandwidth;

    var cfgNames = {
      relative: ['bandwidth_relative_active',
                 'bandwidth_relative_warning',
                 'bandwidth_relative_alert'],
      absolute: ['bandwidth_absolute_active',
                 'bandwidth_absolute_warning',
                 'bandwidth_absolute_alert']
    };

    var cfgValues = {
      absolute: [0, 0, 0],
      relative: [0, 0, 0]
    };

    if (typeof(Storage) !== "undefined") {
      cfgValues = {
        absolute: [app.getLocalValue("bandwidthAbsoluteActive"),
                   app.getLocalValue("bandwidthAbsoluteWarning"),
                   app.getLocalValue("bandwidthAbsoluteAlert")],
        relative: [app.getLocalValue("bandwidthRelativeActive"),
                   app.getLocalValue("bandwidthRelativeWarning"),
                   app.getLocalValue("bandwidthRelativeAlert")]
      };
    }

    b.updatePeriod = app.getConfigValue('bandwidth_update_rate') * 1000; // in millisec
    b.bandwidthThreshold = app.getConfigValue('bandwidth_threshold');
    b.active = app.getConfigValue(cfgNames[b.bandwidthThreshold][0]);
    b.warning = app.getConfigValue(cfgNames[b.bandwidthThreshold][1]);
    b.alert = app.getConfigValue(cfgNames[b.bandwidthThreshold][2]);
  },

  styleReturn: function(d, values) {
    if (d.active)
      return values[0];
    if (d.warning)
      return values[1];
    if (d.alert)
      return values[2];
    return values[3];
  },

  styleStrokeDasharray: function(d) {
    return this.styleReturn(d, ["20", "20", "20", ""]);
  },

  styleStrokeDashoffset: function(d) {
    return this.styleReturn(d, ["80 ", "80", "80", ""]);
  },

  styleAnimation: function(d) {
    var animate = function(speed) {
      return "dash "+speed+" linear forwards infinite";
    };
    return this.styleReturn(d, [animate("6s"), animate("3s"), animate("1s"), ""]);
  },

  styleStroke: function(d) {
    return this.styleReturn(d, ["YellowGreen", "Yellow", "Tomato", ""]);
  },

  bindLinkLabelData: function() {
    this.linkLabel = this.linkLabel.data(Object.values(this.linkLabelData), function(d) { return d.id; });
  },

  updateLinkLabelData: function() {
    var self = this;

    const driver = self.linkLabelFactory();

    for (var i in this.links) {
      var link = this.links[i];

      if (!link.source.visible || !link.target.visible)
        continue;
      if (link.Metadata.RelationType !== "layer2")
        continue;

      driver.updateData(link);

      if (driver.hasData(link)) {
        this.linkLabelData[link.id] = {
          id: "link-label-" + link.id,
          link: link,
          text: driver.getText(link),
          active: driver.isActive(link),
          warning: driver.isWarning(link),
          alert: driver.isAlert(link),
        };
      } else {
        delete this.linkLabelData[link.id];
      }
    }
  },

  updateLinkLabelHandler: function() {
    var self = this;

    this.updateLinkLabelData();
    this.bindLinkLabelData();

    // update links which don't have traffic
    var exit = this.linkLabel.exit();
    exit.each(function(d) {
      self.g.select("#link-" + d.link.id)
      .classed("link-label-active", false)
      .classed("link-label-warning", false)
      .classed("link-label-alert", false)
      .style("stroke-dasharray", "")
      .style("stroke-dashoffset", "")
      .style("animation", "")
      .style("stroke", "");
    });
    exit.remove();

    var enter = this.linkLabel.enter()
      .append('text')
      .attr("id", function(d) { return "link-label-" + d.id; })
      .attr("class", "link-label");
    enter.append('textPath')
      .attr("startOffset", "50%")
      .attr("xlink:href", function(d) { return "#link-" + d.link.id; } );
    this.linkLabel = enter.merge(this.linkLabel);

    this.linkLabel.select('textPath')
      .classed("link-label-active", function(d) { return d.active; })
      .classed("link-label-warning", function(d) { return d.warning; })
      .classed("link-label-alert",  function(d) { return d.alert; })
      .text(function(d) { return d.text; });

    this.linkLabel.each(function(d) {
      self.g.select("#link-" + d.link.id)
        .classed("link-label-active", d.active)
        .classed("link-label-warning", d.warning)
        .classed("link-label-alert", d.alert)
        .style("stroke-dasharray", self.styleStrokeDasharray(d))
        .style("stroke-dashoffset", self.styleStrokeDashoffset(d))
        .style("animation", self.styleAnimation(d))
        .style("stroke", self.styleStroke(d));
    });

    // force a tick
    this.tick();
  },

  delLinkLabel: function(link) {
    if (!(link.id in this.linkLabelData))
      return;
    delete this.linkLabelData[link.id];

    this.bindLinkLabelData();
    this.linkLabel.exit().remove();

    // force a tick
    this.tick();
  },

  arrowhead: function(link) {
    let none = "url(#arrowhead-none)";

    if (link.source.Metadata.Type !== "networkpolicy") {
      return none
    }

    if (link.target.Metadata.Type !== "pod") {
      return none
    }

    if (link.Metadata.RelationType !== "networkpolicy") {
      return none
    }

    return "url(#arrowhead-"+link.Metadata.PolicyType+"-"+link.Metadata.PolicyTarget+"-"+link.Metadata.PolicyPoint+")";
  },

  update: function() {
    var self = this;

    var nodes = Object.values(this.nodes), links = Object.values(this.links);

    var linkWraps = [];
    for (var i in links) {
      linkWraps.push({link: links[i]});
    }

    this.node = this.node.data(nodes, function(d) { return d.id; });
    this.node.exit().remove();

    var nodeEnter = this.node.enter()
      .append("g")
      .attr("class", this.nodeClass)
      .attr("id", function(d) { return "node-" + d.id; })
      .on("click", this.onNodeClick.bind(this))
      .on("dblclick", this.collapseByNode.bind(this))
      .call(d3.drag()
        .on("start", this.onNodeDragStart.bind(this))
        .on("drag", this.onNodeDrag.bind(this))
        .on("end", this.onNodeDragEnd.bind(this)));

    nodeEnter.append("circle")
      .attr("r", this.nodeSize);

    // node picto
    nodeEnter.append("image")
      .attr("id", function(d) { return "node-img-" + d.id; })
      .attr("class", "picto")
      .attr("x", -12)
      .attr("y", -12)
      .attr("width", "24")
      .attr("height", "24")
      .attr("xlink:href", this.nodeImg);

    // node rectangle
    nodeEnter.append("rect")
      .attr("class", "node-text-rect")
      .attr("width", function(d) { return self.nodeTitle(d).length * 10 + 10; })
      .attr("height", 25)
      .attr("x", function(d) {
        return self.nodeSize(d) * 1.6 - 5;
      })
      .attr("y", -8)
      .attr("rx", 4)
      .attr("ry", 4);

    // node title
    nodeEnter.append("text")
      .attr("dx", function(d) {
        return self.nodeSize(d) * 1.6;
      })
      .attr("dy", 10)
      .text(this.nodeTitle);

    nodeEnter.filter(function(d) { return d.isGroupOwner(); })
      .each(this.groupOwnerSet.bind(this));

    nodeEnter.filter(function(d) { return d.Metadata.Capture; })
      .each(this.captureStarted.bind(this));

    nodeEnter.filter(function(d) { return d.Metadata.Manager; })
      .each(this.managerSet.bind(this));

    nodeEnter.filter(function(d) { return d._emphasized; })
      .each(this.emphasizeNode.bind(this));

    this.node = nodeEnter.merge(this.node);

    this.link = this.link.data(links, function(d) { return d.id; });
    this.link.exit().remove();

    var linkEnter = this.link.enter()
      .append("path")
      .attr("id", function(d) { return "link-" + d.id; })
      .on("click", this.onEdgeClick.bind(this))
      .on("mouseover", this.highlightLink.bind(this))
      .on("mouseout", this.unhighlightLink.bind(this))
      .attr("class", this.linkClass);

    this.link = linkEnter.merge(this.link);

    this.linkWrap = this.linkWrap.data(linkWraps, function(d) { return d.link.id; });
    this.linkWrap.exit().remove();

    var linkWrapEnter = this.linkWrap.enter()
      .append("line")
      .attr("id", function(d) { return "link-wrap-" + d.link.id; })
      .on("click", function(d) {self.onEdgeClick(d.link); })
      .on("mouseover", function(d) { self.highlightLink(d.link); })
      .on("mouseout", function(d) { self.unhighlightLink(d.link); })
      .attr("class", this.linkWrapClass)
      .attr("marker-end", function(d) { return self.arrowhead(d.link); });

    this.linkWrap = linkWrapEnter.merge(this.linkWrap);

    this.group = this.group.data(this.groups, function(d) { return d.id; });
    this.group.exit().remove();

    var groupEnter = this.group.enter()
      .append("path")
      .attr("class", this.groupClass)
      .attr("id", function(d) { return "group-" + d.id; });

    this.group = groupEnter.merge(this.group).order();

    this.simulation.nodes(nodes);
    this.simulation.force("link").links(links);
    this.simulation.alpha(1).restart();
  },

  highlightLink: function(d) {
    if(d.collapse) return;
    var t = d3.transition()
      .duration(300)
      .ease(d3.easeLinear);
    this.g.select("#link-wrap-" + d.id).transition(t).style("stroke", "rgba(30, 30, 30, 0.15)");
    this.g.select("#link-" + d.id).transition(t).style("stroke-width", 2);
  },

  unhighlightLink: function(d) {
    if(d.collapse) return;
    var t = d3.transition()
      .duration(300)
      .ease(d3.easeLinear);
    this.g.select("#link-wrap-" + d.id).transition(t).style("stroke", null);
    this.g.select("#link-" + d.id).transition(t).style("stroke-width", null);
  },

  groupClass: function(d) {
    var clazz = "group " + d.ownerType;

    if (d.owner.Metadata.Probe) clazz += " " + d.owner.Metadata.Probe;

    return clazz;
  },

  highlightNodeID: function(id) {
    var self = this;

    if (id in this.nodes) this.nodes[id]._highlighted = true;
    if (id in this._nodes) this._nodes[id]._highlighted = true;

    if (!this.g.select("#node-highlight-" + id).empty()) return;

    this.g.select("#node-" + id)
      .insert("circle", ":first-child")
      .attr("id", "node-highlight-" + id)
      .attr("class", "highlighted")
      .attr("r", function(d) { return self.nodeSize(d) + 16; });
  },

  unhighlightNodeID: function(id) {
    if (id in this.nodes) this.nodes[id]._highlighted = false;
    if (id in this._nodes) this._nodes[id]._highlighted = false;

    this.g.select("#node-highlight-" + id).remove();
  },

  emphasizeNodeID: function(id) {
    var self = this;

    if (id in this.nodes) this.nodes[id]._emphasized = true;
    if (id in this._nodes) this._nodes[id]._emphasized = true;

    if (!this.g.select("#node-emphasize-" + id).empty()) return;

    var circle;
    if (this.g.select("#node-highlight-" + id).empty()) {
      circle = this.g.select("#node-" + id).insert("circle", ":first-child");
    } else {
      circle = this.g.select("#node-" + id).insert("circle", ":nth-child(2)");
    }

    circle.attr("id", "node-emphasize-" + id)
      .attr("class", "emphasized")
      .attr("r", function(d) { return self.nodeSize(d) + 8; });
  },

  deemphasizeNodeID: function(id) {
    if (id in this.nodes) this.nodes[id]._emphasized = false;
    if (id in this._nodes) this._nodes[id]._emphasized = false;

    this.g.select("#node-emphasize-" + id).remove();
  },

  emphasizeNode: function(d) {
    this.emphasizeNodeID(d.id);
  },

  nodeClass: function(d) {
    var clazz = "node " + d.Metadata.Type;

    if (d.Metadata.Probe) clazz += " " + d.Metadata.Probe;
    if (d.Metadata.State == "DOWN") clazz += " down";
    if (d.highlighted) clazz += " highlighted";
    if (d.selected) clazz += " selected";

    return clazz;
  },

  linkClass: function(d) {
    var clazz = "link " + d.Metadata.RelationType;

    if (d.Metadata.Type) clazz += " " + d.Metadata.Type;

    if (!d.collapse) clazz += " real-edge";

    return clazz;
  },

  linkWrapClass: function(d) {
    var clazz = "link-wrap";

    if (!d.link.collapse) clazz += " real-edge-wrap";
    return clazz;
  },

  nodeTitle: function(d) {
    if (d.Metadata.Type === "host") {
      return d.Metadata.Name.split(".")[0];
    }
    return d.Metadata.Name ? d.Metadata.Name.length > 12 ? d.Metadata.Name.substr(0, 12)+"..." : d.Metadata.Name : "";
  },

  nodeSize: function(d) {
    var size;
    switch(d.Metadata.Type) {
      case "host": size = 30; break;
      case "netns": size = 26; break;
      case "port":
      case "ovsport": size = 22; break;
      case "switch":
      case "ovsbridge": size = 24; break;
      default:
        size = d.isGroupOwner() ? 26 : 20;
    }
    if (d.selected) size += 3;

    return size;
  },

  nodeImg: function(d) {
    var t = d.Metadata.Type || "default";
    return (t in nodeImgMap) ? nodeImgMap[t] : nodeImgMap["default"];
  },

  managerImg: function(d) {
    var t = d.Metadata.Orchestrator || d.Metadata.Manager || "default";
    return (t in managerImgMap) ? managerImgMap[t] : managerImgMap["default"];
  },

  collapseImg: function(d) {
    if (d.group && d.group.collapsed) return plusImg;
    return minusImg;
  }

};



/* jshint multistr: true */

var topologyComponent

var TopologyComponentOldApproach = {

  name: 'topology',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div class="topology">\
      <div class="col-sm-7 fill content">\
        <div class="topology-d3">\
          <div class="topology-legend">\
            <strong>Topology view</strong></br>\
            <p v-if="currTopologyFilter">{{currTopologyFilter}}</p>\
            <p v-else>Full</p>\
            <p v-if="topologyHumanTimeContext">{{topologyHumanTimeContext}}</p>\
            <p v-else>Live</p>\
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
                  <div class="switch switch--vertical">\
                    <input id="radio-abs" name="time-type" type="radio" value="absolute" v-model="timeType" checked="checked" :disabled="topologyMode === \'live\'"/>\
                    <label for="radio-abs">Absolute</label>\
                    <input id="radio-rel" name="time-type" type="radio" value="relative" v-model="timeType" :disabled="topologyMode === \'live\'"/>\
                    <label for="radio-rel">Relative</label>\
                    <span class="toggle-outside">\
                    <span class="toggle-inside"></span></span>\
                  </div>\
                  <datepicker id="topology-datepicker" :calendar-class="\'topology-datepicker\'" \
                    :format="\'dd/MM/yyyy\'" placeholder="dd/MM/yyyy" \
                    v-on:opened="topologyOptionsExpanded" \
                    v-on:closed="topologyOptionsCollapsed" \
                    v-model="topologyDate" input-class="input-sm form-control"\
                    :disabled-picker="topologyMode === \'live\'"\
                    v-if="timeType === \'absolute\'"></datepicker> \
                  <input id="topology-timepicker" placeholder="HH:mm:ss" \
                    v-model="topologyTime" class="input-sm form-control" \
                    :disabled="topologyMode === \'live\'" style="margin-left: 10px" \
                    @keyup.enter="topologyTimeTravel"\
                    v-if="timeType === \'absolute\'"></input>\
                  <input id="topology-rel-time" placeholder="2h30m" \
                    v-model="topologyRelTime" class="input-sm form-control" \
                    :disabled="topologyMode === \'live\'" style="margin-left: 10px"\
                    v-if="timeType === \'relative\'"></input>\
                  <span class="help-btn" v-if="timeType === \'relative\'" title="relative time: ex 1d2h3m5s or 2h5m or 30m">&#8253;</span>\
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
                    <input list="topology-gremlin-favorites" placeholder="e.g. g.V().Has(,)"\
                      id="topology-filter" type="text" style="width: 400px"\
                      v-model="topologyFilter" @keyup.enter="topologyFilterQuery"\
                      v-on:input="onFilterDatalistSelect"\
                      class="input-sm form-control">\
                    </input>\
                    <span class="clear-btn" @click.stop="topologyFilterClear">&times;</span>\
                    <datalist id="topology-gremlin-favorites" class="topology-gremlin-favorites">\
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
                    v-model="topologyEmphasize" @keyup.enter="emphasizeGremlinExpr" \
                    v-on:input="onFilterDatalistSelect" \
                    class="input-sm form-control"></input>\
                    <span class="clear-btn" @click.stop="topologyEmphasizeClear">&times;</span>\
                    <datalist id="topology-highlight-list" class="topology-gremlin-favorites">\
                    </datalist>\
                  </div>\
                </div>\
            </div>\
          </div>\
          <div style="margin-top: 10px">\
            <div class="trigger">\
              <button @mouseenter="showTopologyOptions" @mouseleave="clearTopologyTimeout" @click="hideTopologyOptions">\
                <span :class="[\'glyphicon\', isTopologyOptionsVisible ? \'glyphicon-remove\' : \'glyphicon-filter\']" aria-hidden="true"></span>\
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
          <button v-if="currentNode != null && currentNode.isGroupOwner()" id="expand-all" type="button" class="btn btn-primary" \
                  title="Expand/Collapse Current Node Tree" @click="toggleExpandAll(currentNode)">\
            <span class="expand-icon-stack">\
              <span v-if="currentNode.group != null" class="glyphicon icon-main" \
                 :class="{\'glyphicon-resize-full\': currentNode.group.collapsed, \'glyphicon-resize-small\': !currentNode.group.collapsed}" aria-hidden="true"></span>\
            </span>\
          </button>\
        </div>\
      </div>\
      <div id="info-panel" class="col-sm-5 sidebar">\
        <tabs v-if="isAnalyzer" :active="!canReadCaptures ? 2 : 0">\
          <tab-pane title="Captures" v-if="canReadCaptures">\
            <capture-list></capture-list>\
            <capture-form v-if="canWriteCaptures && topologyMode ===  \'live\'"></capture-form>\
          </tab-pane>\
          <tab-pane title="Generator" v-if="topologyMode ===  \'live\' && canInjectPackets">\
            <injection-list></injection-list>\
            <inject-form></inject-form>\
          </tab-pane>\
          <tab-pane title="Flows">\
            <flow-table-control></flow-table-control>\
          </tab-pane>\
          <tab-pane title="Alerts">\
            <alert-list></alert-list>\
            <alert-form></alert-form>\
          </tab-pane>\
          <tab-pane title="Workflows">\
            <workflow-call></workflow-call>\
          </tab-pane>\
          <tab-pane title="Topology rules" v-if="topologyMode === \'live\'">\
            <topology-rules></topology-rules>\
          </tab-pane>\
        </tabs>\
        <panel id="node-metadata" v-if="currentNodeMetadata"\
               :collapsed="false"\
               title="Metadata">\
          <template slot="actions">\
            <button v-if="currentNode.isGroupOwner()"\
                    title="Expand/Collapse Node"\
                    class="btn btn-default btn-xs"\
                    @click.stop="toggleExpandAll(currentNode)">\
              <i class="node-action fa"\
                 :class="{\'fa-expand\': currentNode.group.collapsed, \'fa-compress\': !currentNode.group.collapsed}" />\
            </button>\
          </template>\
          <object-detail :object="currentNodeMetadata"\
                         :links="metadataLinks(currentNodeMetadata)"\
                         :collapsed="metadataCollapseState">\
          </object-detail>\
        </panel>\
        <panel id="edge-metadata" v-if="currentEdge"\
               title="Metadata">\
          <object-detail :object="currentEdge.Metadata"></object-detail>\
        </panel>\
        <panel id="docker-metadata" v-if="currentNodeDocker"\
               title="Docker">\
          <object-detail :object="currentNodeDocker"></object-detail>\
        </panel>\
        <panel id="k8s-metadata" v-if="currentNodeK8s"\
               title="K8s">\
          <object-detail :object="currentNodeK8s"></object-detail>\
        </panel>\
        <panel id="feature-table" v-if="currentNodeFeatures"\
               title="Features">\
          <feature-table :features="currentNodeFeatures"></feature-table>\
        </panel>\
        <panel id="ovs-rules" v-if="currentNodeMetadata && currentNodeMetadata.Type == \'ovsbridge\'"\
               title="Rules">\
          <rule-detail :bridge="currentNode" :graph="graph"></rule-detail>\
        </panel>\
        <panel id="total-metric" v-if="currentNodeMetric"\
               title="Metrics">\
          <h2>Total metrics</h2>\
          <metrics-table :object="currentNodeMetric" :keys="globalVars[\'interface-metric-keys\']"></metrics-table>\
          <div v-show="currentNodeLastUpdateMetric && topologyTimeContext === 0">\
            <h2>Last metrics</h2>\
            <metrics-table :object="currentNodeLastUpdateMetric" :keys="globalVars[\'interface-metric-keys\']" \
              :defaultKeys="[\'Last\', \'Start\', \'RxBytes\', \'RxPackets\', \'TxBytes\', \'TxPackets\']"></metrics-table>\
          </div>\
        </panel>\
        <panel id="ovs-metric" v-if="currentNodeOvsMetric"\
               title="OVS metrics">\
          <h2>Total metrics</h2>\
          <metrics-table :object="currentNodeOvsMetric" :keys="globalVars[\'interface-metric-keys\']"></metrics-table>\
          <div v-show="currentNodeOvsLastUpdateMetric && topologyTimeContext === 0">\
            <h2>Last metrics</h2>\
            <metrics-table :object="currentNodeOvsLastUpdateMetric" :keys="globalVars[\'interface-metric-keys\']" \
              :defaultKeys="[\'Last\', \'Start\', \'RxBytes\', \'RxPackets\', \'TxBytes\', \'TxPackets\']"></metrics-table>\
          </div>\
        </panel>\
        <panel id="routing-tabel" v-if="currentNodeMetadata && currentNode.Metadata.RoutingTables"\
               title="Routing tables">\
          <div v-for="rt in currentNode.Metadata.RoutingTables">\
            <h2>src: {{rt.Src || "none"}}<span class="pull-right">(id: {{rt.Id}})</span></h2>\
            <routing-table :rt="rt"></routing-table>\
          </div>\
        </panel>\
        <panel id="flow-table" v-if="isAnalyzer && currentNodeFlowsQuery"\
               title="Flows">\
          <flow-table :value="currentNodeFlowsQuery"></flow-table>\
        </panel>\
      </div>\
    </div>\
  ',

  data: function() {
    return {
      dynamicFilter: [],
      topologyTimeContext: 0,
      topologyTime: null,
      topologyDate: '',
      collapsed: false,
      topologyFilter: "",
      currTopologyFilter: "",
      topologyEmphasize: "",
      topologyMode: "live",
      topologyHumanTimeContext: "",
      isTopologyOptionsVisible: false,
      timeType: "absolute",
      topologyRelTime: "1m",
      metadataCollapseState: {
        IPV4: false,
        IPV6: false,
        LinkFlags: false,
        'Neutron.IPV4': false,
        'Neutron.IPV6': false,
      },
    };
  },

  mounted: function() {
    topologyComponent = this;
    var self = this;

    // run d3 layout
    this.graph = new Graph(websocket, function(err) {
      this.$error({
        message: err
      });
    }.bind(this));

    this.layout = new TopologyGraphLayout(this, ".topology-d3");

    this.graph.addHandler(this.layout);
    this.graph.addHandler(this);
    this.layout.addHandler(this);

    this.emphasize = debounce(self.emphasizeGremlinExpr.bind(self), 300);
    var emphasizeWatcher = {
      onEdgeAdded: this.emphasize,
      onNodeAdded: this.emphasize,
    };
    this.graph.addHandler(emphasizeWatcher);

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

    // trigered when some component wants to highlight/emphasize some nodes
    this.$store.subscribe(function(mutation) {
      if (mutation.type === "highlight")
        self.layout.highlightNodeID(mutation.payload);
      else if (mutation.type === "unhighlight")
        self.layout.unhighlightNodeID(mutation.payload);
      else if (mutation.type === "emphasize")
        self.layout.emphasizeNodeID(mutation.payload);
      else if (mutation.type === "deemphasize")
        self.layout.deemphasizeNodeID(mutation.payload);
    });

    this.setGremlinFavoritesFromConfig();

    if (typeof(this.$route.query.highlight) !== "undefined") {
      this.topologyEmphasize = this.$route.query.highlight;
      setTimeout(function(){self.emphasizeGremlinExpr();}, 2000);
    }

    if (typeof(this.$route.query.filter) !== "undefined") {
      this.topologyFilter = this.$route.query.filter;
    }

    if (typeof(this.$route.query.expand) !== "undefined") {
      this.layout.autoExpand(true);
    } else {
      this.collapse();
      websocket.addConnectHandler(function() {
        self.collapse();
      });
    }

    if (typeof(this.$route.query.link_label_type) !== "undefined") {
      this.layout.linkLabelType = this.$route.query.link_label_type;
    }

    if (typeof(this.$route.query.topology_legend_hide) !== "undefined") {
      $('.topology-legend').remove();
    }

    websocket.addConnectHandler(function() {
      if (self.topologyFilter !== '') {
        self.topologyFilterQuery();
      }
    });

    if (self.isK8SEnabled()) {
      self.setk8sNamespacesFilter();
    }
  },

  beforeDestroy: function() {
    this.$store.commit('nodeUnselected');
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
        this.topologyDate = dt;
        this.topologyTime = dt.getHours() + ":" + dt.getMinutes() + ":" + dt.getSeconds();
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

    timeType: function(val) {
      this.topologyTimeTravel();
    }
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

    currentNodeMetadata: function() {
      if (!this.currentNode) return null;
      return this.extractMetadata(this.currentNode.Metadata,
        ['LastUpdateMetric', 'Metric', 'Ovs.Metric', 'Ovs.LastUpdateMetric', 'RoutingTables', 'Features', 'K8s', 'Docker']);
    },

    currentNodeFlowsQuery: function() {
      if (this.currentNodeMetadata && this.currentNode.isCaptureAllowed())
        return "G.Flows().Has('NodeTID', '" + this.currentNode.Metadata.TID + "').Sort()";
      return null;
    },

    currentNodeDocker: function() {
      if (!this.currentNodeMetadata || !this.currentNode.Metadata.Docker) return null;
      return this.currentNode.Metadata.Docker;
    },

    currentNodeK8s: function() {
      if (!this.currentNodeMetadata || !this.currentNode.Metadata.K8s) return null;
      return this.currentNode.Metadata.K8s;
    },
 
    currentNodeFeatures: function() {
      if (!this.currentNodeMetadata || !this.currentNode.Metadata.Features) return null;
      return this.currentNode.Metadata.Features;
    },

    currentNodeMetric: function() {
      if (!this.currentNodeMetadata || !this.currentNode.Metadata.Metric) return null;
      return this.normalizeMetric(this.currentNode.Metadata.Metric);
    },

    currentNodeLastUpdateMetric: function() {
      if (!this.currentNodeMetadata || !this.currentNode.Metadata.LastUpdateMetric) return null;
      return this.normalizeMetric(this.currentNode.Metadata.LastUpdateMetric);
    },

    currentNodeOvsMetric: function() {
      if (!this.currentNodeMetadata || !this.currentNode.Metadata.Ovs || !this.currentNode.Metadata.Ovs.Metric) return null;
      return this.normalizeMetric(this.currentNode.Metadata.Ovs.Metric);
    },

    currentNodeOvsLastUpdateMetric: function() {
      if (!this.currentNodeMetadata || !this.currentNode.Metadata.Ovs || !this.currentNode.Metadata.Ovs.LastUpdateMetric) return null;
      return this.normalizeMetric(this.currentNode.Metadata.Ovs.LastUpdateMetric);
    },

    canReadCaptures: function() {
      return app.enforce("capture", "read");
    },

    canWriteCaptures: function() {
      return app.enforce("capture", "write");
    },

    canInjectPackets: function() {
      return app.enforce("injectpacket", "write");
    },

  },

  methods: {

    onPostInit: function() {
      setTimeout(this.emphasize.bind(this), 1000);
    },

    isSSHEnabled: function() {
      return app.getConfigValue('ssh_enabled');
    },

    isK8SEnabled: function() {
      return app.getConfigValue('k8s_enabled')
    },

    metadataLinks: function(m) {
      var self = this;

      var links = {};

      if (m.Type === "host" && this.isSSHEnabled()) {
        links.Name = {
          "class": "indicator glyphicon glyphicon-new-window raw-packet-link",
          "onClick": function() {
            window.open(location.protocol + '//' + location.host + '/dede/terminal/'+m.Name+ '?title='+m.Name+'&cmd=ssh ' + m.Name, '_blank').focus();
          },
          "onMouseOver": function(){},
          "onMouseOut": function(){}
        };
      }

      return links;
    },

    unwatch: function() {
      clearTimeout(this.timeId);
      this.timeId = null;
    },

    onNodeSelected: function(d) {
      this.$store.commit('nodeSelected', d);
    },

    onEdgeSelected: function(e) {
      this.$store.commit('edgeSelected', e);
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

    topologyEmphasizeClear: function () {
      this.topologyEmphasize = '';
      this.emphasizeGremlinExpr();
     },

    endsWith: function (str, suffix) {
       return str.indexOf(suffix, str.length - suffix.length) !== -1;
    },

    setk8sNamespacesFilter: function() {
      var self = this;
      // get k8s namespaces using API
      this.$topologyQuery("G.V().Has('Manager','k8s','Type', 'namespace')")
        .then(function(data) {
          var namespacesAsJsonFilter = '{"Name":"k8s namespace", "Type":"combobox", "value":{'
          data.forEach(function(namespace) {
            var namespaceName = namespace["Metadata"]["Name"]
            var namespaceGremlin = 'G.V().Has(\'Namespace\',\'' + namespaceName + '\')'
            namespacesAsJsonFilter += '"' + namespaceName + '":"' + namespaceGremlin + '",'
            })
          namespacesAsJsonFilter = namespacesAsJsonFilter.slice(0, -1);
          namespacesAsJsonFilter += '}}'
          self.dynamicFilter.push(namespacesAsJsonFilter)
          self.setGremlinFavoritesFromConfig()
         })
        .catch(function() {});
    },

    addFilter: function(label, gremlin) {
      var options = $(".topology-gremlin-favorites");
      options.append($("<option/>").val(label).attr('gremlin', gremlin));
    },

    gremlinK8sTypes: function(types) {
        return "G.V()"
          + ".Has('Manager', Regex('k8s|istio'))" 
          + ".Has('Namespace', Ne('kube-system')).Has('Namespace', Ne('istio-system'))"
          + ".Has('Type', Regex('" + types.join("|") + "'))";
    },

    addFilterK8sTypes: function(label, types) {
        this.addFilter("k8s " + label, this.gremlinK8sTypes(types));
    },

    setGremlinFavoritesFromConfig: function() {
      var self = this;
      var options = $(".topology-gremlin-favorites");
      options.children().remove();
      if (typeof(Storage) !== "undefined" && localStorage.preferences) {
        var favorites = JSON.parse(localStorage.preferences).favorites;
        if (favorites) {
          $.each(favorites, function(i, f) {
            self.addFilter(f.name, f.expression);
          });
        }
      }

      var favorites = app.getConfigValue('topology.favorites');
      if (favorites) {
        $.each(favorites, function(key, value) {
          self.addFilter(key, value);
        });
      }

      if (self.isK8SEnabled()) {
        self.addFilterK8sTypes("all", []);
        self.addFilterK8sTypes("compute", ["cluster", "container", "namespace", "node", "pod"]);
        self.addFilterK8sTypes("deployment", ["cluster", "deployment", "job", "namespace", "node", "pod", "replicaset", "replicationcontroller", "statefulset"]);
        self.addFilterK8sTypes("compute", ["cluster", "container", "namespace", "networkpolicy", "pod"]);
        self.addFilterK8sTypes("service", ["cluster", "endpoints", "ingress",  "namespace", "node", "pod", "service"]);
        self.addFilterK8sTypes("storage", ["cluster", "persistentvolume", "persistentvolumeclaim", "storageclass"]);
      }

      for (var i = 0, len = self.dynamicFilter.length; i < len; i++) {
        filter = JSON.parse(self.dynamicFilter[i])
        $.each(filter["value"], function(key, value) {
          let label = filter["Name"] + ": " + key
          self.addFilter(label, value);
        });
      }

      var default_filter = app.getConfigValue('topology.default_filter');
      if (default_filter) {
        var value = favorites[default_filter];
        if (value) self.topologyFilter = value;
      }

      var default_highlight = app.getConfigValue('topology.default_highlight');
      if (default_highlight) {
        var value = favorites[default_highlight];
        if (value) self.topologyEmphasize = value;
      }
    },

    onFilterDatalistSelect: function(e) {
      var val = e.target.value;
      var listId = e.target.getAttribute("list");
      var opts = document.getElementById(listId).childNodes;
      for (var i = 0; i < opts.length; i++) {
        if (opts[i].value === val) {
          this.topologyFilter = opts[i].getAttribute("gremlin");
          if (e.target.id === "topology-filter") {
            this.topologyFilterQuery();
          } else if (e.target.id === "topology-highlight") {
            this.emphasizeGremlinExpr();
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
      if (this.timeType === "absolute") {
        this.topologyTimeContext = this.getTopologyAbsTime();
      } else {
        this.topologyTimeContext = this.getTopologyRelTime();
      }
      this.$store.commit('topologyTimeContext', this.topologyTimeContext);
      this.syncTopo(this.topologyTimeContext, this.topologyFilter);
    },

    getTopologyAbsTime: function() {

      var time = new Date();
      if (this.topologyDate) time = new Date(this.topologyDate);

      if (this.topologyTime) {
        var tl = this.topologyTime.split(':');
        time.setHours(tl[0]);

        if (tl.length > 1) time.setMinutes(tl[1]);

        if (tl.length > 2) time.setSeconds(tl[2]);
      }

      return time.getTime();
    },

    getTopologyRelTime: function() {
      var relTimeStr = this.topologyRelTime;
      var d = relTimeStr.match(/(\d+)\s*d/);
      var h = relTimeStr.match(/(\d+)\s*h/);
      var m = relTimeStr.match(/(\d+)\s*m/);
      var s = relTimeStr.match(/(\d+)\s*s/);
      var totalTime = 0;
      if (d) totalTime += parseInt(d[1]) * 86400;
      if (h) totalTime += parseInt(h[1]) * 3600;
      if (m) totalTime += parseInt(m[1]) * 60;
      if (s) totalTime += parseInt(s[1]);
      totalTime = totalTime * 1000;
      var currentTime = new Date().getTime();
      var absTime = new Date(currentTime - totalTime);
      return absTime.getTime();
    },

    topologyFilterQuery: function() {
      var self = this;

      if (!this.topologyFilter || this.endsWith(this.topologyFilter, ")")) {
        var filter = this.topologyFilter;
        $("#topology-gremlin-favorites").find('option').filter(function() {
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
      var self = this;
      this.topologyOptionsTimeoutID = setTimeout(function() {
        $("#topology-options").css("left", "0");
        self.isTopologyOptionsVisible = true;
      }, 300);
    },

    clearTopologyTimeout: function() {
      if (this.topologyOptionsTimeoutID) {
        clearTimeout(this.topologyOptionsTimeoutID);
      }
    },

    hideTopologyOptions: function() {
      $('input').blur();
      $("#topology-options").css("left", "-543px");
      this.isTopologyOptionsVisible = false;
    },

    emphasizeNodes: function(gremlinExpr) {
      var self = this;
      var i;

      this.$topologyQuery(gremlinExpr)
        .then(function(data) {
          data.forEach(function(sg) {
            for (i in sg.Nodes) {
              self.$store.commit('emphasize', sg.Nodes[i].ID);
            }

            var toDel = [];
            for (i in self.$store.state.emphasizedNodes) {
              var found = false;
              for (var j in sg.Nodes) {
                if (self.$store.state.emphasizedNodes[i] === sg.Nodes[j].ID) {
                  found = true;
                  break;
                }
              }
              if (!found) {
                toDel.push(self.$store.state.emphasizedNodes[i]);
              }
            }

            for (i in toDel) {
              self.$store.commit('deemphasize', toDel[i]);
            }
          });
        });
    },

    emphasizeGremlinExpr: function() {
      if (this.endsWith(this.topologyEmphasize, ")")) {
        var expr = this.topologyEmphasize;
        if (this.topologyTimeContext !== 0) {
          expr = expr.replace(/g/i, "g.at(" + this.topologyTimeContext + ")");
        }

        var newGremlinExpr = expr + ".SubGraph()";
        this.emphasizeNodes(newGremlinExpr);
      } else {
        var ids = this.$store.state.emphasizedNodes.slice();
        for (var i in ids) {
          this.$store.commit('deemphasize', ids[i]);
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

    normalizeMetric: function(metric) {
      if (metric.Start && metric.Last && (metric.Last - metric.Start) > 0) {
        bps = Math.floor(1000 * 8 * ((metric.RxBytes || 0) + (metric.TxBytes || 0)) / (metric.Last - metric.Start));
        metric["Bandwidth"] = bandwidthToString(bps);
      }

      ['Start', 'Last'].forEach(function(k) {
        if (metric[k] && typeof metric[k] === 'number') {
          metric[k] = new Date(metric[k]).toLocaleTimeString();
        }
      });
      return metric;
    },

    extractMetadata: function(metadata, exclude) {
      var mdata = JSON.parse(JSON.stringify(metadata));
      for (var field of exclude) {
        var path = field.split(".");

        var root = mdata, prev, prevKey;
        for (var i = 0; i < path.length; i++) {
          var key = path[i];
          if (i === path.length - 1) {
            delete root[key];

            // object empty
            if (Object.keys(root).length === 0) delete prev[prevKey];
            break;
          }
          prev = root;
          prevKey = key;

          root = root[key];
          if (!root) break;
        }
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
  this.Metadata = metadata || {};
  this.edges = {};
  this.group = null;

  this.edge2Parent = null;
};

Node.prototype = {

  isGroupOwner: function(type) {
    return this.group && this.group.owner === this && (!type || type === this.group.type);
  },

  isCaptureOn: function() {
    return "Capture/id" in this.Metadata;
  },

  isCaptureAllowed: function() {
    var allowedTypes = ["device", "veth", "ovsbridge", "geneve", "vlan", "bond", "ovsport",
                        "internal", "tun", "bridge", "vxlan", "gre", "gretap", "dpdkport"];
    return allowedTypes.indexOf(this.Metadata.Type) >= 0;
  },

};

var Edge = function(id, host, metadata, source, target) {
  this.id = id;
  this.host = host;
  this.source = source;
  this.target = target;
  this.Metadata = metadata || {};

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

    return node;
  },

  updateNode: function(id, metadata) {
    this.nodes[id].Metadata = metadata;

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
      store.commit('nodeUnselected');
    }

    this.notifyHandlers('nodeDeleted', node);
  },

  getNeighbor: function(node, type) {
    for (var id in node.edges) {
      var edge = node.edges[id];
      if (edge.source === node && edge.target.Metadata.Type === type) return edge.source;
      if (edge.target === node && edge.source.Metadata.Type === type) return edge.target;
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
    if (edge.Metadata.RelationType === "ownership" || edge.Metadata.Type === "vlan") {
      var group = this.groups[source.id];
      if (!group) {
        var type = edge.Metadata.RelationType === "ownership" ? "ownership" : "interface";


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

      return edge;
    }
  },

  updateEdge: function(id, metadata) {
    if (id in this.edges) {
      this.edges[id].Metadata = metadata;
    }
  },

  delEdge: function(edge) {
    if (edge.Metadata.RelationType === "ownership" || edge.Metadata.Type === "vlan") {
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
