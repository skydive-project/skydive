/*
 * Copyright (C) 2017 Orange.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

/** Important actions for summary */
var ActionOutput = 0;
var ActionResubmit = 1;
var ActionFlood = 2;
var ActionNormal = 3;
var ActionDrop = 4;

/** Port number representing any port */
var ANY_PORT = -1;
/** Port number representing the local port */
var LOCAL_PORT = -2;
/** Port number representing the controller port */
var CONTROLLER_PORT = -3;
/** Port number representing all the ports but the entry port */
var OTHER_PORTS = -4;
/** Port number representing the entry port - not an Openflow normalized port */
var SAME_PORT = -5;

/** Translation from OVS syntax to summary actions */
var actionTable = {
  'output': ActionOutput,
  'enqueue': ActionOutput,
  'resubmit': ActionResubmit,
  'drop': ActionDrop,
  'local': ActionOutput,
  'flood': ActionFlood,
  'normal': ActionNormal,
  'in_port': ActionOutput
};
/** Small function to parse port numbers.
 *
 * Default on ANY port
 * @param s: string to parse
 * @return port number
 */
function safePort(s) {
  var v = parseInt(s);
  return isNaN(v) ? ANY_PORT : v;
}
/** Computes the action summary from a rule associated to an element of the actions
 *
 * The rule is modified and its outAction is filled.
 * @param rule a rule without outAction
 * @param element the element of the
 */
function computeAction(rule, element) {
  var verb = element['Function'].toLowerCase();
  var args = element['Arguments'];
  function getNumArg(i) {
    return safePort(args[i]?args[i]['Function']:'');
  }
  var action = actionTable[verb];
  if (action !== undefined) {
    var summary = { action: action };
    switch (verb) {
      case 'resubmit':
        summary.port = getNumArg(0);
        if (summary.port === ANY_PORT)
          summary.port = SAME_PORT;
        if (args.length > 1)
          summary.table = getNumArg(1);
        break;
      case 'output':
      case 'enqueue':
        summary.port = getNumArg(0);
        break;
      case 'local':
        summary.port = LOCAL_PORT;
        break;
      case 'in_port':
        summary.port = rule.inPort;
        break;
    }
    rule.outAction.push(summary);
  }
}
/** Summarize the actions of a rule, filling the outAction field
 *  @param rule: the rule to summarize.
 */
function summarizeActions(rule) {
  rule.outAction = [];
  var actions = rule.Actions;
  for (var i = 0; i < actions.length; i++) {
    computeAction(rule, actions[i]);
  }
}

/** Compute the inport from a filter expression
 * @param filter: the filter as a single string
 * @return the port or ANY_PORT if not found.
*/
function inport(filters) {
  var matchInport = ANY_PORT;
  if (filters) {
    for (var i=0; i <  filters.length; i++) {
      var filter = filters[i];
      if (filter['Key'] == 'in_port') {
        matchInport = safePort(filter['Value']);
        break;
      }
    }
  }
  return matchInport;
}

/** Summarize the filter of a rule, filling the inPort
 *  @param rule: the rule to complete
 */
function summarizeFilter(rule) {
  rule.inPort = inport(rule.Filters);
}

/** Text from a list of Openflow filters
 * @param filters: list of Openflow filter in JSON syntax
 * @return a single string, empty if no filter.
 */
function textFilters(filters) {
  function text(e) {
    var r = e['Value'] != '' ? ':' + e['Value'] + (e['Mask'] != undefined ? '/' + e['Mask']: '')  : '';
    return e['Key'] + r
  }
  return !filters ? '' : filters.map(text).join(',');
}

/** Pretty print an action
 *
 * @param a: the json action to pretty print
 * @return a string in the spirit of OVS syntax but with less quirks.
 */
function textAction(a) {
  if (!a) return ''
  var args = a['Arguments'];
  var f = a['Function'];
  var r;
  switch(f) {
    case '=':
      r = textAction(args[0]) + '=' + textAction(args[1]);
      break;
    case 'range':
      r = (
        textAction(args[0]) + '[' +
        (args.length > 1
          ? textAction(args[1]) +
            (args.length == 3 ? '..' +  textAction(args[2]) + ']' : ']')
          : ']'));
      break;
    default:
      if (args != undefined) {
        r = f + '(' + args.map(textAction).join(',') + ')';
      } else {
        r = f;
      }
  }
  return a['Key'] ? a['Key'] + '=' + r : r;
}

