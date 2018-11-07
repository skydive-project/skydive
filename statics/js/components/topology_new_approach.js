/* jshint multistr: true */

Vue.component('v-select', VueSelect.VueSelect);

window.layoutConfig = new window.TopologyORegistry.config({
    link: {
        distance: function(e) {
            var distance = 100, coeff;

            // application
            if ((e.source.Metadata.Type === "netns") && (e.target.Metadata.Type === "netns"))
                return 1800;

            if (e.source.group !== e.target.group || !e.source.group.isEqualTo(e.target.group)) {
                if (e.source.isGroupOwner()) {
                    coeff = e.source.group.collapsed ? 40 : 60;
                    if (e.source.group.members.size) {
                        distance += coeff * e.source.group.members.size / 10;
                    }
                }
                if (e.target.isGroupOwner()) {
                    coeff = e.target.group.collapsed ? 40 : 60;
                    if (e.target.group.members.size) {
                        distance += coeff * e.target.group.members.size / 10;
                    }
                }
            }
            return distance;
        }
    },
    useHardcodedData: window.location.href.indexOf('use_hardcoded_data=1') !== -1
});

function createHostLayout(hostName, selector, linkLabelType) {
    console.log('use selector ' + selector + ' for skydive default');
    const layout = new window.TopologyORegistry.layouts.skydive_default(selector)
    layout.useConfig(layoutConfig);
    if (linkLabelType) {
      layout.useLinkLabelStrategy(linkLabelType);
    }
    const dataSource = new window.TopologyORegistry.dataSources.hostTopology(hostName)
    dataSource.subscribe();
    dataSource.e.on('broadcastMessage', (type, msg) => {
        layout.reactToDataSourceEvent.call(layout, dataSource, type, msg);
    });
    layout.addDataSource(dataSource, true);
    return layout;
}

Vue.component('full-topology', {
  props: {
    hostName: {
      type: String,
      required: true
    },
    linkLabelType: {
      type: String,
      required: true
    }
  },
  template: '\
    <div>\
      <div class="topology-d3-full-host"></div>\
    </div>\
  ',
  created: function() {
    websocket.addConnectHandler(() => {
      this.layout = createHostLayout(this.hostName, '.topology-d3-full-host', this.linkLabelType);
      this.layout.e.on('node.select', this.$parent.$parent.onNodeSelected.bind(this));
      this.layout.e.on('edge.select', this.$parent.$parent.onEdgeSelected.bind(this));
      this.layout.e.on('host.collapse', this.$parent.$parent.collapseHost.bind(this));
      this.layout.initializer();
    });
  },
  beforeDestroy: function() {
    this.$parent.$parent.$store.commit('nodeUnselected');
    this.$parent.$parent.$store.commit('edgeUnselected');
    this.layout && this.layout.remove();
  },
  methods: {
  }
});

Vue.component('app-topology', {
  props: {
    hostName: {
      type: Object,
      required: true
    }
  },
  template: '\
    <div class="topology-hierarchy">\
      <div v-if="possibleToBuildHierarchiedTopology" class="topology-d3-app-host"></div>\
      <div class="impossible-to-build-hierarchy-topology" v-if="!possibleToBuildHierarchiedTopology">\
        Impossible to build a topology on host because there\'s no virtual machines\
      </div>\
    </div>\
  ',
  created: function() {
     websocket.addConnectHandler(() => {
         this.layout = createHostLayout(this.hostName, '.topology-d3-app-host');
         this.layout.e.on('node.select', this.$parent.$parent.onNodeSelected.bind(this));
         this.layout.e.on('edge.select', this.$parent.$parent.onEdgeSelected.bind(this));
         this.layout.e.on('host.collapse', this.$parent.$parent.collapseHost.bind(this));
         this.layout.initializer();
     });
  },
  beforeDestroy: function() {
    this.$parent.$parent.$store.commit('nodeUnselected');
    this.$parent.$parent.$store.commit('edgeUnselected');
    this.layout && this.layout.remove();
  },
  computed: {
    possibleToBuildHierarchiedTopology: function() {
      return this.layout.dataManager.nodeManager.isThereAnyNodeWithType("libvirt");
    }
  }
});

Vue.component('host-topology', {
  props: {
    layout: {
      type: String,
      default: "full"
    },
    choosenHost: {
      type: String,
      required: true
    },
    linkLabelType: {
      type: String,
      required: true
    }
  },
  template: '\
    <div class="fill">\
      <div v-on:click="back" class="back-to-hosts-topology">\
        <span class="glyphicon glyphicon-backward back-to-infrastructure-topology" aria-hidden="true"></span>\
        <span>Back to hosts topology</span>\
      </div>\
      <div class="layout-switcher">\
        <ul class="nav nav-pills nav-justified">\
          <li v-on:click="switchToLayout(\'app\')" v-bind:class="[layout == \'app\' ? \'active\' : \'\']"><a href="#">App topology</a></li>\
          <li v-on:click="switchToLayout(\'full\')" v-bind:class="[layout == \'full\' ? \'active\' : \'\']"><a href="#">Full topology</a></li>\
        </ul>\
      </div>\
      <full-topology :linkLabelType="linkLabelType" :hostName="choosenHost" v-if="layout == \'full\'"></full-topology>\
      <app-topology :hostName="choosenHost" v-if="layout == \'app\'"></app-topology>\
    </div>\
  ',
  mounted: function() {
  },
  methods: {
    back: function() {
      this.$parent.backToInfrastructureTopology();
    },
    switchToLayout(layoutType) {
      this.layout = layoutType;
    }
  }
});


const HostSelector = {
  name: 'host-selector',
  props: { 
    hosts: {
      default: []
    },
    selected: {
      default: ""
    }
  },
  methods: {
    onSelectionChanged: function(item) {
      if (item) {
        this.$parent.switchToHostTopology(item.name);
      }
    }
  },
  template: '\
      <v-select :on-change="onSelectionChanged" v-model="selected" placeholder="Type to search hosts..." label="name" :options="hosts"></v-select>\
    '
};

window.Vue.component('host-selector', HostSelector);


