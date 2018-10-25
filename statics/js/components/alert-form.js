/* jshint multistr: true */

Vue.component('alert-form', {

  mixins: [apiMixin, notificationMixin],

  template: '\
    <div>\
      <form @submit.prevent="start" class="alert-form" v-if="visible">\
        <div class="form-group">\
          <label for="alert-name">Name</label>\
          <input id="alert-name" type="text" class="form-control input-sm" v-model="name" />\
        </div>\
        <div class="form-group">\
          <label for="alert-desc">Description</label>\
          <textarea id="alert-desc" type="text" class="form-control input-sm" rows="2" v-model="desc"></textarea>\
        </div>\
        <div class="form-group">\
          <label for="alert-expr">Expression</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Gremlin or JavaScript expression evaluated to trigger the alarm"></i></a>\
          <textarea id="alert-expr" type="text" class="form-control input-sm" rows="3" v-model="expr"></textarea>\
        </div>\
        <div class="form-group">\
          <label for="alert-trigger">Trigger</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Event that triggers the alert evaluation"></i></a></br>\
          <label class="radio-inline">\
            <input type="radio" id="graph" name="trigger" value="graph" v-model="trigger"> Graph\
            <span class="checkmark"></span>\
          </label>\
          <label class="radio-inline">\
            <input type="radio" id="periodic" name="trigger" value="periodic" v-model="trigger"> Periodic\
            <span class="checkmark"></span>\
          </label>\
        </div>\
        <div class="form-group" v-if="trigger === \'periodic\'">\
          <label for="alert-duration">Duration</label>\
          <input id="alert-duration" type="text" class="form-control input-sm" v-model="duration" />\
        </div>\
        <div class="form-group">\
          <label for="alert-action">Action</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="URL to trigger. Can be a webhook or a local file(file://)"></i></a>\
          <input id="alert-action" type="text" class="form-control input-sm" v-model="action" />\
        </div>\
        <button type="submit" id="start-alert" class="btn btn-primary">Start</button>\
        <button type="button" class="btn btn-danger" @click="reset">Cancel</button>\
      </form>\
      <button type="button"\
              id="create-alert"\
              class="btn btn-primary"\
              v-else\
              @click="visible = !visible">\
        Create\
      </button>\
    </div>\
  ',

  data: function() {
    return {
      visible: false,
      name: "",
      desc: "",
      expr: "",
      trigger: "graph",
      action: "",
      duration: "5s",
    };
  },

  methods: {
    reset: function() {
      this.visible = false;
      this.name = this.desc = this.expr = this.action = "";
      this.trigger = "graph";
      this.duration = "5s";
    },

    start: function() {
      var self = this;
      if (this.expr === "") {
        this.$error({message: "Expression is mandatory"});
        return;
      }
      if (this.trigger === "periodic") {
        if (this.duration === "") {
          this.$error({message: "Duration is mandatory for Periodic trigger"});
          return;
        }
        this.trigger = "duration:" + this.duration;
      }

      var alert = new api.Alert();
      alert.Name = this.name;
      alert.Description = this.desc;
      alert.Expression = this.expr;
      alert.Trigger = this.trigger;
      alert.Action = this.action;
      return self.alertAPI.create(alert)
        .then(function(data) {
            self.$success({message: 'Alert created'});
            self.reset();
            app.$emit("refresh-alert-list");
            return data;
          })
          .catch(function(e) {
            self.$error({message: 'Alert create error: ' + e.responseText});
            return e;
          });
    }
  }
});
