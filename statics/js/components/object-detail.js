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
    },

    transformer: {
      type: Function
    },

    path: {
      type: String
    },

  },

  template: '\
    <div class="object-detail">\
      <div class="object-key-value" v-for="(value, key) in object" :class="[typeof(key) == \'string\' ? key.toLowerCase() : key]">\
        <div v-if="Array.isArray(value)">\
          <a class="object-key" data-toggle="collapse" :href="\'#\' + getNewUniqueId()" class="collapse-title">{{key}} :\
            <i class="indicator glyphicon glyphicon-chevron-down pull-right"></i>\
          </a>\
          <div :class="[collapsedByDefault(key) ? \'collapse\' : \'collapse in\']" :id="uniqueId()">\
            <div v-for="(v, index) in value">\
              <div v-if="typeof v == \'object\'" class="object-sub-detail" style="margin-left: 20px;">\
                <span v-if="Object.keys(v).length > 0" class="object-key" :class="typeof(value)" style="float:left">- </span>\
                <object-detail :object="v" :path="path ? path+\'.\'+key : key" :transformer="transformer"></object-detail>\
              </div>\
              <div v-else class="object-sub-detail">\
                <div class="object-detail" :class="typeof(value)">- {{ transform(path ? path+\'.\'+key : key, v) }}</div>\
              </div>\
            </div>\
          </div>\
        </div>\
        <div v-else-if="typeof value == \'object\'" class="object-sub-detail">\
          <span class="object-key">{{key}}</span>\
          <object-detail :object="value" :path="path ? path+\'.\'+key : key" :transformer="transformer"></object-detail>\
        </div>\
        <div v-else>\
          <span class="object-key">{{key}}</span> :\
          <span class="object-value" :class="typeof(value)" v-html="transform(path ? path+\'.\'+key : key, value)"></span>\
          <i v-if="links && links[key]" :class="links[key].class" @click="links[key].onClick" \
            @mouseover="links[key].onMouseOver" @mouseout="links[key].onMouseOut"></i>\
        </div>\
      </div>\
    </div>\
  ',

  methods: {

    transform: function(key, value) {
      if (this.transformer) {
        return this.transformer(key, value);
      }
      return value;
    },

    collapsedByDefault: function(key) {
      if (key === "FDB" || key === "Neighbors" || key === "RoutingTable") return true;
      return false;
    },
  }
});
