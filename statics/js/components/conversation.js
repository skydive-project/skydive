/* jshint multistr: true */

var ConversationComponent = {

  name: 'Conversation',

  mixins: [apiMixin],

  template: '\
    <div class="conversation">\
      <div class="col-sm-8 fill content">\
        <div class="conversation-d3"></div>\
      </div>\
      <div class="col-sm-4 fill sidebar">\
        <div class="left-cont">\
          <div class="left-panel">\
            <form>\
              <div class="form-group" v-if="false">\
                <label for="order-type">Order</label>\
                <select id="order-type" v-model="order" class="form-control input-sm">\
                  <option v-for="order in orders" :value="order.value">{{ order.label }}</option>\
                </select>\
              </div>\
              <div class="form-group">\
                <label for="layer-type">Layer</label>\
                <select id="layer-type" v-model="layer" class="form-control input-sm">\
                  <option v-for="layer in layers" :value="layer.value">{{ layer.label }}</option>\
                </select>\
              </div>\
            </form>\
          </div>\
          <div class="left-panel" v-if="node">\
            <div class="title-left-panel">Interface detail</div>\
            <object-detail :object="node"></object-detail>\
          </div>\
        </div>\
      </div>\
    </div>\
  ',

  data: function() {
    return {
      layers: [
        {label: "Ethernet", value: "ethernet"},
        {label: "IPv4", value: "ipv4"},
        {label: "IPv6", value: "ipv6"},
        {label: "TCP", value: "tcp"},
        {label: "UDP", value: "udp"},
        {label: "SCTP", value: "sctp"},
      ],
      layer: "ethernet",
      orders: [
        {label: "by Name", value: "name"},
        {label: "by Frequency", value: "count"},
        {label: "by Application", value: "group"},
      ],
      order: "name",
      node: null,
    };
  },

  mounted: function() {
    this.layout = new ConversationLayout(this, ".conversation-d3");
    this.layout.ShowConversation("ethernet");
  },

  watch: {

    layer: function() {
      this.layout.ShowConversation(this.layer);
    },

    order: function() {
      this.layout.Order(this.order);
    },

  },

};

var ConversationLayout = function(vm, selector) {
  this.vm = vm;
  this.width = 600;
  this.height = 600;

  var margin = {top: 100, right: 0, bottom: 10, left: 100};

  this.svg = d3.select(selector).append("svg")
    .attr("width", this.width + margin.left + margin.right)
    .attr("height", this.height + margin.top + margin.bottom)
    .style("margin-left", -margin.left + 20 + "px")
    .append("g")
    .attr("transform", "translate(" + margin.left + "," + margin.top + ")");

  this.orders = {};
};

ConversationLayout.prototype.Order = function(order) {
  if (!(order in this.orders))
    return;

  var x = d3.scale.ordinal().rangeBands([0, this.width]);

  x.domain(this.orders[order]);

  var t = this.svg.transition().duration(2500);

  t.selectAll(".row")
  .delay(function(d, i) { return x(i) * 4; })
  .attr("transform", function(d, i) {
    return "translate(0," + x(i) + ")";
  })
  .selectAll(".cell")
  .delay(function(d) { return x(d.x) * 4; })
  .attr("x", function(d) { return x(d.x); });

  t.selectAll(".column")
  .delay(function(d, i) { return x(i) * 4; })
  .attr("transform", function(d, i) {
    return "translate(" + x(i) + ")rotate(-90)";
  });
};

ConversationLayout.prototype.NodeDetails = function(node) {
  this.vm.node = node;
};