/** Computes the summary of a rule, both filters and outActions
 * @param rule: the rule to complete
 */
function summarize(rule) {
  summarizeFilter(rule);
  summarizeActions(rule);
  rule['textFilters'] = textFilters(rule['Filters']);
  rule['textActions'] = rule['Actions'].map(textAction).join(',');
}

/** Computes the summary of a group node.
 *
 * The main action is to parse the buckets and isolate additional information
 * as the selection algorithm (assumed before the first bucket)
 */
function summarizeGroup(group) {
  function textMeta(m){
    return m['Key'] + (m['Value'] ? '=' + m['Value'] : '');
  }
  function textBucket(bucket) {
    var content;
    var actions = bucket.Actions.map(textAction).join(',');
    if (bucket.Meta) {
      content = bucket.meta.map(textMeta).join(',') + ',actions='  + actions;
    } else {
      content = 'actions=' + actions;
    }
    return {id: bucket.Id, content: content};
  }
  group.metaText = group.Meta ? group.Meta.map(textMeta).join(',') : '';
  group.bucketsText = group.Buckets.map(textBucket);
}

/** Compare two openflow rules by priority and then action.
 *  @param rule1: first rule
 *  @param rule2: second rule
 *  @return an integer as specified by array.sort
 */
function compareRules(rule1, rule2) {
  if (rule1.Priority > rule2.Priority) return -1;
  if (rule1.Priority == rule2.Priority && rule1.Actions < rule2.Actions) return -1;
  if (rule1.Priority == rule2.Priority && rule1.Actions == rule2.Actions && rule1.Filters < rule2.Filters) return -1;
  return 1;
}

/** Adds rowspan to have a nicely formatted priorities and actions
 * @param rules: a set of Openflow rules
 */
function addRowspan(rules) {
  rules.sort(compareRules);
  var prevActions;
  var prevPriority;
  for(var i=0; i<rules.length; i++) {
    var rule = rules[i];
    if(rule.Priority == prevPriority) {
      rule.prioritySpan = -1;
    } else {
      prevPriority=rule.Priority;
      var span=0;
      for(var j=i; j<rules.length && rules[j].Priority == prevPriority; j++) {
        span = span+1;
      }
      rule.prioritySpan = span;
    }
    if(rule.Priority == prevPriority && rule.Actions == prevActions) {
      rule.actionsSpan = -1;
    } else {
      prevActions=rule.Actions;
      var span=0;
      for(var j=i; j<rules.length && rules[j].Actions == prevActions; j++) {
        span = span+1;
      }
      rule.actionsSpan = span;
    }
  }
}

/** Classify the eleements of an array into a table according to a classifier function.
 *
 * A generic function that taken a classifier that gives back
 * the kind of an object as an integers transform an array of objects
 * in a table indexed by integers of list where each list is the set of elements of the
 * array that have the key as classifier.
 * @param array: the array to classify
 * @param classifier: the classifier function
 * @return the result as an integer indexed table of list
 */
function classify(array, classifier) {
  var result = {};
  for (var i = 0; i < array.length; i++) {
    var elem = array[i];
    var key = classifier(elem);
    var list = result[key];
    if (list !== undefined)
      list.push(elem);
    else
      result[key] = [elem];
  }
  for(var key in result) {
    addRowspan(result[key]);
  }
  return result;
}

