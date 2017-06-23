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
      <div class="object-key-value" v-for="(value, key) in object" :class="[typeof(key) == \'string\' ? key.toLowerCase() : key]">\
        <div v-if="Array.isArray(value)">\
          <span class="object-key">{{key}}</span> :\
          <div v-for="(v, index) in value">\
            <div v-if="typeof v == \'object\'" class="object-sub-detail">\
              <span class="object-key" :class="typeof(value)">- </span>\
              <object-detail :object="v"></object-detail>\
            </div>\
            <div v-else class="object-sub-detail">\
              <div class="object-detail" :class="typeof(value)">- {{v}}</div>\
            </div>\
          </div>\
        </div>\
        <div v-else-if="typeof value == \'object\'" class="object-sub-detail">\
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