ConversationLayout.prototype.ShowConversation = function(layer) {
  this.svg.selectAll("*").remove();

  var _this = this;
  var gremlinQuery = "g.Flows()";
  var flowLayer = "";
  var layerMap = {};

  switch (layer) {
    case "ethernet":
      gremlinQuery += ".Has('Link.Protocol', 'ETHERNET')";
      flowLayer = "Link";
      break;
    case "ipv4":
      gremlinQuery += ".Has('Network.Protocol', 'IPV4')";
      flowLayer = "Network";
      break;
    case "ipv6":
      gremlinQuery += ".Has('Network.Protocol', 'IPV6')";
      flowLayer = "Network";
      break;
    case "udp":
      gremlinQuery += ".Has('Transport.Protocol', 'UDPPORT')";
      flowLayer = "Transport";
      break;
    case "tcp":
      gremlinQuery += ".Has('Transport.Protocol', 'TCPPORT')";
      flowLayer = "Transport";
      break;
    case "sctp":
      gremlinQuery += ".Has('Transport.Protocol', 'SCTPPORT')";
      flowLayer = "Transport";
      break;
    default:
      return;
  }

  this.vm.$topologyQuery(gremlinQuery)
    .then(function(data) {
      var nodes = [];
      var links = [];

      if (data === null)
        return [];

      data.forEach(function(flow, i) {
        var l = flow[flowLayer];
        var AB = l.A;
        var BA = l.B;

        if (layerMap[AB] === undefined) {
          layerMap[AB] = Object.keys(layerMap).length;
          nodes.push({"name": AB, "group": 0});
        }

        if (layerMap[BA] === undefined) {
          layerMap[BA] = Object.keys(layerMap).length;
          nodes.push({"name": BA, "group": 0});
        }

        links.push({"source": layerMap[AB], "target": layerMap[BA], "value": flow.Metric.ABBytes + flow.Metric.BABytes});
      });

      var matrix = [];
      var n = nodes.length;

      // Compute index per node.
      nodes.forEach(function(node, i) {
        node.index = i;
        node.count = 0;
        matrix[i] = d3.range(n).map(function(j) { return {x: j, y: i, z: 0}; });
      });

      // Convert links to matrix; count character occurrences.
      links.forEach(function(link) {
        matrix[link.source][link.target].z += link.value;
        matrix[link.target][link.source].z += link.value;
        nodes[link.source].count += link.value;
        nodes[link.target].count += link.value;
      });

      // Precompute the orders.
      _this.orders = {
        name: d3.range(n).sort(function(a, b) {
          return d3.ascending(nodes[a].name, nodes[b].name);
        }),
        count: d3.range(n).sort(function(a, b) {
          return nodes[b].count - nodes[a].count;
        }),
        group: d3.range(n).sort(function(a, b) {
          return nodes[b].group - nodes[a].group;
        })
      };

      var x = d3.scale.ordinal().rangeBands([0, _this.width]);
      var z = d3.scale.linear().domain([0, 4]).clamp(true);
      var c = d3.scale.category10().domain(d3.range(10));

      // The default sort order.
      x.domain(_this.orders.name);

      _this.svg.append("rect")
      .attr("class", "background")
      .attr("width", _this.width)
      .attr("height", _this.height);

      var row = _this.svg.selectAll(".row")
      .data(matrix)
      .enter().append("g")
      .attr("class", "row")
      .attr("transform", function(d, i) {
        return "translate(0," + x(i) + ")"; })
      .each(function(row) {
        var cell = d3.select(this).selectAll(".cell")
        .data(row.filter(function(d) { return d.z; }))
        .enter().append("rect")
        .attr("class", "cell")
        .attr("x", function(d) { return x(d.x); })
        .attr("width", x.rangeBand())
        .attr("height", x.rangeBand())
        .style("fill-opacity", function(d) { return z(d.z); })
        .style("fill", function(d) { return "rgb(31, 119, 180)"; })
        .on("mouseover", function(p) {
          d3.selectAll(".row text").classed("active", function(d, i) { return i == p.y; });
          d3.selectAll(".column text").classed("active", function(d, i) { return i == p.x; });
        })
        .on("mouseout", function(p) {
          d3.selectAll("text").classed("active", false);
        });
      });

      row.append("line")
        .attr("x2", _this.width);

      row.append("text")
      .attr("x", -6)
      .attr("y", x.rangeBand() / 2)
      .attr("dy", ".32em")
      .attr("text-anchor", "end")
      .text(function(d, i) { return nodes[i].name; });

      var column = _this.svg.selectAll(".column")
      .data(matrix)
      .enter().append("g")
      .attr("class", "column")
      .attr("transform", function(d, i) {
        return "translate(" + x(i) + ")rotate(-90)";
      });

      column.append("line")
      .attr("x1", -_this.width);

      column.append("text")
      .attr("x", 6)
      .attr("y", x.rangeBand() / 2)
      .attr("dy", ".32em")
      .attr("text-anchor", "start")
      .text(function(d, i) { return nodes[i].name; });
    });
};
