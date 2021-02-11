/* jshint multistr: true */

Vue.component('panel', {

  props: {

    title: {
      type: String,
      required: true,
    },

    description: {
      type: String,
      required: false,
    },

    collapsed: {
      type: Boolean,
      default: true,
    }

  },

  template: '\
    <div class="panel">\
      <collapse :collapsed="collapsed" class="panel-content">\
        <h1 slot="collapse-header" slot-scope="props" :class="{\'closed\': !props.active}">\
          {{ title }}\
          <span class="pull-right">\
            <i class="glyphicon glyphicon-chevron-left rotate" :class="{\'down\': props.active}"></i>\
          </span>\
          <span v-if="description" class="description">{{ description }}</span>\
        </h1>\
        <div slot="collapse-body">\
          <div ref="body">\
            <slot>Empty panel</slot>\
          </div>\
        </div>\
      </collapse>\
      <div class="panel-actions">\
        <slot name="actions"></slot>\
      </div>\
    </div>\
  ',

});
