/* jshint multistr: true */

window.layoutConfig = new window.TopologyORegistry.config({
  typesWithTextLabels: [],
  colors: {
    collapsedNode: "#c7e9c0",
    unCollapsedNode: "#517587",
    nodeWithoutLeafs: "#c7e9c0"
  },
  radius: {
    collapsedNode: 4.5, // 4.5
    uncollapsedNode: function(d) { return Math.sqrt(d.size) / 10; }
  },
  node: {
    radius: function(d) { return 33; },
    clickSizeDiff: function(d) { return 8; },
    size: function(d) {
      let size = 20;
      if (d.selected) size += 3;
      return size;
    },
    title: function(d) {
      if (d.selected) {
        return d.Metadata.Name;
      }
      return d.Metadata.Name ? d.Metadata.Name.length > 12 ? d.Metadata.Name.substr(0, 10) + "..." : d.Metadata.Name : "";
    },
    typeLabelXShift: function(d) {
      let shift;
      switch (d.Metadata.Type) {
        case 'dpdk':
        case 'device':
          shift = -15;
          break;
        case 'vfio':
          shift = -10;
          break;
        case 'libvirt':
        case 'bridge':
        case 'ovsbridge':
          shift = -13;
          break;
        default: shift = 0;
      }
      return shift;
    },
    typeLabelYShift: function(d) {
      switch (d.Metadata.Type) {
        case 'vfio':
        case 'bridge':
        case 'ovsbridge':
          return 7;
        default: return 5;
      }
    },
    labelColor: function(d) {
      return '#000';
    },
    height: function(d) { return 66; }
  },
  link: {
    className: function(d) {
      if (d.generateD3Path) {
        return "link";
      }
      if (!d.source.data.Metadata || !d.target.data.Metadata) {
        console.log('Absent source or target.', d);
        return "";
      }
      return "link source-" + d.source.data.Metadata.Type + " target-" + d.target.data.Metadata.Type;
    },
    color: function(d) {
      if (d.generateD3Path) {
        return d.generateD3Path().color;
      }
      if (!d.source.data.Metadata || !d.target.data.Metadata) {
        console.log('Absent source or target.', d);
        return "";
      }
      return '#111';
    }
  }
});

window.topologyManager = new window.TopologyORegistry.manager();
const testLayout = new window.TopologyORegistry.layouts.test(".topology-d3");
testLayout.useConfig(window.layoutConfig);
window.topologyManager.layouts.add(testLayout);
