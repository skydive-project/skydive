/* jshint multistr: true */

var FilterSelector = {

  mixins: [apiMixin],

  props: {

    filters: {
      type: Object,
      required: true,
    },

    query: {
      type: String,
      required: true,
    },

  },

  template: '\
    <button-dropdown class="filter-selector"\
                     :auto-close="false"\
                     :b-class="{\'btn-xs\': true, \'active\': hasFilters}">\
      <span slot="button-text">\
        <i class="fa fa-filter" aria-hidden="true"></i>\
      </span>\
      <li>\
        <form class="filter-form form-inline" @submit.prevent="add">\
          <autocomplete :suggestions="keySuggestions"\
                        placeholder="Filter by flow key..."\
                        v-model="key">\
          </autocomplete>\
          <autocomplete :suggestions="valueSuggestions"\
                        placeholder="key value..."\
                        v-model="value">\
          </autocomplete>\
          <button type="submit" class="btn btn-primary btn-sm">\
            <i class="fa fa-plus" aria-hidden="true" title="Add filter"></i>\
          </button>\
        </form>\
      </li>\
      <li v-if="hasFilters" role="separator" class="divider"></li>\
      <li v-for="(values, key) in filters">\
        <a href="#">\
          <small>{{key}}</small>\
          <span class="label label-info" v-for="(value, index) in values"\
                @click.stop="remove(key, index)">{{value}}\
            <i class="fa fa-close" aria-hidden="true"></i>\
          </span>\
        </a>\
      </li>\
    </button-dropdown>\
  ',

  data: function() {
    return {
      key: "",
      value: "",
    };
  },

  computed: {

    hasFilters: function() {
      return Object.keys(this.filters).length > 0;
    },

  },

  methods: {

    keySuggestions: function() {
      return this.$topologyQuery(this.query + ".Keys()");
    },

    valueSuggestions: function() {
      if (!this.key)
        return $.Deferred().resolve([]);
      return this.$topologyQuery(this.query + ".Values('"+this.key+"').Dedup()")
        .then(function(values) {
          return values.map(function(v) { return v.toString(); });
        });
    },

    add: function() {
      if (this.key && this.value) {
        this.$emit('add', this.key, this.value);
        this.key = this.value = "";
      }
    },

    remove: function(key, index) {
      this.$emit('remove', key, index);
    },

  },

};

var HighlightMode = {

  props: {

    value: {
      type: String,
      required: true,
    },

  },

  template: '\
    <button-dropdown :text="buttonText" b-class="btn-xs">\
      <li v-for="mode in highlightModes">\
        <a href="#" @click="select(mode)">\
          <small><i class="fa fa-check text-success pull-right"\
             aria-hidden="true" v-show="value == mode.field"></i>\
          {{mode.label}}</small>\
        </a>\
      </li>\
    </button-dropdown>\
  ',

  data: function() {
    return {
      highlightModes: [
        {
          field: 'TrackingID',
          label: 'Follow L2',
        },
        {
          field: 'L3TrackingID',
          label: 'Follow L3',
        },
      ],
    };
  },

  computed: {

    buttonText: function() {
      var self = this;
      return this.highlightModes.reduce(function(acc, m) {
        if (m.field == self.value)
          return m.label;
        return acc;
      }, "");
    },

  },

  methods: {

    select: function(mode) {
      this.$emit('input', mode.field);
    },

  },

};

var IntervalButton = {

  props: {

    value: {
      type: Number,
      required: true,
    },

  },

  template: '\
    <button-dropdown :text="intervalText" b-class="btn-xs">\
      <li v-for="val in values">\
        <a href="#" @click="select(val)">\
          <small><i class="fa fa-check text-success pull-right"\
             aria-hidden="true" v-show="val == value"></i>\
          {{val/1000}}s</small>\
        </a>\
      </li>\
    </button-dropdown>\
  ',

  data: function() {
    return {
      values: [1000, 5000, 10000, 20000, 40000],
    };
  },

  computed: {

    intervalText: function() {
      return "Every " + this.value / 1000 + "s";
    },

  },

  methods: {

    select: function(value) {
      this.$emit('input', value);
    },

  }

};

var LimitButton = {

  props: {

    value: {
      type: Number,
      required: true,
    },

  },

  template: '\
    <button-dropdown :text="buttonText" b-class="btn-xs">\
      <li v-for="val in values">\
        <a href="#" @click="select(val)">\
          <small><i class="fa fa-check text-success pull-right"\
             aria-hidden="true" v-show="val == value"></i>\
             {{valueText(val)}}</small>\
        </a>\
      </li>\
    </button-dropdown>\
  ',

  data: function() {
    return {
      values: [10, 30, 50, 100, 200, 500, 0],
    };
  },

  computed: {

    buttonText: function() {
      if (this.value === 0) {
        return "No limit";
      }
      return "Limit: " + this.value;
    },

  },

  methods: {

    valueText: function(value) {
      if (value === 0)
        return "No limit";
      return value;
    },

    select: function(value) {
      this.$emit('input', value);
    },

  }

};


