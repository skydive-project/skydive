/* jshint multistr: true */

var StatusComponent = {

  name: 'Status',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div class="status statustab">\
      <div class="panel panel-content">\
        <h1>Services Status</h1>\
        <object-detail :object="status"></object-detail>\
      </div>\
    </div>\
  ',

  data: function() {
    return {
      status: null,
    };
  },

  created: function() {
    this.getStatus();
  },

  methods: {
    getStatus: function() {
      var self = this;
      $.ajax({
        dataType: "json",
        url: '/api/status',
        contentType: "application/json; charset=utf-8",
        method: 'GET',
      })
      .then(function(data) {
        self.status = data;
      })
      .fail(function(e) {
        self.$error({message: 'Not able to get status, error: ' + e.responseText});
      });
    },
  }
};
