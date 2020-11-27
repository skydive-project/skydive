var websocket = new WSHandler();

var store = new Vuex.Store({

  state: {
    config: uiConfig,
    connected: null,
    logged: null,
    permissions: getPermissions(),
    service: null,
    version: null,
    history: null,
    currentNode: null,
    currentEdge: null,
    emphasizedIDs: [],
    highlightedNodes: [],
    highlightInprogress: new Map(),
    notifications: [],
    topologyFilter: "",
    topologyHighlight: "",
    topologyTimeContext: 0,
  },

  mutations: {

    history: function(state, support) {
      state.history = support;
    },

    topologyFilter: function(state, filter) {
      state.topologyFilter = filter;
    },

    topologyHighlight: function(state, filter) {
      state.topologyHighlight = filter;
    },

    topologyTimeContext: function(state, time) {
      state.topologyTimeContext = time;
    },

    login: function(state, data) {
      state.logged = true;
      state.permissions = getPermissions();
    },

    logout: function(state) {
      state.logged = false;
      state.permissions = [];
    },

    connected: function(state) {
      state.connected = true;
    },

    disconnected: function(state) {
      state.connected = false;
    },

    nodeSelected: function(state, node) {
      state.currentNode = node;
    },

    nodeUnselected: function(state) {
      state.currentNode = null;
    },

    edgeSelected: function(state, edge) {
      state.currentEdge = edge;
    },

    edgeUnselected: function(state) {
      state.currentEdge = null;
    },

    highlight: function(state, id) {
      if (state.highlightedNodes.indexOf(id) < 0) state.highlightedNodes.push(id);
    },

    unhighlight: function(state, id) {
      state.highlightedNodes = state.highlightedNodes.filter(function(_id) {
        return id !== _id;
      });
    },

    highlightStart: function(state, uuid) {
      state.highlightInprogress.set(uuid, true);
    },

    highlightEnd: function(state, uuid) {
      state.highlightInprogress.set(uuid, false);
    },

    highlightDelete: function(state, uuid) {
      state.highlightInprogress.delete(uuid);
    },

    emphasize: function(state, id) {
      if (state.emphasizedIDs.indexOf(id) < 0) state.emphasizedIDs.push(id);
    },

    deemphasize: function(state, id) {
      state.emphasizedIDs = state.emphasizedIDs.filter(function(_id) {
        return id !== _id;
      });
    },

    service: function(state, service) {
      state.service = service.charAt(0).toUpperCase() + service.slice(1);
    },

    version: function(state, version) {
      state.version = version;
    },

    addNotification: function(state, notification) {
      if (state.notifications.length > 0 &&
          state.notifications.some(function(n) {
            return n.message === notification.message;
          })) {
        return;
      }
      state.notifications.push(notification);
    },

    removeNotification: function(state, notification) {
      state.notifications = state.notifications.filter(function(n) {
        return n !== notification;
      });
    },

  },

});

var routes = [
  { path: '/login', component: LoginComponent },
  { path: '/logout',
    component: {
      template: '<div></div>',
      created: function() {
        setCookie("authtoken", "", -1);
        setCookie("permissions", "", -1);
        websocket.disconnect();
        this.$store.commit('logout');
      }
    }
  },
  { path: '/topology', component: TopologyComponent, props: (route) => ({ query: route.query }) },
  { path: '/preference', component: PreferenceComponent },
  { path: '/status', component: StatusComponent },
  { path: '*', redirect: '/topology' }
];

var router = new VueRouter({
  mode: 'history',
  linkActiveClass: 'active',
  routes: routes
});

// if not logged, always route to /login
// if already logged don't route to /login
router.beforeEach(function(to, from, next) {
  if (store.state.logged === false && to.path !== '/login')
    next('/login');
  else if (store.state.logged === true && to.path == '/login')
    next(false);
  else
    next();
});

