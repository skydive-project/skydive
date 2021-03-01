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
      <div class="object-key-value" v-for="obj in orderedObject" :class="[typeof(obj.key) == \'string\' ? obj.key.toLowerCase() : obj.key]">\
        <div v-if="Array.isArray(obj.value)">\
          <collapse :collapsed="collapsedState(path(obj.key))">\
            <div slot="collapse-header" slot-scope="props" class="object-key">\
            {{obj.key}} :\
            <span class="pull-right">\
              <i class="glyphicon glyphicon-chevron-left rotate" :class="{\'down\': props.active}"></i>\
            </span>\
            </div>\
            <div slot="collapse-body">\
              <div v-for="(v, index) in obj.value">\
                <div v-if="typeof v == \'object\'" class="object-sub-detail" style="margin-left: 20px;">\
                  <span v-if="Object.keys(v).length > 0" class="object-key" :class="typeof(value)" style="float:left">- </span>\
                  <object-detail :object="v" :pathPrefix="path(obj.key)" :transformer="transformer" :collapsed="collapsed"></object-detail>\
                </div>\
                <div v-else class="object-sub-detail">\
                  <div class="object-detail copy-clipboard" :class="typeof(obj.value)" @click="copyToClipboard(v)">- {{ transform(obj.key, v) }}</div>\
                </div>\
              </div>\
            </div>\
          </collapse>\
        </div>\
        <div v-else-if="typeof obj.value == \'object\' && !$.isEmptyObject(obj.value)" class="object-sub-detail">\
          <collapse :collapsed="collapsedState(path(obj.key))">\
            <div slot="collapse-header" slot-scope="props" class="object-key">\
              {{obj.key}} :\
              <span class="pull-right">\
                <i class="glyphicon glyphicon-chevron-left rotate" :class="{\'down\': props.active}"></i>\
              </span>\
            </div>\
            <div slot="collapse-body">\
              <object-detail :object="obj.value" :pathPrefix="path(obj.key)" :transformer="transformer" :collapsed="collapsed"></object-detail>\
            </div>\
          </collapse>\
        </div>\
        <div v-else-if="typeof obj.value == \'boolean\' && !$.isEmptyObject(obj.value)">\
          <span class="object-key">{{obj.key}}</span> :\
          <span class="object-value copy-clipboard">\
            <i v-if="value == true" class="fa fa-check bool-value-true" aria-hidden="true"></i>\
            <i v-else class="fa fa-times bool-value-false" aria-hidden="true"></i>\
          </span>\
        </div>\
        <div v-else>\
          <span class="object-key">{{obj.key}}</span> :\
          <span v-if="typeof obj.value != \'object\' || !$.isEmptyObject(obj.value)" class="object-value copy-clipboard" :class="typeof(obj.value)" v-html="transform(obj.key, obj.value)" @click="copyToClipboard(obj.value)"></span>\
          <i v-if="links && links[obj.key]" :class="links[obj.key].class" @click="links[obj.key].onClick()" \
            @mouseover="links[obj.key].onMouseOver" @mouseout="links[obj.key].onMouseOut"></i>\
        </div>\
      </div>\
    </div>\
  ',

  computed: {
    orderedObject: function() {
      var entries = [];

      for (var key in this.object) {
        entries.push({"key": key, "value": this.object[key]});
      }

      entries.sort(function(a, b) { return a.key.localeCompare(b.key)});
      return entries;
    }
  },

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
