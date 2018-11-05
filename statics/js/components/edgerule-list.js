/* jshint multistr: true */

var EdgeRule = {

  mixins: [apiMixin, notificationMixin],

  props: {
    edgerule: {
      type: Object,
      required: true,
    }
  },

  template: '\
    <div class="rule-item">\
      <div class="rule-title">\
        <i class="rule-action rule-delete fa fa-trash"\
          @click="remove(edgerule)">\
        </i>\
        {{edgerule.UUID}}\
      </div>\
      <dl class="dl-horizontal">\
        <dt v-if="edgerule.Name">Name</dt>\
        <dd v-if="edgerule.Name">{{edgerule.Name}}</dd>\
        <dt v-if="edgerule.Description">Description</dt>\
        <dd v-if="edgerule.Description">{{edgerule.Description}}</dd>\
        <dt v-if="edgerule.Query">Query</dt>\
        <dd v-if="edgerule.Query">{{edgerule.Query}}</dd>\
      </dl>\
    </div>\
  ',

  methods: {
    remove: function(er) {
      this.edgeruleAPI.delete(er.UUID)
        .catch(function(e) {
          self.$error({message: 'Edge Rule delete error: ' + e.responseText});
          return e;
        })
        .finally(function() {
          app.$emit("refresh-edgerule-list");
        });
    },
  },
};

Vue.component('edgerule-list', {

  mixins: [apiMixin, notificationMixin],

  components: {
    'edgerule': EdgeRule,
  },

  template: '\
    <div class="sub-panel" v-if="count > 0">\
      <ul class="rule-list">\
        <li class="rule-item" v-for="edgerule in edgerules" :id="edgerule.UUID">\
          <edgerule :edgerule="edgerule"></edgerule>\
        </li>\
      </ul>\
    </div>\
  ',

  data: function() {
    return {
      edgerules: {},
      intervalID: "",
    };
  },

  computed: {
    count: function() {
      return Object.keys(this.edgerules).length;
    },
  },

  created: function() {
    var self = this;
    this.intervalID = setInterval(this.getEdgeRuleList.bind(this), 30000);
    this.getEdgeRuleList();
    app.$on("refresh-edgerule-list", function() {
      self.getEdgeRuleList();
    });
  },

  destroyed: function() {
    clearInterval(this.intervalID);
  },

  methods: {
    getEdgeRuleList: function() {
      var self = this;
      self.edgeruleAPI.list()
        .then(function(data) {
          self.edgerules = data;
        })
        .catch(function(e) {
          self.$error({message: 'Edge rule list error: ' + e.responseText});
          return e;
        });
    },
  },
});
