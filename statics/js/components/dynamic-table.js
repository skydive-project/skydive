Vue.component('dynamic-table', {

  props: {

    rows: {
      type: Array,
      required: true,
    },

    error: {
      type: String,
    },

    fields: {
      type: Array,
      required: true,
    },

    sortOrder: {
      type: Number,
    },

    sortBy: {
      type: Array,
    },

  },

  template: '\
    <div v-if="!error" class="dynamic-table">\
      <slot name="header"></slot>\
      <div class="dynamic-table-wrapper">\
        <table class="table table-condensed table-bordered">\
          <thead>\
            <tr>\
              <th v-for="field in fields"\
                  v-if="field.show"\
                  @click="sort(field.name)"\
                  >\
                {{field.label}}\
                <i v-if="field.name == sortBy"\
                   class="pull-right fa"\
                   :class="{\'fa-chevron-down\': sortOrder == 1,\
                            \'fa-chevron-up\': sortOrder == -1}"\
                   aria-hidden="true"></i>\
              </th>\
            </tr>\
          </thead>\
          <tbody>\
            <slot name="row" v-for="row in rows" :row="row"></slot>\
          </tbody>\
        </table>\
      </div>\
      <div class="dynamic-table-actions">\
        <button-dropdown b-class="btn-xs" :auto-close="false">\
          <span slot="button-text">\
            <i class="fa fa-cog" aria-hidden="true"></i>\
          </span>\
          <li v-for="(field, index) in fields">\
            <a href="#" @click="toggleField(field, index)">\
              <small><i class="fa fa-check text-success pull-right"\
                 aria-hidden="true" v-show="field.show"></i>\
              {{field.label}}</small>\
            </a>\
          </li>\
        </button-dropdown>\
        <slot name="actions"></slot>\
      </div>\
    </div>\
    <div v-else class="alert-danger">{{error}}</div>\
  ',

  methods: {

    sort: function(name) {
      if (name == this.sortBy) {
        this.$emit('order', this.sortOrder * -1);
      } else {
        this.$emit('sort', name);
      }
    },

    toggleField: function(field, index) {
      this.$emit('toggleField', field, index);
    },

  },

});
