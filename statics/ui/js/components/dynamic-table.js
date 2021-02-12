/* jshint multistr: true */

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

    toggleFields: {
      type: Array,
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
              <th v-for="field in visibleFields"\
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
            <tr v-if="!rows.length" class="bg-warning text-warning">\
              <td :colspan="visibleFields.length">\
                <slot name="empty">No results</slot>\
              </td>\
            </tr>\
            <slot name="row" v-for="row in rows"\
                  :row="row" :visibleFields="visibleFields">\
              <tr class="flow-row">\
                <td v-for="field in visibleFields">\
                  <span v-if="field.type == \'boolean\'">\
                    <i v-if="fieldBoolValue(row, field.name)" class="fa fa-check bool-value-true" aria-hidden="true"></i>\
                    <i v-else class="fa fa-times bool-value-false" aria-hidden="true"></i>\
                  </span>\
                  <span v-else>\
                    {{fieldValue(row, field.name)}}\
                  </span>\
                </td>\
              </tr>\
            </slot>\
          </tbody>\
        </table>\
      </div>\
      <div class="dynamic-table-actions">\
        <slot name="actions"></slot>\
        <div class="dynmic-table-actions-item">\
          <button-dropdown b-class="btn-xs" :auto-close="false">\
            <span slot="button-text">\
              <i class="fa fa-cog" aria-hidden="true"></i>\
            </span>\
            <li v-for="(field, index) in toggleEntries">\
              <a href="#" @click="toggleField(field, index)">\
                <small><i class="fa fa-check text-success pull-right"\
                  aria-hidden="true" v-show="field.show"></i>\
                {{field.label}}</small>\
              </a>\
            </li>\
          </button-dropdown>\
        </div>\
      </div>\
    </div>\
    <div v-else class="alert-danger">{{error}}</div>\
  ',

  computed: {

    visibleFields: function() {
      return this.fields.filter(function(f) {
        return f.show === true;
      });
    },

    toggleEntries: function() {
      if (!this.toggleFields) {
        return this.fields;
      }
      return this.toggleFields;
    },

  },

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

    fieldBoolValue: function(object, key) {
      return object[key[0]] == true;
    },

    fieldValue: function(object, key) {
      var value = object[key[0]];
      if (isNaN(value)) return value;
      return value.toLocaleString();
    },

  },

});
