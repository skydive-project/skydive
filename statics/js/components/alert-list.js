/* jshint multistr: true */

var Alert = {

  mixins: [apiMixin, notificationMixin],

  props: {
    alert: {
      type: Object,
      required: true,
    }
  },

  template: '\
    <div class="alert-item">\
      <div class="alert-title">\
        <i class="alert-action alert-delete fa fa-trash"\
          @click="remove(alert)">\
        </i>\
        {{alert.UUID}}\
      </div>\
      <dl class="dl-horizontal">\
        <dt v-if="alert.Name">Name</dt>\
        <dd v-if="alert.Name">{{alert.Name}}</dd>\
        <dt v-if="alert.Description">Description</dt>\
        <dd v-if="alert.Description">{{alert.Description}}</dd>\
        <dt>Expression</dt>\
        <dd>{{alert.Expression}}</dd>\
        <dt v-if="alert.Action">Action</dt>\
        <dd v-if="alert.Action">{{alert.Action}}</dd>\
        <dt>Trigger</dt>\
        <dd>{{alert.Trigger}}</dd>\
        <dt>CreateTime</dt>\
        <dd>{{alert.CreateTime}}</dd>\
      </dl>\
    </div>\
  ',

  methods: {
    remove: function(alert) {
      this.$alertDelete(alert.UUID)
        .always(function() {
          app.$emit("refresh-alert-list");
        });
    },
  },
};

Vue.component('alert-list', {

  mixins: [apiMixin, notificationMixin],

  components: {
    'alert': Alert,
  },

  template: '\
    <div class="sub-panel" v-if="count > 0">\
      <ul class="alert-list">\
        <li class="alert-item" v-for="alert in alerts" :id="alert.UUID">\
          <alert :alert="alert"></alert>\
        </li>\
      </ul>\
    </div>\
  ',

  data: function() {
    return {
      alerts: {},
    };
  },

  computed: {
    count: function() {
      return Object.keys(this.alerts).length;
    },
  },

  created: function() {
    var self = this;
    setInterval(this.getAlertList.bind(this), 30000);
    this.getAlertList();
    app.$on("refresh-alert-list", function() {
      self.getAlertList();
    });
  },

  methods: {
    getAlertList: function() {
      var self = this;
      this.$alertList()
        .then(function(data) {
          self.alerts = data;
        });
    },
  },

});
