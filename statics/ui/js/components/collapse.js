/* jshint multistr: true */

Vue.component('collapse', {

  template: '\
    <div class="collapse-item" :class="{ \'collapse-open\': active }">\
      <div class="collapse-header" @click.prevent="toggle">\
        <slot name="collapse-header" :active="active"></slot>\
      </div>\
      <div class="collapse-content" v-if="active">\
        <div class="collapse-content-box">\
          <slot name="collapse-body"></slot>\
        </div>\
      </div>\
    </div>\
  ',

  data: function() {
    return {
      active: false
    };
  },

  props: {

    collapsed: {
      type: Boolean,
      required: true,
      default: true,
    }

  },

  created: function() {
    this.active = !this.collapsed;
  },

  ready: function() {
    if (this.active) {
      this.$emit('collapse-open', this.index);
    }
  },

  methods: {

    toggle: function() {
      this.active = !this.active;
      if (this.active) {
        this.$emit('collapse-open', this.index);
      } else {
        this.$emit('collapse-close', this.index);
      }
    }

  }

});
