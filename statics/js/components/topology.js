/* jshint multistr: true */

var topologyComponent

var TopologyComponent = {

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
          <object-detail :object="currentEdge.metadata"></object-detail>\
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
          <rule-detail :bridge="currentNode" :realgraph="graph"></rule-detail>\
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
        <panel id="routing-tabel" v-if="currentNodeMetadata && currentNode.metadata.RoutingTable"\
               title="Routing tables">\
          <div v-for="rt in currentNode.metadata.RoutingTable">\
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
      return this.extractMetadata(this.currentNode.metadata,
        ['LastUpdateMetric', 'Metric', 'Ovs.Metric', 'Ovs.LastUpdateMetric', 'RoutingTable', 'Features', 'K8s', 'Docker']);
    },

    currentNodeFlowsQuery: function() {
      if (this.currentNodeMetadata && this.currentNode.isCaptureAllowed())
        return "G.Flows().Has('NodeTID', '" + this.currentNode.metadata.TID + "').Sort()";
      return null;
    },

    currentNodeDocker: function() {
      if (!this.currentNodeMetadata || !this.currentNode.metadata.Docker) return null;
      return this.currentNode.metadata.Docker;
    },

    currentNodeK8s: function() {
      if (!this.currentNodeMetadata || !this.currentNode.metadata.K8s) return null;
      return this.currentNode.metadata.K8s;
    },
 
    currentNodeFeatures: function() {
      if (!this.currentNodeMetadata || !this.currentNode.metadata.Features) return null;
      return this.currentNode.metadata.Features;
    },

    currentNodeMetric: function() {
      if (!this.currentNodeMetadata || !this.currentNode.metadata.Metric) return null;
      return this.normalizeMetric(this.currentNode.metadata.Metric);
    },

    currentNodeLastUpdateMetric: function() {
      if (!this.currentNodeMetadata || !this.currentNode.metadata.LastUpdateMetric) return null;
      return this.normalizeMetric(this.currentNode.metadata.LastUpdateMetric);
    },

    currentNodeOvsMetric: function() {
      if (!this.currentNodeMetadata || !this.currentNode.metadata.Ovs || !this.currentNode.metadata.Ovs.Metric) return null;
      return this.normalizeMetric(this.currentNode.metadata.Ovs.Metric);
    },

    currentNodeOvsLastUpdateMetric: function() {
      if (!this.currentNodeMetadata || !this.currentNode.metadata.Ovs || !this.currentNode.metadata.Ovs.LastUpdateMetric) return null;
      return this.normalizeMetric(this.currentNode.metadata.Ovs.LastUpdateMetric);
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
    var allowedTypes = ["device", "veth", "ovsbridge", "geneve", "vlan", "bond", "ovsport",
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

    return node;
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
      store.commit('nodeUnselected');
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

      return edge;
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