var app = new Vue({
  router: router,

  store: store,

  mixins: [notificationMixin, apiMixin],

  created: function() {
    var self = this;

    this.setThemeFromConfig();

    websocket.addConnectHandler(self.onConnected.bind(self));
    websocket.addDisconnectHandler(self.onDisconnected.bind(self));
    websocket.addErrorHandler(self.onError.bind(self));

    this.checkAPI();

    this.interval = null;

    // global handler to detect authorization errors
    $(document).ajaxError(function(evt, e) {
      switch (e.status) {
        case 401:
          self.$error({message: 'Authentication failed'});
          self.$store.commit('logout');
          break;
      }

      return e;
    });
  },

  computed: Vuex.mapState(['service', 'version', 'logged', 'connected']),

  watch: {

    logged: function(newVal) {
      var self = this;
      if (newVal === true) {
        this.checkAPI();
        router.push('/topology');
        websocket.connect();

        if (!this.interval)
          this.interval = setInterval(this.checkAPI, 5000);

        // check if the Analyzer supports history
        this.$topologyQuery("G.At('-1m').V().Limit(1)")
          .then(function() {
            self.$store.commit('history', true);
          })
          .catch(function() {
            self.$store.commit('history', false);
          });
      } else {
        if (this.interval) {
          clearInterval(self.interval);
          this.interval = null;
        }
        router.push('/login');
      }
    },
  },

  methods: {

    checkAPI: function() {
      var self = this;
      return $.ajax({
        dataType: "json",
        url: '/api',
      })
      .then(function(r) {
        if (!self.$store.state.logged)
          self.$store.commit('login');
        if (self.$store.state.service != r.Service)
          self.$store.commit('service', r.Service);
        if (self.$store.state.version != r.Version)
          self.$store.commit('version', r.Version);
        return r;
      });
    },

    onConnected: function() {
      this.$store.commit('connected');
      this.$success({message: 'Connected'});
    },

    onDisconnected: function() {
      this.$store.commit('disconnected');
      this.$error({message: 'Disconnected'});

      if (this.$store.state.logged)
        setTimeout(function(){websocket.connect();}, 1000);
    },

    onError: function() {
      if (this.$store.state.connected)
        this.$store.commit('disconnected');

      setTimeout(function(){websocket.connect();}, 1000);
    },

    camelize: function(input) {
      return input.toLowerCase().replace(/_(.)/g, function(match, group1) {
        return group1.toUpperCase();
      });
    },

    getLocalValue: function(key) {
      if (!localStorage.preferences) return 0;
      var v = JSON.parse(localStorage.preferences)[key];
      if (!v || v === "0" || v === "null") return 0;
      if (isNaN(v)) return v;
      return Number(v);
    },

    getConfigValue: function(key) {
      var value = this.getLocalValue(this.camelize(key));
      if (!value) {
        var value = this.$store.state.config;
        var splitted = key.split(".");
        for (var s in splitted) {
          value = value[splitted[s]];
          if (value === undefined) {
            break;
          }
        }
      }
      return value;
    },

    enforce: function(object, action) {
      var perms = this.$store.state.permissions;
      for (var perm in perms) {
        if (perms[perm].Object === object && perms[perm].Action === action) {
          return perms[perm].Allowed;
        }
      }
      return false;
    },

    setThemeFromConfig: function() {
      this.setTheme(this.getConfigValue("theme"));
    },

    setTheme: function(theme) {
      switch (theme) {
        case 'light':
          $('body').addClass("light");
          $('body').removeClass("dark");

          $("#navbar").removeClass("navbar-inverse");
          $("#navbar").addClass("navbar-light");
          break;
        default:
          $('body').addClass("dark");
          $('body').removeClass("light");

          theme = 'dark';
          $("#navbar").addClass("navbar-inverse");
          $("#navbar").removeClass("navbar-light");
      }

      for (var i = 0; i < document.styleSheets.length; i++) {
        if (!document.styleSheets[i].href || document.styleSheets[i].href.search(/themes/) == -1) {
          continue;
        }
        if (document.styleSheets[i].href.search(theme) != -1) {
          document.styleSheets[i].disabled = false;
        } else {
          document.styleSheets[i].disabled = true;
        }
      }
    },
  }

});

$(document).ready(function() {
  Vue.config.devtools = true;

  Vue.use(VTooltip, {});
  Vue.component('datepicker', Datepicker);
  Vue.use(VueMarkdown, {});

  app.$mount('#app');

  app.setThemeFromConfig();
});
