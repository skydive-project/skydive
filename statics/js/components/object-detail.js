/* jshint multistr: true */

Vue.component('object-detail', {

  name: 'object-detail',

  mixins: [notificationMixin],

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

    pathPrefix: {
      type: String,
      default: "",
    },

    collapsed: {
      type: Object,
      default: function() { return {}; },
    },

  },

  template: '\
    <div class="object-detail">\
      <div class="object-key-value" v-for="(value, key) in object" :class="[typeof(key) == \'string\' ? key.toLowerCase() : key]">\
        <div v-if="Array.isArray(value)">\
          <collapse :collapsed="collapsedState(path(key))">\
            <div slot="collapse-header" slot-scope="props" class="object-key">\
            {{key}} :\
            <span class="pull-right">\
              <i class="glyphicon glyphicon-chevron-left rotate" :class="{\'down\': props.active}"></i>\
            </span>\
            </div>\
            <div slot="collapse-body">\
              <div v-for="(v, index) in value">\
                <div v-if="typeof v == \'object\'" class="object-sub-detail" style="margin-left: 20px;">\
                  <span v-if="Object.keys(v).length > 0" class="object-key" :class="typeof(value)" style="float:left">- </span>\
                  <object-detail :object="v" :pathPrefix="path(key)" :transformer="transformer" :collapsed="collapsed"></object-detail>\
                </div>\
                <div v-else class="object-sub-detail">\
                  <div class="object-detail copy-clipboard" :class="typeof(value)" @click="copyToClipboard(v)">- {{ transform(key, v) }}</div>\
                </div>\
              </div>\
            </div>\
          </collapse>\
        </div>\
        <div v-else-if="typeof value == \'object\' && !$.isEmptyObject(value)" class="object-sub-detail">\
          <collapse :collapsed="collapsedState(path(key))">\
            <div slot="collapse-header" slot-scope="props" class="object-key">\
              {{key}} :\
              <span class="pull-right">\
                <i class="glyphicon glyphicon-chevron-left rotate" :class="{\'down\': props.active}"></i>\
              </span>\
            </div>\
            <div slot="collapse-body">\
              <object-detail :object="value" :pathPrefix="path(key)" :transformer="transformer" :collapsed="collapsed"></object-detail>\
            </div>\
          </collapse>\
        </div>\
        <div v-else-if="typeof value == \'boolean\'">\
          <span class="object-key">{{key}}</span> :\
          <span class="object-value copy-clipboard">\
            <i v-if="value == true" class="fa fa-check bool-value-true" aria-hidden="true"></i>\
            <i v-else class="fa fa-times bool-value-false" aria-hidden="true"></i>\
          </span>\
        </div>\
        <div v-else>\
          <span class="object-key">{{key}}</span> :\
          <span v-if="typeof value != \'object\' || !$.isEmptyObject(value)" class="object-value copy-clipboard" :class="typeof(value)" v-html="transform(key, value)" @click="copyToClipboard(value)"></span>\
          <i v-if="links && links[key]" :class="links[key].class" @click="links[key].onClick" \
            @mouseover="links[key].onMouseOver" @mouseout="links[key].onMouseOut"></i>\
        </div>\
      </div>\
    </div>\
  ',

  methods: {

    path: function(key) {
      if (this.pathPrefix) {
        return this.pathPrefix + "." + key;
      }
      else {
        return key;
      }
    },

    transform: function(key, value) {
      if (this.transformer) {
        return this.transformer(this.path(key), value);
      }
      return value;
    },

    collapsedState: function(path) {
      if (path in this.collapsed)
        return this.collapsed[path];
      return true;
    },

    copyToClipboard(value) {
      var textArea = document.createElement("textarea");
      textArea.value = value;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand("Copy");
      textArea.remove();

      this.$success({message: 'Copied `' + value + '`'});
    }
  }
});
