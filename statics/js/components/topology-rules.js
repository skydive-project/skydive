/* jshint multistr: true */

Vue.component('topology-rules', {
  template: '\
    <div>\
      <div class="form-group">\
        <label class="radio-inline">\
          <input type="radio" id="node" value="node" v-model="rule"> Node Rules\
          <span class="checkmark"></span>\
        </label>\
        <label class="radio-inline">\
          <input type="radio" id="edge" value="edge" v-model="rule"> Edge Rules\
          <span class="checkmark"></span>\
        </label>\
      </div>\
      <div v-if="rule === \'node\'">\
        <noderule-list></noderule-list>\
        <noderule-form></noderule-form>\
      </div>\
      <div v-if="rule === \'edge\'">\
        <edgerule-list></edgerule-list>\
        <edgerule-form></edgerule-form>\
      </div>\
    </div>\
  ',

  data: function() {
    return {
      rule: "node",
    };
  },
});
