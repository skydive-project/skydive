/* jshint multistr: true */

window.layoutConfig = new window.TopologyORegistry.config({
});

window.topologyManager = new window.TopologyORegistry.manager();
window.topologyManager.layouts.add(new window.TopologyORegistry.layouts.test(".topology-d3"));
