/* jshint multistr: true */

Vue.component('gremlin-console', {

    mixins: [apiMixin,],

    template: '\
    <div>\
      <div class="form-group">\
        <label>Query</label>\
        <textarea id="query" type="text" class="form-control input-sm" rows="5" v-model="query"></textarea>\
      </div>\
      <hr/>\
      <div class="form-group">\
        <label>Result</label>\
        <textarea readonly id="query-result" type="text" class="form-control input-sm" rows="25" v-model="result"></textarea>\
      </div>\
    </div>\
    ',

    data: function() {
      return {
        query: "",
        result: "",
      };
    },

    created: function() {
      this.debouncedQuery = debounce(this.runQuery, 400) 
    },

    watch: {
      query: function() {
        this.debouncedQuery()
      },
    },

    methods: {
      runQuery: function() {
        var self = this;
        this.$topologyQuery(self.query)
          .then(function(data) {
            self.result = JSON.stringify(data, null, 4)
          })
          .catch(function(e) {
            self.result = e.responseText;
          })
      }
    },
});