var TopologyComponentNewApproach = {

  name: 'topology',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div class="topology">\
      <div class="row" v-if="layoutType == \'infra\'">\
        <div class="col-sm-7 fill">\
          <host-selector :selected="selectedHost" :hosts="hosts"></host-selector>\
        </div>\
      </div>\
      <div class="col-sm-7 fill content">\
        <div v-if="layoutType == \'infra\'" class="topology-d3-infra"></div>\
        <host-topology :linkLabelType="linkLabelType" :choosenHost="choosenHostName" v-if="layoutType == \'host\'"></host-topology>\
        <div class="topology-legend">\
          <strong>Topology view</strong></br>\
          <p v-if="currTopologyFilter">{{currTopologyFilter}}</p>\
          <p v-else>Full</p>\
          <p v-if="topologyHumanTimeContext">{{topologyHumanTimeContext}}</p>\
          <p v-else>Live</p>\
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
          <button v-if="layoutType === \'host\'" id="expand" type="button" class="btn btn-primary" \
                  title="Expand" @click="toggleCollapseByLevel(false)">\
            <span class="expand-icon-stack">\
              <i class="glyphicon glyphicon-resize-full icon-main"></i>\
            </span>\
            <span class="expand-icon-stack">\
              <i class="glyphicon glyphicon-plus-sign icon-sub"></i>\
            </span>\
          </button>\
          <button v-if="layoutType === \'host\'" id="collapse" type="button" class="btn btn-primary" \
                  title="Collapse" @click="toggleCollapseByLevel(true)">\
            <span class="expand-icon-stack">\
              <i class="glyphicon glyphicon-resize-small icon-main"></i>\
            </span>\
            <span class="expand-icon-stack">\
              <i class="glyphicon glyphicon-minus-sign icon-sub"></i>\
            </span>\
          </button>\
          <button v-if="layoutType === \'host\' && currentNode != null && currentNode.isGroupOwner()" id="expand-all" type="button" class="btn btn-primary" \
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
                 :class="{\'fa-expand\': currentNode.group ? currentNode.group.collapsed : false, \'fa-compress\': currentNode.group ? !currentNode.group.collapsed : false}" />\
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
        <panel id="routing-tabel" v-if="currentNodeMetadata && currentNode.Metadata.RoutingTable"\
               title="Routing tables">\
          <div v-for="rt in currentNode.Metadata.RoutingTable">\
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
      hosts: [],
      layoutType: 'infra',
      selectedHost: null,
      choosenHostName: null,
      linkLabelType: "bandwidth"
    };
  },

  mounted: function() {
    window.topologyComponent = this;
    var self = this;
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
    // @todo adapt to new ui approach
    // trigered when some component wants to highlight/emphasize some nodes
    this.$store.subscribe(function(mutation) {
      if (self.layoutType !== 'infra') {
          return;
      }
      if (mutation.type === "highlight")
        self.infraLayout.reactToTheUiEvent('node.highlight.byid', mutation.payload);
      else if (mutation.type === "unhighlight")
        self.infraLayout.reactToTheUiEvent('node.unhighlight.byid', mutation.payload);
      else if (mutation.type === "emphasize")
        self.infraLayout.reactToTheUiEvent('node.emphasize.byid', mutation.payload);
      else if (mutation.type === "deemphasize")
        self.infraLayout.reactToTheUiEvent('node.deemphasize.byid', mutation.payload);
    });

    this.setGremlinFavoritesFromConfig();

    if (typeof(this.$route.query.highlight) !== "undefined") {
      this.topologyEmphasize = this.$route.query.highlight;
      setTimeout(function(){self.emphasizeGremlinExpr();}, 2000);
    }

    if (typeof(this.$route.query.filter) !== "undefined") {
      this.topologyFilter = this.$route.query.filter;
    }

    // @todo adapt to new ui approach
    // if (typeof(this.$route.query.expand) !== "undefined") {
    //  this.layout.autoExpand(true);
    // } else {
    //  this.collapse();
    //  websocket.addConnectHandler(function() {
    //    self.collapse();
    //  });
    // }

    if (typeof(this.$route.query.link_label_type) !== "undefined") {
      this.linkLabelType = this.$route.query.link_label_type;
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

    this.bandwidth = {
      bandwidthThreshold: 'absolute',
      updatePeriod: 3000,
      active: 5,
      warning: 100,
      alert: 1000,
      intervalID: null,
    };
    this.loadBandwidthConfig();

  },

  beforeDestroy: function() {
    this.$store.commit('nodeUnselected');
    this.$store.commit('edgeUnselected');
    this.unwatch();
    this.removeInfraLayout();
  },

  created: function() {
    this.initInfraLayout();
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
        ['LastUpdateMetric', 'Metric', 'Ovs.Metric', 'Ovs.LastUpdateMetric', 'RoutingTable', 'Features', 'K8s', 'Docker']);
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
      layoutConfig.setValue('bandwidth', b);
      console.log('set bandwidth config');
    },

    initInfraLayout: function() {
        if (this.layoutType !== "infra") {
            return;
        }
        var self = this;
        const skydiveInfraLayout = new window.TopologyORegistry.layouts.infra('.topology-d3-infra')
        if (!this.infraLayout) {
            const infraTopologyDataSource = new window.TopologyORegistry.dataSources.infraTopology();
            skydiveInfraLayout.useConfig(layoutConfig);
            infraTopologyDataSource.subscribe();
            infraTopologyDataSource.e.on('broadcastMessage', (type, msg) => {
                if (type === 'SyncReply') {
                    const hostSelectorData = [];
                    msg.Obj.Nodes.forEach((node) => {
                        if (node.Metadata.Type !== "host") {
                            return;
                        }
                        hostSelectorData.push({name: node.Metadata.Name});
                    });
                    self.onUpdatedHosts(hostSelectorData);
                };
                self.infraLayout.reactToDataSourceEvent.call(self.infraLayout, infraTopologyDataSource, type, msg);
                self.infraLayout.initializer();
            });
            skydiveInfraLayout.addDataSource(infraTopologyDataSource, true)
            this.infraLayout = skydiveInfraLayout;
            this.infraLayout.e.on('node.select', this.onNodeSelected.bind(this));
            this.infraLayout.e.on('host.uncollapse', this.uncollapseHost.bind(this));
            this.infraLayout.e.on('edge.select', this.onEdgeSelected.bind(this));
        }
    },

    onUpdatedHosts: function(hosts) {
      this.selectedHost = null;
      console.log('Updated hosts', hosts);
      this.hosts = hosts;
    },

    backToInfrastructureTopology: function() {
      this.layoutType = 'infra';
      this.initInfraLayout();
    },

    onPostInit: function() {
      setTimeout(this.emphasize.bind(this), 1000);
    },

    isSSHEnabled: function() {
      return app.getConfigValue('ssh_enabled');
    },

    isK8SEnabled: function() {
      return app.getConfigValue('k8s_enabled');
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

    removeInfraLayout: function() {
      this.infraLayout.remove();
      this.infraLayout = null
    },

    switchToHostTopology: function(hostName) {
      this.removeInfraLayout();
      this.layoutType = 'host';
      this.choosenHostName = hostName;
    },

    onNodeSelected: function(d) {
      this.$store.commit('nodeSelected', d);
    },

    uncollapseHost: function(d) {
      if (this.layoutType === 'infra') {
          this.switchToHostTopology(d.Metadata.Name);
      }
    },

    collapseHost: function(d) {
      this.backToInfrastructureTopology();
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
        // @todo migrate to new ui approach
        // this.syncTopo(this.topologyTimeContext, this.topologyFilter);
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

window.detailedTopology = {"Namespace":"Graph","Type":"SyncReply","UUID":"","Status":200,"Obj":{"Nodes":[{"ID":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Metadata":{"CPU":[{"CacheSize":30720,"CoreID":"0","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":1,"CacheSize":30720,"CoreID":"0","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":2,"CacheSize":30720,"CoreID":"1","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":3,"CacheSize":30720,"CoreID":"1","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":4,"CacheSize":30720,"CoreID":"2","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":5,"CacheSize":30720,"CoreID":"2","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":6,"CacheSize":30720,"CoreID":"3","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":7,"CacheSize":30720,"CoreID":"3","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":8,"CacheSize":30720,"CoreID":"4","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":9,"CacheSize":30720,"CoreID":"4","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":10,"CacheSize":30720,"CoreID":"5","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":11,"CacheSize":30720,"CoreID":"5","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":12,"CacheSize":30720,"CoreID":"8","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":13,"CacheSize":30720,"CoreID":"8","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":14,"CacheSize":30720,"CoreID":"9","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":15,"CacheSize":30720,"CoreID":"9","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":16,"CacheSize":30720,"CoreID":"10","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":17,"CacheSize":30720,"CoreID":"10","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":18,"CacheSize":30720,"CoreID":"11","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":19,"CacheSize":30720,"CoreID":"11","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":20,"CacheSize":30720,"CoreID":"12","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":21,"CacheSize":30720,"CoreID":"12","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":22,"CacheSize":30720,"CoreID":"13","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":23,"CacheSize":30720,"CoreID":"13","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":24,"CacheSize":30720,"CoreID":"0","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":25,"CacheSize":30720,"CoreID":"0","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":26,"CacheSize":30720,"CoreID":"1","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":27,"CacheSize":30720,"CoreID":"1","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":28,"CacheSize":30720,"CoreID":"2","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":29,"CacheSize":30720,"CoreID":"2","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":30,"CacheSize":30720,"CoreID":"3","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":31,"CacheSize":30720,"CoreID":"3","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":32,"CacheSize":30720,"CoreID":"4","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":33,"CacheSize":30720,"CoreID":"4","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":34,"CacheSize":30720,"CoreID":"5","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":35,"CacheSize":30720,"CoreID":"5","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":36,"CacheSize":30720,"CoreID":"8","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":37,"CacheSize":30720,"CoreID":"8","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":38,"CacheSize":30720,"CoreID":"9","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":39,"CacheSize":30720,"CoreID":"9","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":40,"CacheSize":30720,"CoreID":"10","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":41,"CacheSize":30720,"CoreID":"10","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":42,"CacheSize":30720,"CoreID":"11","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":43,"CacheSize":30720,"CoreID":"11","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":44,"CacheSize":30720,"CoreID":"12","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":45,"CacheSize":30720,"CoreID":"12","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":46,"CacheSize":30720,"CoreID":"13","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"0","Stepping":2,"VendorID":"GenuineIntel"},{"CPU":47,"CacheSize":30720,"CoreID":"13","Cores":1,"Family":"6","Mhz":2500,"Microcode":"0x3a","Model":"63","ModelName":"Intel(R) Xeon(R) CPU E5-2650L v3 @ 1.80GHz","PhysicalID":"1","Stepping":2,"VendorID":"GenuineIntel"}],"KernelVersion":"3.10.0-693.2.2.el7.x86_64","Name":"DELL2","OS":"linux","Platform":"redhat","PlatformFamily":"rhel","PlatformVersion":"7.4","TID":"56238a31-1b4d-5b1d-42f5-c5bed9d636e2","Type":"host","VirtualizationRole":"host","VirtualizationSystem":"kvm"},"Host":"DELL2","CreatedAt":1524827146178,"UpdatedAt":1524827146178,"Revision":2},{"ID":"214ac3fc-48cf-4f41-5041-da0a35448a2e","Metadata":{"Driver":"","EncapType":"loopback","IPV4":["127.0.0.1/8"],"IPV6":["::1/128"],"IfIndex":1,"LastUpdateMetric":{"Last":1524827326349,"RxBytes":90825,"RxPackets":1730,"Start":1524827296349,"TxBytes":90825,"TxPackets":1730},"MAC":"","MTU":65536,"Metric":{"Last":1524827326349,"RxBytes":156097359,"RxPackets":2970629,"TxBytes":156097359,"TxPackets":2970629},"Name":"lo","Neighbors":[{"IP":"127.0.0.1","IfIndex":1,"MAC":"00:00:00:00:00:00","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":1}],"Prefix":"127.0.0.0/32","Protocol":2},{"Nexthops":[{"IfIndex":1}],"Prefix":"127.0.0.0/8","Protocol":2},{"Nexthops":[{"IfIndex":1}],"Prefix":"127.0.0.1/32","Protocol":2},{"Nexthops":[{"IfIndex":1}],"Prefix":"127.255.255.255/32","Protocol":2},{"Nexthops":[{"IfIndex":1}],"Prefix":"::1/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fd27:d2b3:3d12:3765:f68e:38ff:febf:e17/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fd80:2eec:eee7:ada0:128::17/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::40a9:61ff:fef6:8922/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::a236:9fff:fe99:3bf6/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::a236:9fff:fe99:3c00/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::a236:9fff:fe99:3c02/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::d410:88ff:fefe:532d/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::f68e:38ff:febf:e14/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::f68e:38ff:febf:e14/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::f68e:38ff:febf:e16/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::f68e:38ff:febf:e17/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::f68e:38ff:febf:e17/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::fc16:3eff:fe7b:9452/128"},{"Nexthops":[{"IfIndex":1}],"Prefix":"fe80::fc16:3eff:fe84:2bc5/128"}],"Src":"127.0.0.1"},{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"::/96","Protocol":3},{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"0.0.0.0/0","Protocol":3},{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"2002:a00::/24","Protocol":3},{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"2002:7f00::/24","Protocol":3},{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"2002:a9fe::/32","Protocol":3},{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"2002:ac10::/28","Protocol":3},{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"2002:c0a8::/32","Protocol":3},{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"2002:e000::/19","Protocol":3},{"Nexthops":[{"IfIndex":1,"Priority":1024}],"Prefix":"3ffe:ffff::/32","Protocol":3}]},{"Id":0,"Routes":[{"Nexthops":[{"IfIndex":1,"Priority":4294967295}],"Protocol":2},{"Nexthops":[{"IfIndex":1,"Priority":4294967295}],"Protocol":2}]}],"State":"UP","TID":"99d471fe-8750-5c59-4f41-25d1794453fa","Type":"device"},"Host":"DELL2","CreatedAt":1524827146285,"UpdatedAt":1524827326351,"Revision":7},{"ID":"71ace516-0ade-432f-4531-4b5e2a828ced","Metadata":{"Driver":"tg3","EncapType":"ether","FDB":[{"IfIndex":2,"MAC":"f4:8e:38:bf:0e:14","State":["NUD_PERMANENT"]},{"IfIndex":2,"MAC":"14:02:ec:02:bc:5e","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"78:da:6e:be:7d:98","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"64:12:25:36:f0:df","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"00:16:32:a2:f5:6a","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"64:00:6a:ea:3c:cb","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"14:02:ec:02:ac:98","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"52:54:00:14:71:35","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"64:00:6a:ea:3c:cc","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"f4:8e:38:bf:0e:14","State":["NUD_PERMANENT"],"Vlan":1},{"IfIndex":2,"MAC":"c0:67:af:87:2e:1f","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"00:16:32:a2:f5:75","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"08:00:1b:ff:03:df","State":["NUD_REACHABLE"]},{"IfIndex":2,"MAC":"52:54:00:bb:41:23","State":["NUD_REACHABLE"]},{"Flags":["NTF_SELF"],"IfIndex":2,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":2,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":2,"MAC":"33:33:ff:bf:0e:14","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":2,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]}],"IPV6":["fe80::f68e:38ff:febf:e14/64"],"IfIndex":2,"LastUpdateMetric":{"Last":1524827326349,"Multicast":19,"RxBytes":15900,"RxDropped":1,"RxPackets":238,"Start":1524827296349,"TxBytes":44478,"TxPackets":68},"MAC":"f4:8e:38:bf:0e:14","MTU":1500,"MasterIndex":10,"Metric":{"Last":1524827326349,"Multicast":65530,"RxBytes":32339546,"RxDropped":3635,"RxPackets":481719,"TxBytes":538376,"TxPackets":4895},"Name":"em1","Neighbors":[{"IP":"ff02::2","IfIndex":2,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]},{"IP":"ff02::1:ffbf:e14","IfIndex":2,"MAC":"33:33:ff:bf:0e:14","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":2,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":2,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":2,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]}],"Speed":1000,"State":"UP","TID":"313e16fb-2ae6-56f1-7c76-e1c8034888ce","Type":"device"},"Host":"DELL2","CreatedAt":1524827146291,"UpdatedAt":1524827326354,"Revision":8},{"ID":"8877cce3-61b5-4177-567f-26911e1efac1","Metadata":{"Driver":"tg3","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":3,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]}],"IfIndex":3,"MAC":"f4:8e:38:bf:0e:15","MTU":1500,"Metric":{},"Name":"em2","State":"DOWN","TID":"797aff53-fa5f-5d1e-7a6d-8aa6fa10a27b","Type":"device"},"Host":"DELL2","CreatedAt":1524827146292,"UpdatedAt":1524827146292,"Revision":2},{"ID":"8a80a519-1b94-484f-55fe-3f5a86c666be","Metadata":{"Driver":"tg3","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":4,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":4,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":4,"MAC":"33:33:ff:bf:0e:16","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":4,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]}],"IPV4":["128.0.100.17/24"],"IPV6":["fe80::f68e:38ff:febf:e16/64"],"IfIndex":4,"LastUpdateMetric":{"Last":1524827326349,"Multicast":33,"RxBytes":2250,"RxPackets":33,"Start":1524827296349},"MAC":"f4:8e:38:bf:0e:16","MTU":1500,"Metric":{"Last":1524827326349,"Multicast":116839,"RxBytes":8199917,"RxPackets":120187,"TxBytes":932,"TxPackets":11},"Name":"em3","Neighbors":[{"IP":"ff02::1:ffbf:e16","IfIndex":4,"MAC":"33:33:ff:bf:0e:16","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":4,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]},{"IP":"ff02::2","IfIndex":4,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":4}],"Prefix":"128.0.100.0/24","Protocol":2},{"Nexthops":[{"IfIndex":4,"Priority":1004}],"Prefix":"169.254.0.0/16","Protocol":3},{"Nexthops":[{"IfIndex":4,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}],"Src":"128.0.100.17"},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":4}],"Prefix":"128.0.100.0/32","Protocol":2},{"Nexthops":[{"IfIndex":4}],"Prefix":"128.0.100.17/32","Protocol":2},{"Nexthops":[{"IfIndex":4}],"Prefix":"128.0.100.255/32","Protocol":2},{"Nexthops":[{"IfIndex":4,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}],"Src":"128.0.100.17"}],"Speed":1000,"State":"UP","TID":"c9ea6988-f44c-54ff-6f99-ca6417ff53f8","Type":"device"},"Host":"DELL2","CreatedAt":1524827146294,"UpdatedAt":1524827326355,"Revision":8},{"ID":"e75f0357-f318-4775-58a1-e0453ce00ab8","Metadata":{"Driver":"tg3","EncapType":"ether","FDB":[{"IfIndex":5,"MAC":"f4:8e:38:bf:0e:14","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"14:02:ec:02:bc:5e","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"64:12:25:36:f0:df","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"00:16:32:a2:f5:6a","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"52:54:00:b1:b4:5d","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"f4:8e:38:13:d6:1b","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"1c:aa:07:d4:b0:30","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"14:02:ec:02:ac:98","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"f4:8e:38:bf:0e:17","State":["NUD_PERMANENT"],"Vlan":1},{"IfIndex":5,"MAC":"c8:f9:f9:3f:61:2f","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"c0:67:af:87:2e:1f","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"00:16:32:a2:f5:75","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"1c:aa:07:d4:b0:40","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"f4:8e:38:bf:0e:17","State":["NUD_PERMANENT"]},{"IfIndex":5,"MAC":"08:00:1b:ff:03:df","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"52:54:00:bb:41:23","State":["NUD_REACHABLE"]},{"Flags":["NTF_SELF"],"IfIndex":5,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":5,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":5,"MAC":"33:33:ff:bf:0e:17","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":5,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]}],"IPV6":["fe80::f68e:38ff:febf:e17/64"],"IfIndex":5,"LastUpdateMetric":{"Last":1524827326349,"Multicast":613,"RxBytes":23144837,"RxDropped":1,"RxPackets":17319,"Start":1524827296349,"TxBytes":1581109,"TxPackets":5786},"MAC":"f4:8e:38:bf:0e:17","MTU":1500,"MasterIndex":11,"Metric":{"Last":1524827326349,"Multicast":2348549,"RxBytes":2096944999,"RxDropped":3637,"RxPackets":4975063,"TxBytes":489210854,"TxPackets":1548069},"Name":"em4","Neighbors":[{"IP":"ff02::1:ffbf:e17","IfIndex":5,"MAC":"33:33:ff:bf:0e:17","State":["NUD_NOARP"]},{"IP":"ff02::2","IfIndex":5,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":5,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":5,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":5,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]}],"Speed":1000,"State":"UP","TID":"d7078512-7922-5a03-6974-5f0944b5f6fb","Type":"device"},"Host":"DELL2","CreatedAt":1524827146302,"UpdatedAt":1524827326350,"Revision":8},{"ID":"e75f0357-f318-4775-58a1-e0453ce00ab8AAA","Metadata":{"Driver":"tg3","EncapType":"ether","FDB":[{"IfIndex":5,"MAC":"f4:8e:38:bf:0e:14","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"14:02:ec:02:bc:5e","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"64:12:25:36:f0:df","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"00:16:32:a2:f5:6a","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"52:54:00:b1:b4:5d","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"f4:8e:38:13:d6:1b","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"1c:aa:07:d4:b0:30","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"14:02:ec:02:ac:98","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"f4:8e:38:bf:0e:17","State":["NUD_PERMANENT"],"Vlan":1},{"IfIndex":5,"MAC":"c8:f9:f9:3f:61:2f","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"c0:67:af:87:2e:1f","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"00:16:32:a2:f5:75","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"1c:aa:07:d4:b0:40","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"f4:8e:38:bf:0e:17","State":["NUD_PERMANENT"]},{"IfIndex":5,"MAC":"08:00:1b:ff:03:df","State":["NUD_REACHABLE"]},{"IfIndex":5,"MAC":"52:54:00:bb:41:23","State":["NUD_REACHABLE"]},{"Flags":["NTF_SELF"],"IfIndex":5,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":5,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":5,"MAC":"33:33:ff:bf:0e:17","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":5,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]}],"IPV6":["fe80::f68e:38ff:febf:e17/64"],"IfIndex":5,"LastUpdateMetric":{"Last":1524827326349,"Multicast":613,"RxBytes":23144837,"RxDropped":1,"RxPackets":17319,"Start":1524827296349,"TxBytes":1581109,"TxPackets":5786},"MAC":"f4:8e:38:bf:0e:17","MTU":1500,"MasterIndex":11,"Metric":{"Last":1524827326349,"Multicast":2348549,"RxBytes":2096944999,"RxDropped":3637,"RxPackets":4975063,"TxBytes":489210854,"TxPackets":1548069},"Name":"em4.1","Neighbors":[{"IP":"ff02::1:ffbf:e17","IfIndex":5,"MAC":"33:33:ff:bf:0e:17","State":["NUD_NOARP"]},{"IP":"ff02::2","IfIndex":5,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":5,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":5,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":5,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]}],"Speed":1000,"State":"UP","TID":"d7078512-7922-5a03-6974-5f0944b5f6fb","Type":"device"},"Host":"DELL2","CreatedAt":1524827146302,"UpdatedAt":1524827326350,"Revision":8},{"ID":"52f04b81-520d-4ec7-74e7-1753322e57c6","Metadata":{"Driver":"ixgbe","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":8,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":8,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":8,"MAC":"33:33:ff:99:3c:00","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":8,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]}],"IPV4":["200.200.200.17/24"],"IPV6":["fe80::a236:9fff:fe99:3c00/64"],"IfIndex":8,"MAC":"a0:36:9f:99:3c:00","MTU":1500,"Metric":{"TxBytes":620,"TxPackets":8},"Name":"p5p1","Neighbors":[{"IP":"ff02::16","IfIndex":8,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]},{"IP":"ff02::1:ff99:3c00","IfIndex":8,"MAC":"33:33:ff:99:3c:00","State":["NUD_NOARP"]},{"IP":"ff02::2","IfIndex":8,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":8,"Priority":1008}],"Prefix":"169.254.0.0/16","Protocol":3},{"Nexthops":[{"IfIndex":8}],"Prefix":"200.200.200.0/24","Protocol":2},{"Nexthops":[{"IfIndex":8,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":8}],"Prefix":"200.200.200.0/32","Protocol":2},{"Nexthops":[{"IfIndex":8}],"Prefix":"200.200.200.17/32","Protocol":2},{"Nexthops":[{"IfIndex":8}],"Prefix":"200.200.200.255/32","Protocol":2},{"Nexthops":[{"IfIndex":8,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}],"Src":"200.200.200.17"}],"Speed":10000,"State":"UP","TID":"b65e4b35-9eac-57af-4689-4fdd55927c3e","Type":"device"},"Host":"DELL2","CreatedAt":1524827146304,"UpdatedAt":1524827146304,"Revision":2},{"ID":"beb1c88d-5fc8-4ffe-46eb-805f860c3c50","Metadata":{"Driver":"ixgbe","EncapType":"ether","FDB":[{"IfIndex":9,"MAC":"a0:36:9f:99:3c:02","State":["NUD_PERMANENT"],"Vlan":1},{"IfIndex":9,"MAC":"f4:8e:38:13:d6:1b","State":["NUD_REACHABLE"]},{"IfIndex":9,"MAC":"1c:aa:07:d4:b0:30","State":["NUD_REACHABLE"]},{"IfIndex":9,"MAC":"a0:36:9f:99:3c:02","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":9,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":9,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":9,"MAC":"33:33:ff:99:3c:02","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":9,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]}],"IPV6":["fe80::a236:9fff:fe99:3c02/64"],"IfIndex":9,"LastUpdateMetric":{"Last":1524827326349,"Multicast":46,"RxBytes":2838,"RxDropped":1,"RxPackets":45,"Start":1524827296349},"MAC":"a0:36:9f:99:3c:02","MTU":1500,"MasterIndex":12,"Metric":{"Last":1524827326349,"Multicast":117339,"RxBytes":7570133,"RxDropped":3637,"RxPackets":117744,"TxBytes":2073,"TxPackets":28},"Name":"p5p2","Neighbors":[{"IP":"ff02::1:ff99:3c02","IfIndex":9,"MAC":"33:33:ff:99:3c:02","State":["NUD_NOARP"]},{"IP":"ff02::2","IfIndex":9,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":9,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":9,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]},{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":9,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]}],"Speed":10000,"State":"UP","TID":"4da1e922-3f82-5e97-73c5-a66279bfad6f","Type":"device"},"Host":"DELL2","CreatedAt":1524827146307,"UpdatedAt":1524827326352,"Revision":8},{"ID":"ec13216d-af7e-42df-5efe-a478733545de","Metadata":{"Driver":"bridge","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":10,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":10,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":10,"MAC":"33:33:ff:bf:0e:14","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":10,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]}],"IPV4":["106.120.104.17/24"],"IPV6":["fe80::f68e:38ff:febf:e14/64"],"IfIndex":10,"LastUpdateMetric":{"Last":1524827326349,"RxBytes":11650,"RxPackets":238,"Start":1524827296349,"TxBytes":51541,"TxPackets":69},"MAC":"f4:8e:38:bf:0e:14","MTU":1500,"Metric":{"Last":1524827326349,"RxBytes":22465820,"RxPackets":471844,"TxBytes":450317,"TxPackets":4854},"Name":"br-corp","Neighbors":[{"IP":"106.120.104.191","IfIndex":10,"MAC":"52:54:00:14:71:35","State":["NUD_REACHABLE"]},{"IP":"106.120.104.15","IfIndex":10,"MAC":"f4:d9:fb:7c:a0:cb","State":["NUD_STALE"]},{"IP":"106.120.104.1","IfIndex":10,"MAC":"c0:67:af:87:2e:1f","State":["NUD_STALE"]},{"IP":"106.120.104.38","IfIndex":10,"MAC":"64:00:6a:ea:3c:cc","State":["NUD_STALE"]},{"IP":"106.120.104.10","IfIndex":10,"MAC":"64:12:25:36:f0:df","State":["NUD_STALE"]},{"IP":"ff02::1:ffbf:e14","IfIndex":10,"MAC":"33:33:ff:bf:0e:14","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":10,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]},{"IP":"ff02::2","IfIndex":10,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":10,"Src":"106.120.104.1"}],"Protocol":3},{"Nexthops":[{"IfIndex":10}],"Prefix":"106.120.104.0/24","Protocol":2},{"Nexthops":[{"IfIndex":10,"Priority":1010}],"Prefix":"169.254.0.0/16","Protocol":3},{"Nexthops":[{"IfIndex":10,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":10}],"Prefix":"106.120.104.0/32","Protocol":2},{"Nexthops":[{"IfIndex":10}],"Prefix":"106.120.104.17/32","Protocol":2},{"Nexthops":[{"IfIndex":10}],"Prefix":"106.120.104.255/32","Protocol":2},{"Nexthops":[{"IfIndex":10,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}],"Src":"106.120.104.17"}],"State":"UP","TID":"b876702c-6c8a-51f8-6414-4956b14f67d9","Type":"bridge"},"Host":"DELL2","CreatedAt":1524827146311,"UpdatedAt":1524827326353,"Revision":8},{"ID":"8a4028f4-8c00-48b8-4b73-a566f70edba8","Metadata":{"Driver":"bridge","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":11,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":11,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":11,"MAC":"33:33:ff:bf:0e:17","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":11,"MAC":"33:33:ff:00:00:17","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":11,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":11,"MAC":"01:80:c2:00:00:21","State":["NUD_PERMANENT"]}],"IPV4":["128.0.0.17/24"],"IPV6":["fd80:2eec:eee7:ada0:128::17/64","fe80::f68e:38ff:febf:e17/64"],"IfIndex":11,"LastUpdateMetric":{"Last":1524827326349,"RxBytes":22229776,"RxDropped":2,"RxPackets":5565,"Start":1524827296349,"TxBytes":1518501,"TxPackets":5197},"MAC":"f4:8e:38:bf:0e:17","MTU":1500,"Metric":{"Last":1524827326349,"RxBytes":1950358017,"RxDropped":366,"RxPackets":4047535,"TxBytes":469045230,"TxPackets":1336952},"Name":"br-mgmt","Neighbors":[{"IP":"128.0.0.18","IfIndex":11,"MAC":"00:c8:8b:7d:b5:26","State":["NUD_STALE"]},{"IP":"128.0.0.191","IfIndex":11,"MAC":"52:54:00:b1:b4:5d","State":["NUD_REACHABLE"]},{"IP":"128.0.0.21","IfIndex":11,"MAC":"a0:36:9f:8c:c6:82","State":["NUD_STALE"]},{"IP":"ff02::16","IfIndex":11,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]},{"IP":"ff02::1:ffbf:e17","IfIndex":11,"MAC":"33:33:ff:bf:0e:17","State":["NUD_NOARP"]},{"IP":"ff02::1:ff00:17","IfIndex":11,"MAC":"33:33:ff:00:00:17","State":["NUD_NOARP"]},{"IP":"ff02::2","IfIndex":11,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":11}],"Prefix":"128.0.0.0/24","Protocol":2},{"Nexthops":[{"IfIndex":11,"Priority":1011}],"Prefix":"169.254.0.0/16","Protocol":3},{"Nexthops":[{"IfIndex":11,"Priority":256}],"Prefix":"fd80:2eec:eee7:ada0::/64","Protocol":2},{"Nexthops":[{"IfIndex":11,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}],"Src":"128.0.0.17"},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":11}],"Prefix":"128.0.0.0/32","Protocol":2},{"Nexthops":[{"IfIndex":11}],"Prefix":"128.0.0.17/32","Protocol":2},{"Nexthops":[{"IfIndex":11}],"Prefix":"128.0.0.255/32","Protocol":2},{"Nexthops":[{"IfIndex":11,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}],"Src":"128.0.0.17"}],"State":"UP","TID":"8fc05e1e-dd80-5b2c-6b88-e7bcf529f3a6","Type":"bridge"},"Host":"DELL2","CreatedAt":1524827146315,"UpdatedAt":1524827326351,"Revision":8},{"ID":"26a7235d-0a68-42f7-7bdb-ec29aab8e020","Metadata":{"Driver":"bridge","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":12,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":12,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":12,"MAC":"33:33:ff:99:3b:f6","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":12,"MAC":"33:33:00:00:02:02","State":["NUD_PERMANENT"]}],"IPV4":["128.0.250.17/24"],"IPV6":["fe80::a236:9fff:fe99:3bf6/64"],"IfIndex":12,"LastUpdateMetric":{"Last":1524827326349,"RxBytes":2128,"RxPackets":44,"Start":1524827296349},"MAC":"a0:36:9f:99:3c:02","MTU":1500,"Metric":{"Last":1524827326349,"RxBytes":5625241,"RxPackets":114007,"TxBytes":1080,"TxPackets":16},"Name":"br-storage","RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":12}],"Prefix":"128.0.250.0/24","Protocol":2},{"Nexthops":[{"IfIndex":12,"Priority":1012}],"Prefix":"169.254.0.0/16","Protocol":3},{"Nexthops":[{"IfIndex":12,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}],"Src":"128.0.250.17"},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":12}],"Prefix":"128.0.250.0/32","Protocol":2},{"Nexthops":[{"IfIndex":12}],"Prefix":"128.0.250.17/32","Protocol":2},{"Nexthops":[{"IfIndex":12}],"Prefix":"128.0.250.255/32","Protocol":2},{"Nexthops":[{"IfIndex":12,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}],"Src":"128.0.250.17"}],"State":"UP","TID":"d962ad6b-87c9-50b7-7c53-bd025b435f33","Type":"bridge"},"Host":"DELL2","CreatedAt":1524827146317,"UpdatedAt":1524827326355,"Revision":8},{"ID":"db3c23ae-2f82-43c6-4ee6-74c68f25da52","Metadata":{"Driver":"ixgbe","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":37,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]}],"IfIndex":37,"MAC":"a0:36:9f:99:3b:f6","MTU":1500,"Metric":{},"Name":"p7p2","State":"DOWN","TID":"14e81f49-be45-580e-68a4-3139884abb81","Type":"device"},"Host":"DELL2","CreatedAt":1524827146329,"UpdatedAt":1524827146329,"Revision":2},{"ID":"e97d7c22-7a39-428c-7ac5-a49b74dbc222","Metadata":{"Driver":"ixgbe","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":50,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]}],"IfIndex":50,"MAC":"a0:36:9f:99:3b:f4","MTU":1500,"Metric":{},"Name":"p7p1","State":"DOWN","TID":"a1a34f65-555b-50c9-4bb5-1294ec6574cc","Type":"device"},"Host":"DELL2","CreatedAt":1524827146343,"UpdatedAt":1524827146343,"Revision":2},{"ID":"71bc9e5a-6286-4f08-7edb-16b9da2169fc","Metadata":{"Driver":"bridge","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":51,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":51,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":51,"MAC":"33:33:ff:f6:89:22","State":["NUD_PERMANENT"]}],"IPV6":["fe80::40a9:61ff:fef6:8922/64"],"IfIndex":51,"LastUpdateMetric":{"Last":1524827326349,"RxBytes":2468,"RxPackets":42,"Start":1524827296349},"MAC":"f4:8e:38:bf:0e:17","MTU":1500,"Metric":{"Last":1524827326349,"RxBytes":2648154,"RxDropped":20,"RxPackets":52538,"TxBytes":648,"TxPackets":8},"Name":"brqf7543339-67","Neighbors":[{"IP":"ff02::2","IfIndex":51,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":51,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":51,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":51,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]}],"State":"UP","TID":"75560223-cf81-527e-6431-e129b20dd3d2","Type":"bridge"},"Host":"DELL2","CreatedAt":1524827146344,"UpdatedAt":1524827326355,"Revision":10},{"ID":"71bc9e5a-6286-4f08-7edb-16b9da2169fcAAA","Metadata":{"Driver":"vm","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":51,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":51,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":51,"MAC":"33:33:ff:f6:89:22","State":["NUD_PERMANENT"]}],"IPV6":["fe80::40a9:61ff:fef6:8922/64"],"IfIndex":51,"LastUpdateMetric":{"Last":1524827326349,"RxBytes":2468,"RxPackets":42,"Start":1524827296349},"MAC":"f4:8e:38:bf:0e:17","MTU":1500,"Metric":{"Last":1524827326349,"RxBytes":2648154,"RxDropped":20,"RxPackets":52538,"TxBytes":648,"TxPackets":8},"Name":"mo-vm-c5131e50","Neighbors":[{"IP":"ff02::2","IfIndex":51,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":51,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":51,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":51,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]}],"State":"UP","TID":"75560223-cf81-527e-6431-e129b20dd3d211","Type":"libvirt"},"Host":"DELL2","CreatedAt":1524827146344,"UpdatedAt":1524827326355,"Revision":10},{"ID":"e579f799-bc7a-4fcd-4e92-7536dc0ec70a","Metadata":{"Driver":"802.1Q VLAN Support","EncapType":"ether","FDB":[{"IfIndex":53,"MAC":"1c:aa:07:d4:b0:30","State":["NUD_REACHABLE"]},{"IfIndex":53,"MAC":"f4:8e:38:bf:0e:17","State":["NUD_PERMANENT"],"Vlan":1},{"IfIndex":53,"MAC":"f4:8e:38:bf:0e:17","State":["NUD_PERMANENT"]}],"IfIndex":53,"LastUpdateMetric":{"Last":1524827326349,"Multicast":17,"RxBytes":7394,"RxPackets":88,"Start":1524827296349,"TxBytes":10326,"TxPackets":109},"MAC":"f4:8e:38:bf:0e:17","MTU":1500,"MasterIndex":51,"Metric":{"Last":1524827326349,"Multicast":50457,"RxBytes":3073730,"RxPackets":54667,"TxBytes":407383,"TxPackets":4257},"Name":"br-mgmt.1312","ParentIndex":11,"State":"UP","TID":"09581e27-2da3-5d00-6f51-9374a20c825e","Type":"vlan","Vlan":1312},"Host":"DELL2","CreatedAt":1524827146346,"UpdatedAt":1524827326353,"Revision":8},{"ID":"c0e0c4d3-28e2-4eb7-7ec3-03b5b5b6cd71","Metadata":{"Driver":"bridge","EncapType":"ether","FDB":[{"Flags":["NTF_SELF"],"IfIndex":100,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":100,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":100,"MAC":"33:33:ff:fe:53:2d","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":100,"MAC":"33:33:ff:bf:0e:17","State":["NUD_PERMANENT"]}],"IPV6":["fd27:d2b3:3d12:3765:f68e:38ff:febf:e17/64","fe80::d410:88ff:fefe:532d/64"],"IfIndex":100,"MAC":"","MTU":1500,"Metric":{"RxBytes":10024,"RxPackets":156,"TxBytes":836,"TxPackets":10},"Name":"brq0bcbe5f7-5d","RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":100,"Priority":256}],"Prefix":"fd27:d2b3:3d12:3765::/64","Protocol":2},{"Nexthops":[{"IfIndex":100,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":100,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]}],"State":"UP","TID":"7266fed6-1a0d-567a-5ff7-a4efb2895989","Type":"bridge"},"Host":"DELL2","CreatedAt":1524827146348,"UpdatedAt":1524827146348,"Revision":2},{"ID":"3f52c058-a625-4836-5e70-a4d3e30b190b","Metadata":{"Driver":"802.1Q VLAN Support","EncapType":"ether","IfIndex":102,"LastUpdateMetric":{"Last":1524827326349,"Multicast":15,"RxBytes":750,"RxPackets":15,"Start":1524827296349},"MAC":"f4:8e:38:bf:0e:17","MTU":1500,"Metric":{"Last":1524827326349,"Multicast":4694,"RxBytes":274770,"RxPackets":4926,"TxBytes":37096,"TxPackets":256},"Name":"br-mgmt.1321","ParentIndex":11,"State":"UP","TID":"03111b79-a22e-5d39-5e6c-f8eb3960333e","Type":"vlan","Vlan":1321},"Host":"DELL2","CreatedAt":1524827146349,"UpdatedAt":1524827326352,"Revision":8},{"ID":"edad3077-306e-4b93-63a8-273420bc883f","Metadata":{"Driver":"tun","EncapType":"ether","FDB":[{"IfIndex":107,"MAC":"fe:16:3e:84:2b:c5","State":["NUD_PERMANENT"],"Vlan":1},{"IfIndex":107,"MAC":"fe:16:3e:84:2b:c5","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":107,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":107,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":107,"MAC":"33:33:ff:84:2b:c5","State":["NUD_PERMANENT"]}],"IPV6":["fe80::fc16:3eff:fe84:2bc5/64"],"IfIndex":107,"LastUpdateMetric":{"Last":1524827326349,"RxBytes":42,"RxPackets":1,"Start":1524827296349,"TxBytes":3112,"TxPackets":43},"MAC":"fe:16:3e:84:2b:c5","MTU":1500,"MasterIndex":51,"Metric":{"Last":1524827326349,"RxBytes":11326,"RxPackets":120,"TxBytes":13915,"TxPackets":149},"Name":"tapea7be93d-84","Neighbors":[{"IP":"ff02::2","IfIndex":107,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]},{"IP":"ff02::1:ff84:2bc5","IfIndex":107,"MAC":"33:33:ff:84:2b:c5","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":107,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":107,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]},{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":107,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]}],"Speed":10,"State":"UP","TID":"a71704c0-e0a8-5df4-480f-e17689302ed3","Type":"tun"},"Host":"DELL2","CreatedAt":1524827262659,"UpdatedAt":1524827326350,"Revision":14},{"ID":"18d2dedf-fd70-484d-75ae-ff9ef3d2ab6f","Metadata":{"Driver":"tun","EncapType":"ether","FDB":[{"IfIndex":108,"MAC":"fe:16:3e:7b:94:52","State":["NUD_PERMANENT"],"Vlan":1},{"IfIndex":108,"MAC":"fe:16:3e:7b:94:52","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":108,"MAC":"33:33:00:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":108,"MAC":"01:00:5e:00:00:01","State":["NUD_PERMANENT"]},{"Flags":["NTF_SELF"],"IfIndex":108,"MAC":"33:33:ff:7b:94:52","State":["NUD_PERMANENT"]}],"IPV6":["fe80::fc16:3eff:fe7b:9452/64"],"IfIndex":108,"LastUpdateMetric":{"Last":1524827326349,"RxBytes":10284,"RxPackets":108,"Start":1524827296349,"TxBytes":8174,"TxPackets":78},"MAC":"fe:16:3e:7b:94:52","MTU":1500,"MasterIndex":51,"Metric":{"Last":1524827326349,"RxBytes":10284,"RxPackets":108,"TxBytes":8174,"TxPackets":78},"Name":"tapbbbf73d3-6a","Neighbors":[{"IP":"ff02::1:ff7b:9452","IfIndex":108,"MAC":"33:33:ff:7b:94:52","State":["NUD_NOARP"]},{"IP":"ff02::16","IfIndex":108,"MAC":"33:33:00:00:00:16","State":["NUD_NOARP"]},{"IP":"ff02::2","IfIndex":108,"MAC":"33:33:00:00:00:02","State":["NUD_NOARP"]}],"RoutingTable":[{"Id":255,"Routes":[{"Nexthops":[{"IfIndex":108,"Priority":256}],"Prefix":"ff00::/8","Protocol":3}]},{"Id":254,"Routes":[{"Nexthops":[{"IfIndex":108,"Priority":256}],"Prefix":"fe80::/64","Protocol":2}]}],"Speed":10,"State":"UP","TID":"56de02b9-a738-596d-687e-cd618b9c1659","Type":"tun"},"Host":"DELL2","CreatedAt":1524827312160,"UpdatedAt":1524827326354,"Revision":14}],"Edges":[{"ID":"d4be5055-9159-5910-46f7-d05dcee85a6e","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"214ac3fc-48cf-4f41-5041-da0a35448a2e","Host":"DELL2","CreatedAt":1524827146285,"UpdatedAt":1524827146285},{"ID":"3ef150d7-d401-5bc9-7580-007d7c60312d","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"71ace516-0ade-432f-4531-4b5e2a828ced","Host":"DELL2","CreatedAt":1524827146291,"UpdatedAt":1524827146291},{"ID":"121de7ed-dd56-5ed3-6ff1-b268d626949c","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"8877cce3-61b5-4177-567f-26911e1efac1","Host":"DELL2","CreatedAt":1524827146292,"UpdatedAt":1524827146292},{"ID":"fbcb966a-5a7c-5fa5-5679-99a079bdf791","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"8a80a519-1b94-484f-55fe-3f5a86c666be","Host":"DELL2","CreatedAt":1524827146295,"UpdatedAt":1524827146295},{"ID":"da9756b6-0594-5f6c-7b45-65e0380be06f","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"e75f0357-f318-4775-58a1-e0453ce00ab8","Host":"DELL2","CreatedAt":1524827146302,"UpdatedAt":1524827146302},{"ID":"0720535b-2bd5-531b-74a1-761e04ed36b0","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"52f04b81-520d-4ec7-74e7-1753322e57c6","Host":"DELL2","CreatedAt":1524827146304,"UpdatedAt":1524827146304},{"ID":"5767cacf-7728-5204-71ca-ea29915ea445","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"beb1c88d-5fc8-4ffe-46eb-805f860c3c50","Host":"DELL2","CreatedAt":1524827146308,"UpdatedAt":1524827146308},{"ID":"08883be7-f8be-583d-4bf8-fcd3e611a05f","Metadata":{"RelationType":"layer2"},"Parent":"ec13216d-af7e-42df-5efe-a478733545de","Child":"71ace516-0ade-432f-4531-4b5e2a828ced","Host":"DELL2","CreatedAt":1524827146311,"UpdatedAt":1524827146311},{"ID":"708b4f9b-f03d-5bfb-47ed-36f32ed82a24","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"ec13216d-af7e-42df-5efe-a478733545de","Host":"DELL2","CreatedAt":1524827146311,"UpdatedAt":1524827146311},{"ID":"8b917c03-0e23-5747-5137-0029366258a6","Metadata":{"RelationType":"layer2"},"Parent":"e75f0357-f318-4775-58a1-e0453ce00ab8","Child":"e75f0357-f318-4775-58a1-e0453ce00ab8AAA","Host":"DELL2","CreatedAt":1524827146315,"UpdatedAt":1524827146315},{"ID":"80f1386b-5654-5a5b-6739-905f01f6678e","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"8a4028f4-8c00-48b8-4b73-a566f70edba8","Host":"DELL2","CreatedAt":1524827146315,"UpdatedAt":1524827146315},{"ID":"a819a663-306c-545c-6a98-4ebb616e7900","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"26a7235d-0a68-42f7-7bdb-ec29aab8e020","Host":"DELL2","CreatedAt":1524827146317,"UpdatedAt":1524827146317},{"ID":"8112f417-9171-58cd-5567-5c1db79f53e0","Metadata":{"RelationType":"layer2"},"Parent":"26a7235d-0a68-42f7-7bdb-ec29aab8e020","Child":"beb1c88d-5fc8-4ffe-46eb-805f860c3c50","Host":"DELL2","CreatedAt":1524827146317,"UpdatedAt":1524827146317},{"ID":"4caf02c8-eb52-5782-6602-20a00720d9b3","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"db3c23ae-2f82-43c6-4ee6-74c68f25da52","Host":"DELL2","CreatedAt":1524827146329,"UpdatedAt":1524827146329},{"ID":"a4f86a34-931f-58df-59d4-09e08e16665d","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"e97d7c22-7a39-428c-7ac5-a49b74dbc222","Host":"DELL2","CreatedAt":1524827146343,"UpdatedAt":1524827146343},{"ID":"c2802c5d-3440-5f41-7cef-e6124fcd9a7d","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"71bc9e5a-6286-4f08-7edb-16b9da2169fc","Host":"DELL2","CreatedAt":1524827146345,"UpdatedAt":1524827146345},{"ID":"c2802c5d-3440-5f41-7cef-e6124fcd9a7dAAA","Metadata":{"RelationType":"ownership"},"Parent":"18d2dedf-fd70-484d-75ae-ff9ef3d2ab6f","Child":"71bc9e5a-6286-4f08-7edb-16b9da2169fcAAA","Host":"DELL2","CreatedAt":1524827146345,"UpdatedAt":1524827146345},{"ID":"4ec6643c-d880-5eb3-674d-47d391d47eda","Metadata":{"RelationType":"layer2"},"Parent":"71bc9e5a-6286-4f08-7edb-16b9da2169fc","Child":"e75f0357-f318-4775-58a1-e0453ce00ab8AAA","Host":"DELL2","CreatedAt":1524827146346,"UpdatedAt":1524827146346},{"ID":"2dbc3b92-6c5f-5c0d-6f7e-8a684c42f22d","Metadata":{"RelationType":"layer2","Type":"vlan"},"Parent":"8a4028f4-8c00-48b8-4b73-a566f70edba8","Child":"e579f799-bc7a-4fcd-4e92-7536dc0ec70a","Host":"DELL2","CreatedAt":1524827146346,"UpdatedAt":1524827146346},{"ID":"4d038e9a-fd6f-51f0-61af-e176252cae9c","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"e579f799-bc7a-4fcd-4e92-7536dc0ec70a","Host":"DELL2","CreatedAt":1524827146346,"UpdatedAt":1524827146346},{"ID":"99b61882-a994-52f7-6430-0e41ae38461a","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"c0e0c4d3-28e2-4eb7-7ec3-03b5b5b6cd71","Host":"DELL2","CreatedAt":1524827146348,"UpdatedAt":1524827146348},{"ID":"5a84960a-6a32-5c9c-60fc-5f40c2ae29ab","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"3f52c058-a625-4836-5e70-a4d3e30b190b","Host":"DELL2","CreatedAt":1524827146349,"UpdatedAt":1524827146349},{"ID":"ebc0c799-8ab1-5d1e-5d58-7c32807407a0","Metadata":{"RelationType":"layer2","Type":"vlan"},"Parent":"8a4028f4-8c00-48b8-4b73-a566f70edba8","Child":"3f52c058-a625-4836-5e70-a4d3e30b190b","Host":"DELL2","CreatedAt":1524827146349,"UpdatedAt":1524827146349},{"ID":"f6564e16-d81e-5d73-6acb-e399c74be753","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"edad3077-306e-4b93-63a8-273420bc883f","Host":"DELL2","CreatedAt":1524827262659,"UpdatedAt":1524827262659},{"ID":"c62813ca-addc-5c76-51e7-4bdd3b3f8a5b","Metadata":{"RelationType":"layer2"},"Parent":"71bc9e5a-6286-4f08-7edb-16b9da2169fc","Child":"18d2dedf-fd70-484d-75ae-ff9ef3d2ab6f","Host":"DELL2","CreatedAt":1524827262666,"UpdatedAt":1524827262666},{"ID":"4cf7d469-89f3-58eb-6f35-41fc55079a3e","Metadata":{"RelationType":"ownership"},"Parent":"f3c66e49-a91a-426a-78cc-1fb28ee699bc","Child":"18d2dedf-fd70-484d-75ae-ff9ef3d2ab6f","Host":"DELL2","CreatedAt":1524827312160,"UpdatedAt":1524827312160},{"ID":"4cf7d469-89f3-58eb-6f35-41fc55079a3eAAA","Metadata":{"RelationType":"ownership"},"Parent":"71bc9e5a-6286-4f08-7edb-16b9da2169fc","Child":"18d2dedf-fd70-484d-75ae-ff9ef3d2ab6f","Host":"DELL2","CreatedAt":1524827312160,"UpdatedAt":1524827312160},{"ID":"f2dc0acb-f324-50c6-50f7-656f73578279","Metadata":{"RelationType":"layer2"},"Parent":"71bc9e5a-6286-4f08-7edb-16b9da2169fc","Child":"18d2dedf-fd70-484d-75ae-ff9ef3d2ab6f","Host":"DELL2","CreatedAt":1524827312166,"UpdatedAt":1524827312166}]}};
