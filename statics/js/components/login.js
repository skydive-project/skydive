/* jshint multistr: true */

var LoginComponent = {

  name: 'Login',

  mixins: [notificationMixin],

  template: '\
    <form class="form-signin" @submit.prevent="login">\
      <h2 class="form-signin-heading">Please sign in</h2>\
      <label for="login" class="sr-only">Login</label>\
      <input type="text" name="username" class="form-control" placeholder="Login" required autofocus>\
      <label for="password" class="sr-only">Password</label>\
      <input type="password" name="password" class="form-control" placeholder="Password" required>\
      <button class="btn btn-lg btn-primary btn-block" type="submit">Sign in</button>\
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
