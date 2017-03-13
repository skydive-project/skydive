Vue.component('statistics-table', {

  props: {

    object: {
      type: Object,
      required: true
    },

  },

  template: '\
    <dynamic-table :rows="rows"\
                   :fields="fields"\
                   @toggleField="toggleField">\
      <template slot="row" scope="stats">\
        <tr class="flow-row">\
          <td v-for="field in visibleFields">\
            {{fieldValue(stats.row, field.name)}}\
          </td>\
        </tr>\
      </template>\
    </dynamic-table>\
  ',

  data: function() {
    return {
      fields: []
    };
  },

  created: function() {
    this.generateFields();
  },

  watch: {

    object: function() {
      this.updateFields();
    },

  },

  computed: {

    time: function() {
      return this.$store.state.time;
    },

    timeHuman: function() {
      return this.$store.getters.timeHuman;
    },

    rows: function() {
      return [this.object];
    },

    visibleFields: function() {
      return this.fields.filter(function(f) {
        return f.show === true;
      });
    },

  },

  methods: {

    isTime: function(key) {
      return ['Start', 'Last'].indexOf(key.split('/')[1]) !== -1;
    },

    toggleField: function(field) {
      field.show = !field.show;
      // mark the field if is has been changed by the user
      field.showChanged = true;
    },

    generateFields: function() {
      // at creation show only fields that have a value gt 0
      var self = this;
      Object.getOwnPropertyNames(this.object).forEach(function(key) {
        var f = {
          name: [key],
          label: key.split('/')[1],
          show: self.object[key] > 0,
          showChanged: false
        };
        // put Start and Last fields at the beginning
        if (self.isTime(key)) {
          self.fields.splice(0, 0, f);
        } else {
          self.fields.push(f);
        }
      });
    },

    updateFields: function() {
      // show field automatically if some value is gt 0
      // unless it has been hidden or showed manually by
      // the user.
      var self = this;
      this.fields.forEach(function(f) {
        var newVal = self.object[f.name[0]];
        if (f.showChanged === false && newVal > 0) {
          f.show = true;
        }
      });
    },

    fieldValue: function(object, key) {
      key = key[0];
      if (this.isTime(key)) {
        return new Date(object[key]).toLocaleTimeString();
      }
      return object[key];
    },

  },


});