/** Computes the chain of node that represent the path of node to highlight for a given bridge interface
 *
 * If it is an external port, we only go up to the port itself, if it is a patch port, we continue on the other bridge
 * but the bridge node is not part of the highlighted path. itfs is filled with an entry for the port using the numbering of ovs-ofctl as key
 * and the list of nodes as values.
 * @param c a GNode of type ovsport
 * @param graph the full graph model
 * @param itfs the table to fill with information on the port represented.
*/
function extractPort(c, graph, itfs) {
  var pot = graph.getTargets(c);
  for (var i = 0; i < pot.length; i++) {
    var cc = pot[i];
    var ofport = cc.metadata.OfPort;
    if (ofport === undefined)
      continue;
    var itf = [c, cc];
    itfs[ofport] = itf;
    if (cc.metadata.Type === 'patch') {
      var ccc = graph.getNeighbor(cc, 'patch');
      if (ccc === undefined)
        return;
      itf.push(ccc);
      var cccc = graph.getNeighbor(ccc, 'ovsport');
      if (cccc !== undefined)
        itf.push(cccc);
    }
  }
}

/** A representation of the rules associated to a bridge and the associated functions.
 *  The methods of the rule-detail and rule-table-detail components are in fact implemented
 *  by this object
*/
var BridgeLayout = (function () {

  /** Builds the bridge layout
   * @param graph the global graph hosting the bridge
   * @param bridge the bridge node whose rules are represented.
   */
  function BridgeLayout(graph, filtered, bridge, store) {
    this.graph = graph;
    this.filtered = filtered;
    this.bridge = bridge;
    this.store = store;
    this.compute();
  }

  /** Extract the information on the interfaces and rules from the nodes neighbor of the bridge node. */
  BridgeLayout.prototype.extract = function () {
    var itfs = {};
    var rules = [];
    var rulesUUID = new Set();       // added on check tests, by p.c.
    var groupsUUID = new Set();
    var groups = [];
    var children = this.graph.getTargets(this.bridge);
    for (var i = 0; i < children.length; i++) {
      var c = children[i];
      if (c === undefined)
        continue;
      switch (c.metadata.Type) {
        case 'ovsport':
          extractPort(c, this.graph, itfs);
          break;
        case 'ofrule':
          var rule = c.metadata;
          if (this.filtered != null && ! (this.filtered.includes(rule.UUID))) continue;
          summarize(rule);
          rules.push(rule);
          rulesUUID.add(rule.UUID);
          break;
        case 'ofgroup':
          var group = c.metadata;
          summarizeGroup(group);
          groups.push(group);
          groupsUUID.add(group.UUID);

      }
    }
    this.rules = rules;
    this.groups = groups;
    this.interfaces = itfs;
    this.rulesUUID = rulesUUID;
    this.groupsUUID = groupsUUID;
  };

  /** Structure the information on the rules, classifying by tables and ports. */
  BridgeLayout.prototype.structure = function () {
    var perTableRules =
      classify(this.rules, function (r) { return r.Table; });
    this.structured = {};
    for (var key in perTableRules) {
      var array = perTableRules[key];
      var portRules = classify(array, function (r) { return r.inPort; });
      var anyRules = portRules[ANY_PORT];
      delete portRules[ANY_PORT];
      this.structured[key] = { any: anyRules, ports: portRules };
    }
  };

  /** Computes the layout information for a bridge. */
  BridgeLayout.prototype.compute = function () {
    this.extract();
    this.structure();
  };

  /** Change the selected table in the UI.
   * @param tab the table number (0 is the default Openflow table)
   */
  BridgeLayout.prototype.switchTab = function (tab) {
    $('.nav-pills a[href="#T' + tab + '"]').tab('show');
  };

  /** switchPortTab Changes the selected port in the UI.
   * @param tabR the table
   * @param tabP the port number
   */
  BridgeLayout.prototype.switchPortTab = function (tabR, tabP) {
    $('.nav-pills a[href="#P' + tabR + '-' + tabP + '"]').tab('show');
  };

  /** hightlights the node associated to a port
   * @param p: Openflow index of the port to hightlight.
   */
  BridgeLayout.prototype.mark = function(p) {
    var nodes = this.interfaces[p];
    if (nodes === undefined) return;
    for (var i = 0; i < nodes.length; i++) {
      this.store.commit('highlight', nodes[i].id);
    }
  };

  /** unhightlights the node associated to a port
   * @param p: index of the port to unhighlight
   */
  BridgeLayout.prototype.unmark = function(p) {
    var nodes = this.interfaces[p];
    if (nodes === undefined) return;
    for (var i = 0; i < nodes.length; i++) {
      this.store.commit('unhighlight', nodes[i].id);
    }
  };

  /** Select the node at the output of a rule.
     * @param p: the index of the port to follow to find the new selected node.
     */
  BridgeLayout.prototype.select = function(p) {
    var nodes = this.interfaces[p];
    this.unmark(p);
    var len = nodes.length;
    var last = nodes[len - 1];
    if (len === 4) {
      last = this.graph.getNeighbor(last, 'ovsbridge');
    }
    this.store.commit('nodeSelected', last);
    this.switchTab(0);
  };

  BridgeLayout.prototype.clazz = function(act) {
    var clazz;
    switch (act) {
      case ActionOutput:
        clazz = 'share fa-long-arrow-right';
        break;
      case ActionResubmit:
        clazz = 'fa-level-down';
        break;
      case ActionFlood:
        clazz = 'blue fa-volume-off';
        break;
      case ActionNormal:
        clazz = 'blue fa-volume-off';
        break;
      case ActionDrop:
        clazz = 'red fa-ban';
        break;
      default:
        return null;
    }
    return "fa " + clazz;
  };

  /** Readable name of an openflow port
   * @param pr the index of the port or its stringified value.
  */
  BridgeLayout.prototype.portname = function(pr) {
    var p = (typeof (pr) === 'string') ? parseInt(pr) : pr;
    if (p === ANY_PORT) return 'ANY';
    if (p === LOCAL_PORT) return 'LOCAL';
    if (p === SAME_PORT || p === undefined) return '';
    var nodes = this.interfaces[p];
    if (nodes === undefined) return '???';
    var port = nodes[1];
    var portname = (port === undefined) ? '???' : port.metadata.Name;
    return portname;
  };

  /** Check if rule is highlighted. */
  BridgeLayout.prototype.isHighlighted = function(rule) {
    var current = this.store.state.currentRule;
    var status = current && rule.UUID === current.metadata.UUID;
    return status;
  };

  return BridgeLayout;
}());

