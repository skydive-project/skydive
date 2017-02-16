/* jshint multistr: true */

Vue.component('slider', {

  props: {

    value: {
      type: Number,
      default: 0,
    },

    min: {
      type: Number,
      required: true,
    },

    max: {
      type: Number,
      required: true,
    },

    step: {
      type: Number,
      default: 1,
    },

    info: {
      type: String,
    },

  },

  template: '\
    <div>\
      <div class="pull-left">\
        <input type="range" :min="min" v-model="val" :max="max" :step="step" number>\
      </div>\
      <span v-if="info">{{ info }}</span>\
    </div>\
  ',

  data: function() {
    return {
      val: this.value
    };
  },

  watch: {

    val: function() {
      this.$emit('input', parseInt(this.val));
    }

  },

  methods: {

  },

});