Vue.component('flow-table', {

  mixins: [apiMixin],

  props: {

    value: {
      type: String,
      required: true
    },

  },

  template: '\
    <dynamic-table :rows="sortedResults"\
                   :error="queryError"\
                   :sortOrder="sortOrder"\
                   :sortBy="sortBy"\
                   :fields="fields"\
                   @sort="sort"\
                   @order="order"\
                   @toggleField="toggleField">\
      <template slot="empty">No flows found</template>\
      <template slot="row" scope="flows">\
        <tr v-bind:id="\'flow-\' + flows.row.UUID" class="flow-row"\
            :class="{\'flow-detail\': hasFlowDetail(flows.row)}"\
            @click="toggleFlowDetail(flows.row)"\
            @mouseenter="highlightNodes(flows.row)"\
            @mouseleave="unhighlightNodes(flows.row)">\
          <td v-for="field in flows.visibleFields">\
            {{ fieldValue(flows.row, field.name).toLocaleString() }}\
          </td>\
        </tr>\
        <tr class="flow-detail-row"\
            v-if="hasFlowDetail(flows.row)"\
            @mouseenter="highlightNodes(flows.row)"\
            @mouseleave="unhighlightNodes(flows.row)">\
          <td :colspan="flows.visibleFields.length">\
            <object-detail :object="flows.row" :transformer="transform" :links="flowDetailLinks(flows.row)"></object-detail>\
          </td>\
        </tr>\
      </template>\
      <template slot="actions">\
        <div class="dynmic-table-actions-item">\
          <filter-selector :query="value"\
                          :filters="filters"\
                          @add="addFilter"\
                          @remove="removeFilter"></filter-selector>\
        </div>\
        <div class="dynmic-table-actions-item">\
          <limit-button v-model="limit"></limit-button>\
        </div>\
        <div class="dynmic-table-actions-item">\
          <highlight-mode v-model="highlightMode"></highlight-mode>\
        </div>\
        <div class="dynmic-table-actions-item">\
          <button-state class="btn-xs pull-right"\
                        v-model="autoRefresh"\
                        enabled-text="Auto refresh on"\
                        disabled-text="Auto refresh off"></button-state>\
        </div>\
        <div class="dynmic-table-actions-item">\
          <interval-button v-if="autoRefresh"\
                          class="pull-right"\
                          v-model="interval"></interval-button>\
        </div>\
        <div class="dynmic-table-actions-item">\
          <button class="btn btn-default btn-xs pull-right"\
                  type="button"\
                  @click="getFlows"\
                  title="Refresh flows"\
                  v-if="!autoRefresh">\
            <i class="fa fa-refresh" aria-hidden="true"></i>\
          </button>\
        </div>\
      </template>\
    </dynamic-table>\
  ',

  components: {
    'interval-button': IntervalButton,
    'highlight-mode': HighlightMode,
    'filter-selector': FilterSelector,
    'limit-button': LimitButton,
  },

  data: function() {
    return {
      queryResults: [],
      queryError: "",
      limit: 30,
      sortBy: null,
      sortOrder: -1,
      interval: 1000,
      intervalId: null,
      autoRefresh: false,
      showDetail: {},
      highlightMode: 'TrackingID',
      currRelatedNodeHighlighted: '',
      filters: {},
      fields: [
        {
          name: ['UUID'],
          label: 'UUID',
          show: false,
        },
        {
          name: ['LayersPath'],
          label: 'Layers',
          show: false,
        },
        {
          name: ['Application'],
          label: 'App.',
          show: true,
        },
        {
          name: ['Network.Protocol', 'Link.Protocol'],
          label: 'Proto.',
          show: false,
        },
        {
          name: ['Network.A', 'Link.A'],
          label: 'A',
          show: true,
        },
        {
          name: ['Network.B', 'Link.B'],
          label: 'B',
          show: true,
        },
        {
          name: ['Transport.Protocol'],
          label: 'L4 Proto.',
          show: false,
        },
        {
          name: ['Transport.A'],
          label: 'A port',
          show: false,
        },
        {
          name: ['Transport.B'],
          label: 'B port',
          show: false,
        },
        {
          name: ['Metric.ABPackets'],
          label: 'AB Pkts',
          show: true,
        },
        {
          name: ['Metric.BAPackets'],
          label: 'BA Pkts',
          show: true,
        },
        {
          name: ['Metric.ABBytes'],
          label: 'AB Bytes',
          show: true,
        },
        {
          name: ['Metric.BABytes'],
          label: 'BA Bytes',
          show: true,
        },
        {
          name: ['TrackingID'],
          label: 'L2 Tracking ID',
          show: false,
        },
        {
          name: ['L3TrackingID'],
          label: 'L3 Tracking ID',
          show: false,
        },
        {
          name: ['NodeTID'],
          label: 'Interface',
          show: false,
        },
        {
          name: ['Metric.RTT'],
          label: 'RTT ms',
          show: true,
        },
      ]
    };
  },

  created: function() {
    // sort by Application by default
    this.sortBy = this.fields[2].name;
    this.getFlows();
  },

  beforeDestroy: function() {
    this.stopAutoRefresh();
  },

  watch: {

    autoRefresh: function(newVal) {
      if (newVal === true)
        this.startAutoRefresh();
      else
        this.stopAutoRefresh();
    },

    interval: function() {
      this.stopAutoRefresh();
      this.startAutoRefresh();
    },

    value: function() {
      this.getFlows();
    },

    limitedQuery: function() {
      this.getFlows();
    },

  },

  computed: {

    time: function() {
      return this.$store.state.topologyTimeContext;
    },

    sortedResults: function() {
      return this.queryResults.sort(this.compareFlows);
    },

    // When Dedup() is used we show the detail of
    // the flow using TrackingID because the flow
    // returned has not always the same UUID
    showDetailField: function() {
      if (this.value.search('Dedup') !== -1) {
        return 'TrackingID';
      }
      return 'UUID';
    },

    timedQuery: function() {
      return this.setQueryTime(this.value);
    },

    filteredQuery: function() {
      var filteredQuery = this.timedQuery;
      for (var k of Object.keys(this.filters)) {
        if (this.filters[k].length === 1) {
          filteredQuery += ".Has('"+k+"', '"+this.filters[k][0]+"')";
        }
        else if (this.filters[k].length > 1) {
          var values = this.filters[k].join("','");
          filteredQuery += ".Has('"+k+"', within('"+values+"'))";
        }
      }
      return filteredQuery;
    },

    limitedQuery: function() {
      if (this.limit === 0 || this.filteredQuery.match(/limit/i)) {
        return this.filteredQuery;
      }
      return this.filteredQuery + '.Limit(' + this.limit + ')';
    },

  },

  methods: {

    transformer: function() {
      return this.transform.bind(this);
    },

    transform: function(key, value) {
      var dt;
      switch (key) {
        case "LastUpdateMetric.RTT":
        case "Metric.RTT":
          return value / 1000000 + " ms";
        case "Start":
        case "Last":
        case "LastUpdateMetric.Start":
        case "LastUpdateMetric.Last":
        case "TCPMetric.ABSynStart":
        case "TCPMetric.BASynStart":
        case "TCPMetric.ABFinStart":
        case "TCPMetric.BAFinStart":
        case "TCPMetric.ABRstStart":
        case "TCPMetric.BARstStart":
        case "TCPMetric.ABSawStart":
        case "TCPMetric.BASawStart":
        case "TCPMetric.AASawEnd":
        case "TCPMetric.BASawEnd":
        case "Metric.Start":
        case "Metric.Last":
          if (value) {
            dt = new Date(value);
            return dt.toLocaleString();
          }
          return "";
        case "Metric.ABPackets":
        case "Metric.BAPackets":
        case "LastUpdateMetric.ABPackets":
        case "LastUpdateMetric.BAPackets":
          return value.toLocaleString();
        case "Metric.ABBytes":
        case "Metric.BABytes":
        case "LastUpdateMetric.ABBytes":
        case "LastUpdateMetric.BABytes":
          if (value) {
            return prettyBytes(value);
          }
          return value;
      }
      return value;
    },

    startAutoRefresh: function() {
      this.intervalId = setInterval(this.getFlows.bind(this), this.interval);
    },

    stopAutoRefresh: function() {
      if (this.intervalId !== null) {
        clearInterval(this.intervalId);
        this.intervalId = null;
      }
    },

    getFlows: function() {
      var self = this;
      this.$topologyQuery(this.limitedQuery)
        .then(function(flows) {
          // much faster than replacing
          // the array with vuejs
          self.queryResults.splice(0);
          flows.forEach(function(f) {
            self.queryResults.push(f);
          });
        })
        .catch(function(r) {
          self.queryError = r.responseText + "Query was : " + self.limitedQuery;
          self.stopAutoRefresh();
        });
    },

    setQueryTime: function(query) {
      if (this.time !== 0) {
        return query.replace("G.", "G.At("+ this.time +").");
      }
      return query;
    },

    hasFlowDetail: function(flow) {
      return this.showDetail[flow[this.showDetailField]] || false;
    },

    // Keep track of which flow detail we should display
    toggleFlowDetail: function(flow) {
      if (this.showDetail[flow[this.showDetailField]]) {
        Vue.delete(this.showDetail, flow[this.showDetailField]);
      } else {
        Vue.set(this.showDetail, flow[this.showDetailField], true);
      }
    },

    flowDetailLinks: function(flow) {
      var self = this;

      var links = {};

      if (flow.RawPacketsCaptured > 0) {
        links.RawPacketsCaptured = {
          "class": "indicator glyphicon glyphicon-download-alt raw-packet-link",
          "onClick": function() {
            self.$downloadRawPackets(flow.UUID);
          },
          "onMouseOver": function(){},
          "onMouseOut": function(){}
        };
      }

      return links;
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

    unhighlightNodes: function(obj) {
      this.waitForHighlight(obj.UUID);
    },

    highlightNodes: function(obj) {
      var self = this,
          query = "G.Flows().Has('" + this.highlightMode + "', '" + obj[this.highlightMode] + "').Node()";
      this.$store.commit('highlightStart', obj.UUID);
      query = this.setQueryTime(query);
      this.$topologyQuery(query)
        .then(function(nodes) {
          nodes.forEach(function(n) {
            self.$store.commit('highlight', n.ID);
          });
          self.$store.commit('highlightEnd', obj.UUID);
        });
    },

    compareFlows: function(f1, f2) {
      if (!this.sortBy) {
        return 0;
      }
      var f1FieldValue = this.fieldValue(f1, this.sortBy),
          f2FieldValue = this.fieldValue(f2, this.sortBy);
      if (f1FieldValue < f2FieldValue)
        return -1 * this.sortOrder;
      if (f1FieldValue > f2FieldValue)
        return 1 * this.sortOrder;
      return 0;
    },

    fieldValue: function(object, paths) {
      for (var path of paths) {
        var value = object;
        for (var k of path.split(".")) {
          if (value[k] !== undefined) {
            value = value[k];
          } else {
            value = null;
            break;
          }
        }
        if (value !== null) {
          switch (path) {
            case "LastUpdateMetric.RTT":
            case "Metric.RTT":
              value /= 1000000; // ms
          }
          return value;
        }
      }
      return "";
    },

    sort: function(sortBy) {
      this.sortBy = sortBy;
    },

    order: function(sortOrder) {
      this.sortOrder = sortOrder;
    },

    addFilter: function(key, value) {
      if (!this.filters[key]) {
        Vue.set(this.filters, key, []);
      }
      this.filters[key].push(value);
    },

    removeFilter: function(key, index) {
      this.filters[key].splice(index, 1);
      if (this.filters[key].length === 0) {
        Vue.delete(this.filters, key);
      }
    },

    toggleField: function(field) {
      field.show = !field.show;
    },

  },

});

Vue.component('flow-table-control', {

  mixins: [apiMixin],

  template: '\
    <div>\
      <form @submit.prevent="validateQuery">\
        <div class="form-group has-feedback" :class="{\'has-success\': !error, \'has-error\': error}">\
          <label for="flow-table-query">Flow query</label>\
          <input id="flow-table-query" type="text" class="form-control" v-model="query" />\
          <span v-if="error" class="glyphicon glyphicon-remove form-control-feedback" aria-hidden="true"></span>\
          <span v-else class="glyphicon glyphicon-ok form-control-feedback" aria-hidden="true"></span>\
          <span v-show="error" class="help-block">{{error}}</span>\
        </div>\
      </form>\
      <flow-table :value="validatedQuery"></flow-table>\
    </div>\
  ',

  data: function() {
    return {
      query: "G.Flows().Sort()",
      validatedQuery: "G.Flows().Sort()",
      validationId: null,
      error: "",
    };
  },

  created: function() {
    this.debouncedValidation = debounce(this.validateQuery, 400);
  },

  watch: {

    query: function() {
      this.debouncedValidation();
    },

  },

  computed: {

    time: function() {
      return this.$store.state.topologyTimeContext;
    },

  },

  methods: {

    validateQuery: function() {
      var self = this;
      this.$topologyQuery(self.query)
        .then(function() {
          self.validatedQuery = self.query;
          self.error = "";
        })
        .catch(function(e) {
          self.error = e.responseText;
        });
    }

  }

});
