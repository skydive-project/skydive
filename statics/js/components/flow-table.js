/* jshint multistr: true */

var FilterSelector = {

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
      return TopologyAPI.query(this.query + ".Keys()");
    },

    valueSuggestions: function() {
      if (!this.key)
        return $.Deferred().resolve([]);
      return TopologyAPI.query(this.query + ".Values('"+this.key+"').Dedup()")
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

var TableHeader = {

  props: {

    fields: {
      type: Array,
      required: true,
    },

    sortBy: {
      type: Array,
      required: true,
    },

    sortOrder: {
      type: Number,
      required: true,
    },

  },

  template: '\
    <thead>\
      <tr>\
        <th v-for="field in fields"\
            v-if="field.show"\
            @click="sort(field.name)"\
            >\
          {{field.label}}\
          <i v-if="field.name == sortBy"\
             class="pull-right fa"\
             :class="{\'fa-chevron-down\': sortOrder == 1,\
                      \'fa-chevron-up\': sortOrder == -1}"\
             aria-hidden="true"></i>\
        </th>\
      </tr>\
    </thead>\
  ',

  methods: {

    sort: function(name) {
      if (name == this.sortBy) {
        this.$emit('order', this.sortOrder * -1);
      } else {
        this.$emit('sort', name);
      }
    },

  }

};


Vue.component('flow-table', {

  props: {

    value: {
      type: String,
      required: true
    },

  },

  template: '\
    <div v-if="!queryError" class="flow-table">\
      <div class="flow-table-wrapper">\
        <table class="table table-condensed table-bordered">\
          <table-header :fields="visibleFields"\
                        :sortOrder="sortOrder"\
                        :sortBy="sortBy"\
                        @sort="sort"\
                        @order="order"></table-header>\
          <tbody>\
            <template v-for="flow in sortedResults">\
              <tr class="flow-row"\
                  :class="{\'flow-detail\': hasFlowDetail(flow)}"\
                  @click="toggleFlowDetail(flow)"\
                  @mouseenter="highlightNodes(flow, true)"\
                  @mouseleave="highlightNodes(flow, false)">\
                <td v-for="field in visibleFields">\
                  {{fieldValue(flow, field.name)}}\
                </td>\
              </tr>\
              <tr class="flow-detail-row"\
                  v-if="hasFlowDetail(flow)"\
                  @mouseenter="highlightNodes(flow, true)"\
                  @mouseleave="highlightNodes(flow, false)">\
                <td :colspan="visibleFields.length">\
                  <object-detail :object="flow"></object-detail>\
                </td>\
              </tr>\
            </template>\
          </tbody>\
        </table>\
      </div>\
      <div class="actions">\
        <button-dropdown b-class="btn-xs" :auto-close="false">\
          <span slot="button-text">\
            <i class="fa fa-cog" aria-hidden="true"></i>\
          </span>\
          <li v-for="field in fields">\
            <a href="#" @click="field.show = !field.show">\
              <small><i class="fa fa-check text-success pull-right"\
                 aria-hidden="true" v-show="field.show"></i>\
              {{field.label}}</small>\
            </a>\
          </li>\
        </button-dropdown>\
        <filter-selector :query="value"\
                         :filters="filters"\
                         @add="addFilter"\
                         @remove="removeFilter"></filter-selector>\
        <highlight-mode v-model="highlightMode"></highlight-mode>\
        <button-state class="btn-xs pull-right"\
                      v-model="autoRefresh"\
                      enabled-text="Auto refresh on"\
                      disabled-text="Auto refresh off"></button-state>\
        <interval-button v-if="autoRefresh"\
                         class="pull-right"\
                         v-model="interval"></interval-button>\
        <button class="btn btn-default btn-xs pull-right"\
                type="button"\
                @click="getFlows"\
                title="Refresh flows"\
                v-if="!autoRefresh">\
          <i class="fa fa-refresh" aria-hidden="true"></i>\
        </button>\
      </div>\
    </div>\
    <div v-else class="alert-danger">{{queryError}}</div>\
  ',

  components: {
    'table-header': TableHeader,
    'interval-button': IntervalButton,
    'highlight-mode': HighlightMode,
    'filter-selector': FilterSelector,
  },

  data: function() {
    return {
      queryResults: [],
      queryError: "",
      sortBy: null,
      sortOrder: -1,
      interval: 1000,
      intervalId: null,
      autoRefresh: false,
      showDetail: {},
      highlightMode: 'TrackingID',
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

    filteredQuery: function() {
      this.getFlows();
    },

  },

  computed: {

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

    visibleFields: function() {
      return this.fields.filter(function(f) {
        return f.show === true;
      });
    },

    filteredQuery: function() {
      var filteredQuery = this.value;
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

  },

  methods: {

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
      TopologyAPI.query(this.filteredQuery)
        .then(function(flows) {
          // much faster than replacing
          // the array with vuejs
          self.queryResults.splice(0);
          flows.forEach(function(f) {
            self.queryResults.push(f);
          });
        })
        .fail(function(r) {
          self.queryError = r.responseText;
          self.stopAutoRefresh();
        });
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

    highlightNodes: function(obj, bool) {
      var self = this,
          query = "G.Flows().Has('" + this.highlightMode + "', '" + obj[this.highlightMode] + "').Hops()";
      TopologyAPI.query(query)
        .then(function(nodes) {
          nodes.forEach(function(n) {
            topologyLayout.SetNodeClass(n.ID, "highlighted", bool);
            if (n.Metadata.TID == obj.NodeTID) {
              topologyLayout.SetNodeClass(n.ID, "current", bool);
            }
          });
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

  },

});

Vue.component('flow-table-control', {

  template: '\
    <form @submit.prevent="validateQuery">\
      <div class="form-group has-feedback" :class="{\'has-success\': !error, \'has-error\': error}">\
        <label for="flow-table-query">Flow query</label>\
        <input id="flow-table-query" type="text" class="form-control" v-model="query" />\
        <span v-if="error" class="glyphicon glyphicon-remove form-control-feedback" aria-hidden="true"></span>\
        <span v-else class="glyphicon glyphicon-ok form-control-feedback" aria-hidden="true"></span>\
        <span v-show="error" class="help-block">{{error}}</span>\
      </div>\
      <flow-table :value="validatedQuery"></flow-table>\
    </form>\
  ',

  data: function() {
    return {
      query: "G.Flows().Sort().Limit(500)",
      validatedQuery: "G.Flows().Sort().Limit(500)",
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

  methods: {

    validateQuery: function() {
      var self = this;
      TopologyAPI.query(self.query)
        .then(function() {
          self.validatedQuery = self.query;
          self.error = "";
        })
        .fail(function(e) {
          self.error = e.responseText;
        });
    }

  }

});
