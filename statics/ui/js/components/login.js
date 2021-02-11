/* jshint multistr: true */

var LoginComponent = {

  name: 'Login',

  mixins: [notificationMixin],

  data: function() {
    return {
      "username": ""
    };
  },

  template: '\
      <form class="form-signin" @submit.prevent="login">\
        <h2 class="form-signin-heading">Please sign in</h2>\
          <div class="form-signin-fields">\
            <label for="login" class="sr-only">Login</label>\
            <input id="username" type="text" name="username" class="form-control" v-model="username" placeholder="Login" required autofocus>\
            <label for="password" class="sr-only">Password</label>\
            <input id="password" type="password" name="password" class="form-control" placeholder="Password" required>\
          </div>\
        <button id="signin" class="btn btn-lg btn-primary btn-block" type="submit">Sign in</button>\
      </form>\
  ',

  methods: {

    login: function() {
      var self = this;
      $.ajax({
        url: '/login',
        data: $(this.$el).serialize(),
        method: 'POST',
      })
      .then(function(data) {
        self.$store.commit('login');
      });
    },

  }

};
