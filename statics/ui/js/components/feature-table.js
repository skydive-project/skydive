/* jshint multistr: true */

Vue.component('feature-table', {

  props: {

    features: {
      type: Object,
      required: true
    },

  },

  template: '\
    <dynamic-table :rows="rows"\
                   :fields="fields"\
                   :sortOrder="sortOrder"\
                   :sortBy="sortBy"\
                   :toggleFields="toggleFields"\
                   @toggleField="toggleField"\
                   @order="order">\
    </dynamic-table>\
  ',

  data: function() {
    return {
      sortBy: null,
      sortOrder: 1,
      fields: [
        {
          name: ['feature'],
          label: 'Feature',
          show: true,
        },
        {
          name: ['state'],
          label: 'State',
          show: true,
          type: 'boolean'
        },
      ],
      toggleFields: [
        {
          name: ['on'],
          label: 'On',
          show: true,
        },
        {
          name: ['off'],
          label: 'Off',
          show: false,
        },
      ]
    };
  },

  created: function() {
    // sort by feature name by default
    this.sortBy = this.fields[0].name;
  },

  computed: {

    rows: function() {
      var self = this;

      var r = [];
      for (let key in this.features) {
        if (this.isStateViewable(this.features[key])) {
          r.push({"feature": key, "state": this.features[key]});
        }
      }
      r.sort(function(a, b) {
        if (a.feature == b.feature) {
          return 0;
        }
        if (a.feature > b.feature) {
          return 1 * self.sortOrder;
        }

        return -1 * self.sortOrder;
      });

      return r;
    },

  },

  methods: {

    isStateViewable: function(state) {
      if (state && this.toggleFields[0].show) return true;
      if (!state && this.toggleFields[1].show) return true;

      return false;
    },

    order: function(sortOrder) {
      this.sortOrder = sortOrder;
    },

    toggleField: function(field) {
      field.show = !field.show;
    },

  },

});
