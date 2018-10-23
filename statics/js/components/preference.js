/* jshint multistr: true */

var PreferenceComponent = {

  name: 'Preference',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <form class="form-preference" @submit.prevent="save">\
      <div class="form-group preference-field">\
        <label for="theme">Theme</label>\
        <select id="theme" v-model="preferences.theme" class="form-control custom-select">\
          <option value="dark">Dark</option>\
          <option value="light">Light</option>\
        </select>\
      </div>\
      <div class="form-group preference-field">\
        <label for="default-favorite">Favorite Gremlin Expressions</label>\
        <a><i class="fa fa-question help-text" aria-hidden="true" title="Filter and highlight gremlin queries, displayed in the left panel"></i></a>\
        <div v-for="favorite in preferences.favorites">\
          <div class="form-group">\
            <div class="input-group">\
              <label class="input-group-addon filter-title">Name: </label>\
              <input class="form-control filter-field" v-model="favorite.name"/>\
              <span class="input-group-btn">\
                <button class="btn btn-filter" type="button" @click="removeFavorite(favorite)" title="Delete favorite expression">\
                  <i class="fa fa-trash-o" aria-hidden="true"></i>\
                </button>\
              </span>\
            </div>\
            <div class="input-group filter-components">\
              <label class="input-group-addon filter-title">Filter:  </label>\
              <input class="form-control filter-field" v-model="favorite.expression"/>\
            </div>\
          </div>\
        </div>\
        <button class="btn btn-primary spl-btn" type="button" @click="addFavorite" title="Add new favorite expression">+ Add New Expression</button>\
      </div>\
      <div class="form-group preference-field">\
        <label for="bw-threshold">Bandwidth Threshold</label>\
        <a><i class="fa fa-question help-text" aria-hidden="true" title="bandwidth threshold mode (relative/absolute)"></i></a>\
        <select id="bw-threshold" v-model="preferences.bandwidthThreshold" class="form-control custom-select">\
          <option value="absolute">Absolute</option>\
          <option value="relative">Relative</option>\
        </select>\
      </div>\
      <div v-if="preferences.bandwidthThreshold == \'absolute\'">\
        <div class="form-group preference-field">\
          <label for="bw-abs-active">Bandwidth Absolute Active</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Active threshold in kbps"></i></a>\
          <input id="bw-abs-active" type="number" class="form-control input-sm" v-model.number="preferences.bandwidthAbsoluteActive" min="0"/>\
        </div>\
        <div class="form-group preference-field">\
          <label for="bw-abs-warning">Bandwidth Absolute Warning</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Warning threshold in kbps"></i></a>\
          <input id="bw-abs-warning" type="number" class="form-control input-sm" v-model.number="preferences.bandwidthAbsoluteWarning" min="0"/>\
        </div>\
        <div class="form-group preference-field">\
          <label for="bw-abs-alert">Bandwidth Absolute Alert</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Alert threshold in kbps"></i></a>\
          <input id="bw-abs-alert" type="number" class="form-control input-sm" v-model.number="preferences.bandwidthAbsoluteAlert" min="0"/>\
        </div>\
      </div>\
      <div v-if="preferences.bandwidthThreshold == \'relative\'">\
        <div class="form-group preference-field">\
          <label for="bw-rel-active">Bandwidth Relative Active</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Active threshold in between 0 to 1"></i></a>\
          <input id="bw-rel-active" type="number" class="form-control input-sm" v-model.number="preferences.bandwidthRelativeActive" min="0" max="1" step="0.1"/>\
        </div>\
        <div class="form-group preference-field">\
          <label for="bw-rel-warning">Bandwidth Relative Warning</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Warning threshold in between 0 to 1"></i></a>\
          <input id="bw-rel-warning" type="number" class="form-control input-sm" v-model.number="preferences.bandwidthRelativeWarning" min="0" max="1" step="0.1"/>\
        </div>\
        <div class="form-group preference-field">\
          <label for="bw-rel-alert">Bandwidth Relative Alert</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Alert threshold in between 0 to 1"></i></a>\
          <input id="bw-rel-alert" type="number" class="form-control input-sm" v-model.number="preferences.bandwidthRelativeAlert" min="0" max="1" step="0.1"/>\
        </div>\
      </div>\
      <div class="form-group preference-field">\
        <label for="bpf-favorite">Favorite BPF Filters</label>\
        <a><i class="fa fa-question help-text" aria-hidden="true" title="BPF filters, listed in the capture form"></i></a>\
        <div v-for="f in preferences.bpf">\
          <div class="form-group">\
            <div class="input-group">\
              <label class="input-group-addon filter-title">Name: </label>\
              <input class="form-control filter-field" v-model="f.name"/>\
              <span class="input-group-btn">\
                <button class="btn btn-filter" type="button" @click="removeBPF(f)" title="Delete BPF filter">\
                  <i class="fa fa-trash-o" aria-hidden="true"></i>\
                </button>\
              </span>\
            </div>\
            <div class="input-group filter-components">\
              <label class="input-group-addon filter-title">Filter:  </label>\
              <input class="form-control filter-field" v-model="f.expression"/>\
            </div>\
          </div>\
        </div>\
        <button class="btn btn-primary spl-btn" type="button" @click="addBPF" title="Add new BPF">+ Add New BPF</button>\
      </div>\
      <hr>\
      <div class="button-holder row">\
        <button class="btn btn-lg btn-primary spl-btn" type="button" @click="saveToFile" title="download preferences to local file"> Export</button>\
        <input type="file" @change="openfile($event)" id="file_selector" style="display:none;">\
        <button class="btn btn-lg btn-primary btn-group-pref spl-btn" type="button" @click="uploadFile" title="upload preferences from local file"> Import</button>\
        <button class="btn btn-lg btn-primary" style="float: right" type="submit" title="save in local storage">Save</button>\
        <button class="btn btn-lg btn-danger" style="float: right" type="button" @click="cancel"> Cancel</button>\
      </div>\
    </form>\
  ',

  data: function() {
    var self = this;
    var p = {};
    if (localStorage.preferences) p = JSON.parse(localStorage.preferences);
    if (!p.favorites || p.favorites.length <= 0) p.favorites = [{name:"", expression:""}];
    if (!p.bpf || p.bpf.length <= 0) p.bpf = [{name:"", expression:""}];

    return {
      preferences: p,
    };
  },

  methods: {

    save: function() {
      this.preferences.favorites = this.filterEmpty(this.preferences.favorites);
      this.preferences.bpf = this.filterEmpty(this.preferences.bpf);
      localStorage.setItem("preferences", JSON.stringify(this.preferences));
      this.$success({message: 'Preferences Saved'});
      this.$router.push("/topology");
      app.setThemeFromConfig();
    },

    cancel: function() {
      if (confirm("Do you really want to cancel?. May loose unsaved data!")) {
        this.$error({message: 'Modifications not saved'});
        this.$router.go(-1);
      }
    },

    addFavorite: function() {
      this.preferences.favorites.push({name: "", expression: ""});
    },

    removeFavorite: function(favorite) {
      i = this.preferences.favorites.indexOf(favorite);
      this.preferences.favorites.splice(i, 1);
    },

    addBPF: function() {
      this.preferences.bpf.push({name: "", expression: ""});
    },

    removeBPF: function(filter) {
      i = this.preferences.bpf.indexOf(filter);
      this.preferences.bpf.splice(i, 1);
    },

    filterEmpty: function(list) {
      var newList = [];
      $.each(list, function(i, f) {
        if (f.name !== "" && f.expression !== "") newList.push(f);
      });
      return newList;
    },

    saveToFile: function() {
      var a = document.createElement('a'), url = URL.createObjectURL(new Blob([localStorage.preferences], {type: 'plain/text'}));
      a.href = url;
      a.download = "skydive_preferences.txt";
      document.body.appendChild(a);
      a.click();
      setTimeout(function() {
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
      }, 10);
    },

    openfile: function(e) {
      var self = this;
      var reader = new FileReader();
      reader.onload = function() {
        var obj = JSON.parse(reader.result);
        self.preferences = obj;
	if (!self.preferences.favorites || self.preferences.favorites.length <= 0) self.preferences.favorites = [{name: "", expression: ""}];
	if (!self.preferences.bpf || self.preferences.bpf.length <= 0) self.preferences.bpf = [{name: "", expression: ""}];
      };
      reader.readAsText(e.target.files[0]);
      e.target.value = "";
    },

    uploadFile: function() {
      document.getElementById('file_selector').click();
    },

  }

};
