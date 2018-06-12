
var Notification = {

  props: {

    notification: {
      type: Object,
      required: true
    },

  },

  template: '\
    <transition name="slide" mode="out-in">\
      <div class="alert" :class="css" role="alert">\
          <button type="button" aria-hidden="true" class="close" @click="close">×</button>\
          <span data-notify="title">{{notification.title}}</span>\
          <span data-notify="message">{{notification.message}}</span>\
      </div>\
    </transition>\
  ',

  mounted: function() {
    var self = this;
    if (self.notification.timeout === false)
      return;
    this.timeout = setTimeout(function() {
      self.$store.commit('removeNotification', self.notification);
    }, self.notification.delay || 2000);
  },

  computed: {

    css: function() {
      return "alert-" + this.notification.type;
    }

  },

  methods: {

    close: function() {
      if (this.timeout) {
        clearInterval(this.timeout);
      }
      this.$store.commit('removeNotification', this.notification);
    }

  }

};

Vue.component('notifications', {

  components: {
    notification: Notification,
  },

  template: '\
    <div class="col-xs-11 col-sm-4" style="position: fixed; z-index: 1031; top: 20px; right: 20px;">\
      <notification id="notification" v-for="n in notifications" :key="n.title" :notification="n"></notification>\
    </div>\
  ',

  computed: Vuex.mapState(['notifications']),

});

var notificationMixin = {

  methods: {

    $notify: function(options) {
      this.$store.commit('addNotification', Object.assign({
        type: 'info',
        title: '',
        timeout: true,
        delay: 2000,
      }, options));
    },

    $error: function(options) {
      this.$store.commit('addNotification', Object.assign({
        type: 'danger',
        title: '',
        timeout: true,
        delay: 3000,
      }, options));
    },

    $success: function(options) {
      this.$store.commit('addNotification', Object.assign({
        type: 'success',
        title: '',
        timeout: true,
        delay: 2000,
      }, options));
    },

  }

};
