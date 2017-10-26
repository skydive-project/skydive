/* jshint multistr: true */

Vue.component('object-detail', {

  name: 'object-detail',

  props: {

    object: {
      type: Object,
      required: true,
    },

    links: {
      type: Object
    }

  },

  template: '\
    <div class="object-detail">\
      <div class="object-key-value" v-for="(value, key) in object" :class="[typeof(key) == \'string\' ? key.toLowerCase() : key]">\
        <div v-if="Array.isArray(value)">\
          <a class="object-key" data-toggle="collapse" :href="\'#\' + key" class="collapse-title">{{key}} :\
            <i class="indicator glyphicon glyphicon-chevron-down pull-right"></i>\
          </a>\
          <div :class="[collapsedByDefault(key) ? \'collapse\' : \'collapse in\']" :id="key">\
            <div v-for="(v, index) in value">\
              <div v-if="typeof v == \'object\'" class="object-sub-detail" style="margin-left: 20px;">\
                <span class="object-key" :class="typeof(value)" style="float:left">- </span>\
                <object-detail :object="v"></object-detail>\
              </div>\
              <div v-else class="object-sub-detail">\
                <div class="object-detail" :class="typeof(value)">- {{v}}</div>\
              </div>\
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
          <i v-if="links && links[key]" class="indicator glyphicon glyphicon-download-alt raw-packet-link" @click="links[key]"></i>\
        </div>\
      </div>\
    </div>\
  ',

  methods: {

    collapsedByDefault: function(key) {
      if (key === "FDB" || key === "Neighbors") return true;
      return false;
    }

  }

});
