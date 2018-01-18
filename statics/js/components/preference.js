/* jshint multistr: true */

var PreferenceComponent = {

  name: 'Preference',

  mixins: [apiMixin, notificationMixin],

  template: '\
    <form class="form-preference" @submit.prevent="save">\
      <div class="form-group">\
        <label for="default-filter">Filters</label>\
        <a><i class="fa fa-question help-text" aria-hidden="true" title="filter and highlight gremlin queries, displayed in the left panel"></i></a>\
        <div v-for="filter in filters">\
          <div class="form-group">\
            <div class="input-group">\
              <label class="input-group-addon">Name: </label>\
              <input class="form-control" v-model="filter.name"/>\
              <span class="input-group-btn">\
                <button class="btn btn-danger" type="button" @click="removeFilter(filter)" title="delete filter">\
                  <i class="fa fa-trash-o" aria-hidden="true"></i>\
                </button>\
              </span>\
            </div>\
            <div class="input-group filter-field">\
              <label class="input-group-addon">Filter:  </label>\
              <input class="form-control" v-model="filter.filter"/>\
            </div>\
          </div>\
        </div>\
        <button class="btn btn-primary btn-round-xs btn-xs" type="button" @click="addFilter" title="add new filter">+</button>\
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
    var filters = [{name:"", filter:""}];
    
    f = JSON.parse(localStorage.getItem("filters"));
    if (f && f.length > 0) filters = f;

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
      filters: filters,
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
      localStorage.setItem("filters", JSON.stringify(this.filterEmpty(this.filters)));
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

    addFilter: function() {
      this.filters.push({name: "", filter: ""});
    },

    removeFilter: function(filter) {
      i = this.filters.indexOf(filter);
      this.filters.splice(i, 1);
    },

    filterEmpty: function(list) {
      var newList = [];
      $.each(list, function(i, f) {
        if (f.name !== "" && f.filter !== "") newList.push(f);
      });
      return newList;
    }
  }

};
