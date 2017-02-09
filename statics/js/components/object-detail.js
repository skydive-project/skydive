/* jshint multistr: true */

Vue.component('object-detail', {

  name: 'object-detail',

  props: {

    object: {
      type: Object,
      required: true,
    }

  },

  template: '\
    <div class="object-detail">\
      <div class="object-key-value" v-for="(value, key) in object" :class="key.toLowerCase()">\
        <div v-if="typeof value == \'object\'" class="object-sub-detail">\
          <span class="object-key">{{key}}</span>\
          <object-detail :object="value"></object-detail>\
        </div>\
        <div v-else>\
          <span class="object-key">{{key}}</span> :\
          <span class="object-value" :class="typeof(value)">{{value}}</span>\
        </div>\
      </div>\
    </div>\
  ',

});
