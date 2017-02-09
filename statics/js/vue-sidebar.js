var VueSidebar = {
  el: "#vue-sidebar",

  data: {
    service: null,
    currentNode: null,
  },

  created: function() {
    var self = this;
    this.$on('NODE_SELECTED', function(node) {
      self.currentNode = node;
    });
    this.$on('NODE_DELETED', function(node) {
      if (self.currentNode && node.ID === self.currentNode.ID) {
        self.currentNode = null;
      }
    });
  },

  computed: {

    currentNodeFlowsQuery: function() {
      if (this.currentNode)
        return "G.V('" + this.currentNode.ID + "').Flows().Dedup()";
      return "";
    },

  }

};