/** Graphical component that represents a set of rules associated to a given
 *  Openflow table and port and displayed as a single HTML table.
 */
Vue.component('rule-table-detail', {
  template: '\
    <div class="dynamic-table">\
      <table class="table table-bordered table-condensed">\
        <thead>\
          <tr>\
            <th class="priority-column">priority</th>\
            <th class="filters-column">filters</th>\
            <th class="summary-column">summary</th>\
            <th class="actions-column">actions</th>\
          </tr>\
        </thead>\
        <tbody>\
            <tr v-for="rule in rules"\
                :id="\'R-\' + rule.UUID"\
                v-bind:class="{soft: layout.isHighlighted(rule)}">\
                <td v-if="rule.prioritySpan != -1" :rowspan="rule.prioritySpan">\
                  {{rule.Priority}}\
                </td>\
                <td>\
                  {{ splitLine(rule.textFilters) }}\
                </td>\
                <td v-if="rule.actionsSpan != -1" :rowspan="rule.actionsSpan">\
                    <table class="inner-table">\
                        <tr v-for="act in rule.outAction">\
                            <td>\
                              <i :class="layout.clazz(act.action)"></i>\
                            </td>\
                            <td v-on:mouseover="layout.mark(act.port)"\
                                v-on:mouseleave="layout.unmark(act.port)"\
                                v-on:click="layout.select(act.port)">\
                                <span class="port-link">{{layout.portname(act.port)}}</span>\
                            </td>\
                            <td><a class="table-link" v-on:click="layout.switchTab(act.table)">{{act.table}}</a></td>\
                        </tr>\
                    </table>\
                </td>\
                <td v-if="rule.actionsSpan != -1" :rowspan="rule.actionsSpan">\
                  {{ splitLine(rule.textActions) }}\
                </td>\
            </tr>\
        </tbody>\
      </table>\
    </div>',
  props: {
    rules: {
      type: Object,
      required: true
    },
    layout: {
      type: Object,
      required: true
    }
  },
  methods: {
    splitLine: function(elt_list) {
      function reresplit(elt) {
        return elt.length > 40 ? elt.match(/.{1,40}/g).join('\u200b') : elt;
      }
      function resplit(elt) {
        return (
          elt.length > 40 ?
          elt.split(';').map(reresplit).join(';\u200b') : elt);
      }
      return elt_list.split(',').map(resplit).join(',\u200b');
    }
  }
});

