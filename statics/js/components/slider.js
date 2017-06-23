/* jshint multistr: true */

Vue.component('slider', {

  components: {
    'vue-slider': vueSlider
  },

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
      <div class="slide">\
        <vue-slider ref="slider" v-model="val" :min="min" :max="max" width="300" height="8" dotSize="14" lazy=true :interval="step" show=true tooltip="hover" tooltip-dir="bottom" formatter="{value} min."></vue-slider>\
      </div>\
      <span class="slider-info" v-if="info">{{ info }}</span>\
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
