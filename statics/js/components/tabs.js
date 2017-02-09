/* jshint multistr: true */

Vue.component('tab-pane', {

  props: ['title'],

  template: '\
    <div class="tab-pane"\
         v-bind:class="{active: selected}"\
         v-if="selected">\
      <div class="left-cont">\
        <div class="left-panel">\
          <slot></slot>\
        </div>\
      </div>\
    </div>\
  ',

  computed: {

    index: function() {
      return this.$parent.panes.indexOf(this);
    },

    selected: function() {
      return this.index === this.$parent.selected;
    }

  },

  created: function() {
    this.$parent.addPane(this);
  },

  beforeDestroy: function() {
    this.$parent.removePane(this);
  },

});

Vue.component('tabs', {

  template: '\
    <div class="flow-ops-panel">\
      <ul class="nav nav-pills">\
        <li v-for="(pane, index) in panes" v-bind:class="{active: pane.selected}">\
          <a href="#" @click="select(index)">{{pane.title}}</a>\
        </li>\
      </ul>\
      <div class="tab-content clearfix">\
        <slot></slot>\
      </div>\
    </div>\
  ',

  data: function() {
    return {
      panes: [],
      selected: 0
    };
  },

  methods: {

    select: function(index) {
      this.selected = index;
    },

    addPane: function(pane) {
      this.panes.push(pane);
    },

    removePane: function(pane) {
      this.panes.splice(this.panes.indexOf(this), 1);
    },

  }

});