/** Graphical component for group table */
Vue.component('groups-detail', {
  template: '\
    <div class="group-detail" v-if="Object.keys(layout.groups).length > 0">\
        <ul class="nav nav-pills" role="tablist">\
          <li>\
            <span style="display: block;padding: 10px 10px;font-weight: bold;">Group</span>\
          </li>\
          <li :class="{ active: index==0 }"\
              v-for="(group, index) in layout.groups">\
              <a data-toggle="tab"\
                  role="tab"\
                  :href="\'#G\' + group.GroupId">{{group.GroupId}}</a>\
          </li>\
        </ul>\
      <div class="groups">\
        <div class="tab-content clearfix">\
            <div :class="{ active: index==0 }"\
                class="tab-pane"\
                :id="\'G\' + group.GroupId"\
                role="tabpanel"\
                v-for="(group, index) in layout.groups">\
                <div class="object-detail">\
                  <div class="object-key-value">\
                    <span class="object-key">Type</span>: \
                    <span class="object-detail">{{group.GroupType}}</span>\
                  </div>\
                  <div class="object-key-value">\
                    <span class="object-key">Additional</span>: \
                    <span class="object-detail">{{group.metaText}}</span>\
                  </div>\
                </div>\
                <div class="dynamic-table">\
                  <table class="table table-bordered table-condensed">\
                      <thead>\
                          <tr>\
                              <th class="id">id</th>\
                              <th class="content">content</th>\
                          </tr>\
                      </thead>\
                      <tbody>\
                          <tr v-for="bucket in group.bucketsText"\
                              :id="\'GB-\' + group.UUID + \'-\' + bucket.id">\
                              <td>{{bucket.id}}</td>\
                              <td>{{bucket.content}}</td>\
                          </tr>\
                      </tbody>\
                  </table>\
                </div>\
            </div>\
          </div>\
        </div>\
      </div>\
    </div>\
  ',
  props: {
    layout: {
      type: Object,
      required: true
    }
  },
});


