/* jshint multistr: true */

Vue.component('tab-pane', {

  props: ['title'],

  template: '\
    <div class="tab-pane"\
         v-bind:class="{active: selected}"\
         v-if="selected">\
      <div class="panel">\
        <div class="panel-content">\
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
        <li v-for="(pane, index) in panes" v-bind:class="{active: pane.selected}" @click="select(index)" style="cursor: pointer">\
          <a v-bind:id="pane.title">{{pane.title}}</a>\
        </li>\
      </ul>\
      <div class="tab-content clearfix">\
        <slot></slot>\
      </div>\
    </div>\
  ',

  props: {

    active: {
      type: Number,
      default: 0
    },

  },

  data: function() {
    return {
      panes: [],
      selected: 0
    };
  },

  watch: {

    active: function(newValue) {
      this.select(newValue);
    },

  },

  methods: {

    select: function(index) {
      this.selected = index;
    },

    addPane: function(pane) {
      this.panes.push(pane);
    },

    removePane: function(pane) {
      var idx = this.panes.indexOf(pane);
      this.panes.splice(idx, 1);
      if (idx <= this.selected) {
        this.selected -= 1;
      }
    },

  }

});
