/* jshint multistr: true */

var NodeRule = {

  mixins: [apiMixin, notificationMixin],

  props: {
    noderule: {
      type: Object,
      required: true,
    }
  },

  template: '\
    <div class="rule-item">\
      <div class="rule-title">\
        <i class="rule-action rule-delete fa fa-trash"\
          @click="remove(noderule)">\
        </i>\
        {{noderule.UUID}}\
      </div>\
      <dl class="dl-horizontal">\
        <dt v-if="noderule.Name">Name</dt>\
        <dd v-if="noderule.Name">{{noderule.Name}}</dd>\
        <dt v-if="noderule.Description">Description</dt>\
        <dd v-if="noderule.Description">{{noderule.Description}}</dd>\
        <dt v-if="noderule.Query">Query</dt>\
        <dd v-if="noderule.Query">{{noderule.Query}}</dd>\
        <dt v-if="noderule.Action">Action</dt>\
        <dd v-if="noderule.Action">{{noderule.Action}}</dd>\
        <dt v-if="noderule.Metadata.Type">Type</dt>\
        <dd v-if="noderule.Metadata.Type">{{noderule.Metadata.Type}}</dd>\
      </dl>\
    </div>\
  ',

  methods: {
    remove: function(nr) {
      this.noderuleAPI.delete(nr.UUID)
        .catch(function(e) {
          self.$error({message: 'Node Rule delete error: ' + e.responseText});
          return e;
        })
        .finally(function() {
          app.$emit("refresh-noderule-list");
        });
    },
  },
};

Vue.component('noderule-list', {

  mixins: [apiMixin, notificationMixin],

  components: {
    'noderule': NodeRule,
  },

  template: '\
    <div class="sub-panel" v-if="count > 0">\
      <ul class="rule-list">\
        <li class="noderule-item" v-for="noderule in noderules" :id="noderule.UUID">\
          <noderule :noderule="noderule"></noderule>\
        </li>\
      </ul>\
    </div>\
  ',

  data: function() {
    return {
      noderules: {},
      intervalID: "",
    };
  },

  computed: {
    count: function() {
      return Object.keys(this.noderules).length;
    },
  },

  created: function() {
    var self = this;
    this.intervalID = setInterval(this.getNodeRuleList.bind(this), 30000);
    this.getNodeRuleList();
    app.$on("refresh-noderule-list", function() {
      self.getNodeRuleList();
    });
  },

  destroyed: function() {
    clearInterval(this.intervalID);
  },

  methods: {
    getNodeRuleList: function() {
      var self = this;
      self.noderuleAPI.list()
        .then(function(data) {
          self.noderules = data;
        })
        .catch(function(e) {
          self.$error({message: 'Node rule list error: ' + e.responseText});
          return e;
        });
    },
  },
});