/** Vue component showing the rules associated to a bridge */
Vue.component('rule-detail', {

  mixins: [apiMixin],

  template: '\
    <div class="rules-detail flow-ops-panel">\
      <ul class="nav nav-pills"\
        role="tablist">\
        <li>\
          <span style="display: block;padding: 10px 10px;font-weight: bold;">Table</span>\
        </li>\
        <li :class="{ active: (tidx==0) }"\
          v-for="(table, tname, tidx) in layout.structured">\
          <a data-toggle="tab"\
            role="tab"\
            :href="\'#T\' + tname">{{tname}}</a>\
        </li>\
      </ul>\
      <div class="rules">\
        <div class="tab-content clearfix">\
          <div :class="{ active: (tidx==0) }"\
            class="tab-pane"\
            :id="\'T\' + tname"\
            role="tabpanel"\
            v-for="(table, tname, tidx) in layout.structured">\
            <div class="container-fluid" v-if="Object.keys(table.ports).length > 0">\
              <div class="navbar-header">\
                <span class="navbar-brand"> Port </span>\
              </div>\
              <ul class="nav nav-pills"\
                role="tablist">\
                <li :class="{ active: (pidx==0) }"\
                  v-for="(rules,port,pidx) in table.ports"\
                  v-on:mouseover="layout.mark(port)"\
                  v-on:mouseleave="layout.unmark(port)">\
                  <a data-toggle="tab"\
                    role="tab"\
                    :href="\'#P\' + tname + \'-\' + port">{{layout.portname(port)}}</a>\
                </li>\
              </ul>\
            </div>\
            <div class="tab-content">\
              <div :class="{ active: (pidx==0) }"\
                class="tab-pane"\
                :id="\'P\' + tname + \'-\' + port"\
                role="tabpanel"\
                v-for="(rules, port, pidx) in table.ports">\
                <rule-table-detail :rules="rules" :layout="layout"/>\
              </div>\
            </div>\
            <rule-table-detail :rules="table.any" :layout="layout"/>\
          </div>\
          <rule-table-detail v-if="Object.keys(layout.structured).length == 0" />\
          <div class="dynamic-table">\
            <div class="dynamic-table-actions">\
              <filter-selector :query="value"\
                :filters="filters"\
                @add="addFilter"\
                @remove="removeFilter"></filter-selector>\
            </div>\
          </div>\
        </div>\
      </div>\
      <div>\
        <groups-detail :layout="layout"/>\
      </div>\
    </div>\
  ',

  components: {
    'filter-selector': FilterSelector
  },

  props: {
    bridge: {
      type: Object,
      required: true
    },
    graph: {
      type: Object,
      required: true
    }
  },

  data: function() {
    return {
      value: "",
      memoBridgeLayout:null,
      filters: {},
      filtered: null
    };
  },

  computed: {
    layout: function () {
      if (! this.memoBridgeLayout || this.memoBridgeLayout.bridge !== this.bridge) {
        this.memoBridgeLayout = new BridgeLayout(this.graph, this.filtered, this.bridge, this.$store);
        this.memoBridgeLayout.switchTab(0);
      }
      return this.memoBridgeLayout;
    }
  },

  beforeDestroy: function () {
    this.unwatch();
    this.graph.removeHandler(this.handler);
  },

  mounted: function () {
    var self = this;
    var handle = function(e) {
      if (! self.bridge) return;
      var tgtType = e.target.metadata.Type;
      if (tgtType === 'ofrule'  && e.source.id == self.bridge.id ) {
        self.getRules();
      } else if (tgtType === 'ofgroup'  && e.source.id == self.bridge.id ) {
        self.memoBridgeLayout = null;
      }
    };

    var handleUpdate = function(n) {
      if ((n.metadata.Type == 'ofgroup' && self.memoBridgeLayout.groupsUUID.has(n.metadata.UUID)) ||
          (n.metadata.Type == 'ofrule' && self.memoBridgeLayout.rulesUUID.has(n.metadata.UUID))) {
            self.memoBridgeLayout = null;
      }
    };
    this.handler = {
      onEdgeAdded: handle,
      onEdgeDeleted: handle,
      onNodeUpdated:handleUpdate
    };
    this.graph.addHandler(this.handler);
    this.getRules();
    var self = this;
    this.unwatch = this.$store.watch(
      function () {
        return self.$store.state.currentRule;
      },
      function (newNode, oldNode) {
        if (oldNode) {
          $('#R-' + oldNode.metadata.UUID).removeClass('soft');
        }
        if (newNode) {
          self.layout.switchTab(newNode.metadata.table);
          var p = inport(newNode.metadata.filters);
          self.layout.switchPortTab(newNode.metadata.table, p);
          $('#R-' + newNode.metadata.UUID).addClass('soft');
        }
      }
    )
  },

  methods: {
    addFilter: function(key, value) {
      if (!this.filters[key]) {
        Vue.set(this.filters, key, []);
      }
      this.filters[key].push(value);
      this.getRules();
    },

    removeFilter: function(key, index) {
      this.filters[key].splice(index, 1);
      if (this.filters[key].length === 0) {
        Vue.delete(this.filters, key);
      }
      this.getRules();
    },
    getRules: function() {
      var self = this;
      console.log(this.filters);
      var keys = Object.keys(this.filters);
      if(keys.length == 0) {
        // Short cut no filter.
        this.filtered = null;
        this.memoBridgeLayout = null;
        return;
      }
      var queryRules = "G.V('" + self.bridge.id + "').Out().Has('Type', 'ofrule')";
      var query = "";
      for (var k in this.filters) {
        var valList = this.filters[k];
        var regex = valList.length == 1 ? valList[0] : '(' + valList.join('|') + ')';
        query += queryRules + ".Has('filters', regex('.*" + k + "=" + regex + ".*')).As('" + k + "').";
      }
      query += "Select('" + keys.join("', '") + "')";
      console.log(query);
      this.$topologyQuery(query)
        .then(function(r) {
          self.filtered = r.map(function(node) {return node.Metadata.UUID});
          self.memoBridgeLayout = null;
        });
    }
  }
});
