/* jshint multistr: true */

var PreferenceComponent = {

  name: 'Preference',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <form class="form-preference" @submit.prevent="save">\
      <div class="form-group">\
        <label for="default-favorite">Favorite Gremlin Expressions</label>\
        <a><i class="fa fa-question help-text" aria-hidden="true" title="Filter and highlight gremlin queries, displayed in the left panel"></i></a>\
        <div v-for="favorite in favorites">\
          <div class="form-group">\
            <div class="input-group">\
              <label class="input-group-addon">Name: </label>\
              <input class="form-control" v-model="favorite.name"/>\
              <span class="input-group-btn">\
                <button class="btn btn-danger" type="button" @click="removeFavorite(favorite)" title="Delete favorite expression">\
                  <i class="fa fa-trash-o" aria-hidden="true"></i>\
                </button>\
              </span>\
            </div>\
            <div class="input-group favorite-field">\
              <label class="input-group-addon">Filter:  </label>\
              <input class="form-control" v-model="favorite.expression"/>\
            </div>\
          </div>\
        </div>\
        <button class="btn btn-primary btn-round-xs btn-xs" type="button" @click="addFavorite" title="Add new favorite expression">+</button>\
      </div>\
      <div class="form-group">\
        <label for="bw-threshold">Bandwidth Threshold</label>\
        <a><i class="fa fa-question help-text" aria-hidden="true" title="bandwidth threshold mode (relative/absolute)"></i></a>\
        <select id="be-threshold" v-model="bwThreshold" class="form-control input-sm">\
          <option value="absolute">Absolute</option>\
          <option value="relative">Relative</option>\
        </select>\
      </div>\
      <div v-if="bwThreshold == \'absolute\'">\
        <div class="form-group">\
          <label for="bw-abs-active">Bandwidth Absolute Active</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Active threshold in kbps"></i></a>\
          <input id="bw-abs-active" type="number" class="form-control input-sm" v-model="bwAbsActive" min="0"/>\
        </div>\
        <div class="form-group">\
          <label for="bw-abs-warning">Bandwidth Absolute Warning</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Warning threshold in kbps"></i></a>\
          <input id="bw-abs-warning" type="number" class="form-control input-sm" v-model="bwAbsWarning" min="0"/>\
        </div>\
        <div class="form-group">\
          <label for="bw-abs-alert">Bandwidth Absolute Alert</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Alert threshold in kbps"></i></a>\
          <input id="bw-abs-alert" type="number" class="form-control input-sm" v-model="bwAbsAlert" min="0"/>\
        </div>\
      </div>\
      <div v-if="bwThreshold == \'relative\'">\
        <div class="form-group">\
          <label for="bw-rel-active">Bandwidth Relative Active</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Active threshold in between 0 to 1"></i></a>\
          <input id="bw-rel-active" type="number" class="form-control input-sm" v-model="bwRelActive" min="0" max="1" step="0.1"/>\
        </div>\
        <div class="form-group">\
          <label for="bw-rel-warning">Bandwidth Relative Warning</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Warning threshold in between 0 to 1"></i></a>\
          <input id="bw-rel-warning" type="number" class="form-control input-sm" v-model="bwRelWarning" min="0" max="1" step="0.1"/>\
        </div>\
        <div class="form-group">\
          <label for="bw-rel-alert">Bandwidth Relative Alert</label>\
          <a><i class="fa fa-question help-text" aria-hidden="true" title="Alert threshold in between 0 to 1"></i></a>\
          <input id="bw-rel-alert" type="number" class="form-control input-sm" v-model="bwRelAlert" min="0" max="1" step="0.1"/>\
        </div>\
      </div>\
      <div class="button-holder row">\
        <button class="btn btn-lg btn-primary" type="submit">Save</button>\
        <button class="btn btn-lg btn-danger" type="button" @click="cancel"> Cancel</button>\
      </div>\
    </form>\
  ',

  data: function() {
    var self = this;
    var favorites = [{name:"", expression:""}];

    f = JSON.parse(localStorage.getItem("favorites"));
    if (f && f.length > 0) favorites = f;

    var bwt = "relative";
    if (localStorage.bandwidthThreshold) {
      bwt = localStorage.bandwidthThreshold;
    } else {
      $.when(this.$getConfigValue('analyzer.bandwidth_threshold')).
        then(function(value) {
          self.bwThreshold = value;
      });
    }

    return {
      favorites: favorites,
      bwThreshold: bwt,
      bwAbsActive: localStorage.bandwidthAbsoluteActive,
      bwAbsWarning: localStorage.bandwidthAbsoluteWarning,
      bwAbsAlert: localStorage.bandwidthAbsoluteAlert,
      bwRelActive: localStorage.bandwidthRelativeActive,
      bwRelWarning: localStorage.bandwidthRelativeWarning,
      bwRelAlert: localStorage.bandwidthRelativeAlert,
      bwUpdatePeriod: localStorage.bandwidthUpdatePeriod,
    };
  },

  methods: {

    save: function() {
      if (this.bwThreshold !== "" && this.bwThreshold !== "absolute" && this.bwThreshold !== "relative") {
        this.$error({message: 'Invalid value for bandwidth threshold. allowed values \'absolute\', \'relative\''});
        return;
      }
      localStorage.setItem("favorites", JSON.stringify(this.filterEmpty(this.favorites)));
      localStorage.setItem("bandwidthThreshold", this.bwThreshold);
      localStorage.setItem("bandwidthAbsoluteActive", this.bwAbsActive);
      localStorage.setItem("bandwidthAbsoluteWarning", this.bwAbsWarning);
      localStorage.setItem("bandwidthAbsoluteAlert", this.bwAbsAlert);
      localStorage.setItem("bandwidthRelativeActive", this.bwRelActive);
      localStorage.setItem("bandwidthRelativeWarning", this.bwRelWarning);
      localStorage.setItem("bandwidthRelativeAlert", this.bwRelAlert);
      localStorage.setItem("bandwidthUpdatePeriod", this.bwUpdatePeriod);
      this.$success({message: 'Preferences Saved'});
      this.$router.push("/topology");
    },

    cancel: function() {
      this.$error({message: 'Modifications not saved'});
      this.$router.go(-1);
    },

    addFavorite: function() {
      this.favorites.push({name: "", expression: ""});
    },

    removeFavorite: function(favorite) {
      i = this.favorites.indexOf(favorite);
      this.favorites.splice(i, 1);
    },

    filterEmpty: function(list) {
      var newList = [];
      $.each(list, function(i, f) {
        if (f.name !== "" && f.expression !== "") newList.push(f);
      });
      return newList;
    }
  }

};
