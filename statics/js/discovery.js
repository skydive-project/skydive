var DiscoveryLayout = function(selector) {
  this.width = 680;
  this.height = 600;
  this.radius = (Math.min(this.width, this.height) / 2) - 50;
  this.color = d3.scale.category20c();

  // Breadcrumb dimensions: width, height, spacing, width of tip/tail.
  this.b = {
    w: 75, h: 30, s: 3, t: 10
  };

  this.svg = d3.select(selector).append("svg")
  .attr("width", this.width)
  .attr("height", this.height)
  .append("g")
  .attr("id", "container")
  .attr("transform", "translate(" + this.width / 2 + "," + this.height * 0.52 + ")");

  this.partition = d3.layout.partition()
  .sort(null)
  .size([2 * Math.PI, this.radius * this.radius])
  .value(function(d) { return 1; });

  this.arc = d3.svg.arc()
  .startAngle(function(d) { return d.x; })
  .endAngle(function(d) { return d.x + d.dx; })
  .innerRadius(function(d) { return Math.sqrt(d.y); })
  .outerRadius(function(d) { return Math.sqrt(d.y + d.dy); });

  var _this = this;
  _this.initializeBreadcrumbTrail();
  d3.selectAll("#type").on("change", function() {
    _this.DrawChart(this.value);
  });
};

DiscoveryLayout.prototype.DrawChart = function(type) {
  var totalSize = 0;
  this.svg.selectAll("*").remove();
  var _this = this;
  //assign bytes as default if no type given.
  type = (type === undefined) ? "bytes" : type;
  d3.json("/api/flow/discovery/" + type, function(root) {
    var path = _this.svg.datum(root).selectAll("path")
      .data(_this.partition.nodes)
      .enter().append("path")
      .attr("display", function(d) { return d.depth ? null : "none"; }) // hide inner ring
      .attr("d", _this.arc)
      .style("stroke", "#fff")
      .style("fill", function(d) { return _this.color((d.children ? d : d.parent).name); })
      .style("fill-rule", "evenodd")
      .on("mouseover", mouseover)
      .each(stash);
    totalSize = path.node().__data__.value;

    // Add the mouseleave handler to the bounding circle
    d3.select("#container").on("mouseleave", mouseleave);

    d3.selectAll("#mode").on("change", function change() {
      var value = this.value === "count" ? function() { return 1; } : function(d) { return d.size; };

      path
        .data(_this.partition.value(value).nodes)
        .transition()
        .duration(1500)
        .attrTween("d", arcTween);
    });
  });

  // On mouseover function
  function mouseover(d) {
    var percentage = (100 * d.value / totalSize).toPrecision(3) + " %";
    var protocol_data = {
      "Name": d.name,
      "Percentage": percentage,
      "Size": d.size,
      "Value": d.value,
      "Depth": d.depth
    };
    var json = JSON.stringify(protocol_data);
    $("#protocol_data").JSONView(json);
    var sequenceArray = getAncestors(d);
    updateBreadcrumbs(sequenceArray, percentage);
  }

  // On mouseleave function
  function mouseleave(d) {
    d3.select("#trail")
        .style("visibility", "hidden");

    $("#protocol_data").JSONView({});
  }

  // Given a node in a partition layout, return an array of all of its ancestor
  // nodes, highest first, but excluding the root.
  function getAncestors(node) {
    var path = [];
    var current = node;
    while (current.parent) {
      path.unshift(current);
      current = current.parent;
    }
    return path;
  }

  // Generate a string that describes the points of a breadcrumb polygon.
  function breadcrumbPoints(d, i) {
    var points = [];
    points.push("0,0");
    points.push(_this.b.w + ",0");
    points.push(_this.b.w + _this.b.t + "," + (_this.b.h / 2));
    points.push(_this.b.w + "," + _this.b.h);
    points.push("0," + _this.b.h);
    if (i > 0) { // Leftmost breadcrumb; don't include 6th vertex.
      points.push(_this.b.t + "," + (_this.b.h / 2));
    }
    return points.join(" ");
  }

  //Update the breadcrumb trail to show the current sequence and percentage.
  function updateBreadcrumbs(nodeArray, percentageString) {

    // Data join; key function combines name and depth (= position in sequence).
    var g = d3.select("#trail")
        .selectAll("g")
        .data(nodeArray, function(d) { return d.name + d.depth; });

    // Add breadcrumb and label for entering nodes.
    var entering = g.enter().append("svg:g");

    entering.append("svg:polygon")
        .attr("points", breadcrumbPoints)
        .style("fill", function(d) { return _this.color(d.name); });

    entering.append("svg:text")
        .attr("x", (_this.b.w + _this.b.t) / 2)
        .attr("y", _this.b.h / 2)
        .attr("dy", "0.35em")
        .attr("text-anchor", "middle")
        .text(function(d) { return d.name; });

    // Set position for entering and updating nodes.
    g.attr("transform", function(d, i) {
      return "translate(" + i * (_this.b.w + _this.b.s) + ", 0)";
    });

    // Remove exiting nodes.
    g.exit().remove();

    // Now move and update the percentage at the end.
    d3.select("#trail").select("#endlabel")
        .attr("x", (nodeArray.length + 0.5) * (_this.b.w + _this.b.s))
        .attr("y", _this.b.h / 2)
        .attr("dy", "0.35em")
        .attr("text-anchor", "middle")
        .text(percentageString);

    // Make the breadcrumb trail visible, if it's hidden.
    d3.select("#trail")
        .style("visibility", "");
  }

  // Stash the old values for transition.
  function stash(d) {
    d.x0 = d.x;
    d.dx0 = d.dx;
  }

  // Interpolate the arcs in data space.
  function arcTween(a) {
    var i = d3.interpolate({x: a.x0, dx: a.dx0}, a);
    return function(t) {
      var b = i(t);
      a.x0 = b.x;
      a.dx0 = b.dx;
      return _this.arc(b);
    };
  }

  d3.select(self.frameElement).style("height", this.height + "px");
};

DiscoveryLayout.prototype.initializeBreadcrumbTrail = function() {
  // Add the svg area.
  var trail = d3.select("#sequence").append("svg:svg")
      .attr("width", this.width)
      .attr("height", 50)
      .attr("id", "trail");
  // Add the label at the end, for the percentage.
  trail.append("svg:text")
    .attr("id", "endlabel")
    .style("fill", "#fff");
};
