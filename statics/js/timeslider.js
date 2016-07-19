var formatDate = d3.time.format("%b %d %H:%M:%S");

function GetTimeScale(width) {
  var now = new Date();
  var startTime = new Date();
  startTime.setMinutes(now.getMinutes() - 60);

  return d3.time.scale()
    .domain([startTime, now])
    .range([0, width])
    .clamp(true);
}

function CreateAxis(timeScale) {
  return d3.svg.axis()
    .scale(timeScale)
    .orient("bottom")
    .tickFormat(function(d) {
      return formatDate(d);
    })
    .tickSize(0)
    .tickPadding(15)
    .tickValues([timeScale.domain()[0], timeScale.domain()[1]]);
}

function SetupTimeSlider() {
  var startingValue = new Date();

  var margin = { top: 30, right: 0, bottom: 25, left: 0 };

  var width = 585 - margin.left - margin.right;
  var height = 60 - margin.bottom - margin.top;

  var orig = d3.select(".timeslider-div").append("svg")
    .attr("class", "slider")
    .attr("width", width + margin.left + margin.right)
    .attr("height", height + margin.top + margin.bottom);

  var svg = orig.append("g")
    .attr("transform", "translate(" + margin.left + "," + margin.top + ")");

  var timeScale = GetTimeScale(width);

  var handle;

  // defines brush
  var brush = d3.svg.brush()
    .x(timeScale)
    .extent([startingValue, startingValue])
    .on("brush", function() {
      var value = brush.extent()[0];

      if (d3.event.sourceEvent) { // not a programmatic event
        var v = timeScale.invert(d3.mouse(this)[0]);
        brush.extent([v, v]);
      }
      handle.attr("transform", "translate(" + timeScale(value) + ",0.5)");
      handle.select('text').text(formatDate(value));
      if (topologyLayout.live !== true) {
        topologyLayout.SyncRequest(value.getTime());
      }
    });

  var axis = CreateAxis(timeScale);

  svg.append("g")
    .attr("class", "x axis")
    .attr("transform", "translate(0," + height / 2 + ")")
    .call(axis)
    .select(".domain")
    .select(function() {
      return this.parentNode.appendChild(this.cloneNode(true));
    })
    .attr("class", "halo");

  var slider = svg.append("g")
    .attr("class", "slider")
    .call(brush);

  slider.selectAll(".extent,.resize")
    .remove();

  slider.select(".background")
    .style("cursor", "pointer")
    .attr("height", height);

  handle = slider.append("g")
    .attr("class", "handle");

  handle.append("path")
    .attr("transform", "translate(0," + height / 2 + ")")
    .attr("d", "M 0 -5 V 5");

  handle.append('text')
    .text(startingValue)
    .attr("transform", "translate(" + (-18) + " ," + (height / 2 - 15) + ")");

  slider.call(brush.event);

  $("[name='live-switch']").bootstrapSwitch({
    onSwitchChange: function(event, state) {
      if (state && topologyLayout.live === false) {
        topologyLayout.SyncRequest(Date.now());
      }

      if (state) {
        orig.append("rect")
          .attr("class", "overlay")
          .attr("width", 585)
          .attr("height", 60)
          .style("opacity", 0)
          .on("mouseover", function(d) {
          });

        $(".timeslider-div").addClass("disabled");
        slider.select(".background").style("cursor", "default");

        $("#captures-panel").show();
        $("#add-capture").show();
      }
      else {
        timeScale = GetTimeScale(width);

        brush.extent([timeScale.domain()[1], timeScale.domain()[1]])
          .x(timeScale);
        axis.scale(timeScale)
          .tickValues([timeScale.domain()[0], timeScale.domain()[1]]);
        svg.select("g.axis").call(axis);

        slider.call(brush.event);

        orig.select("rect.overlay").remove();

        $(".timeslider-div").removeClass("disabled");
        slider.select(".background").style("cursor", "pointer");

        $("#captures-panel").hide();
        $("#add-capture").hide();
      }

      topologyLayout.live = state;
      return true;
    }
  });

  $("[name='live-switch']").bootstrapSwitch('state', true, false);
}
