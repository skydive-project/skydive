Vue.component('metrics-table', {

  props: {

    object: {
      type: Object,
      required: true
    },

    keys: {
      type: Array,
      required: true
    },

    defaultKeys: {
      type: Array,
      default: ['Last', 'RxBytes', 'RxPackets', 'TxBytes', 'TxPackets']
    }
  },

  template: '\
    <dynamic-table :rows="rows"\
                   :fields="fields"\
                   @toggleField="toggleField">\
    </dynamic-table>\
  ',

  data: function() {
    return {
      fields: []
    };
  },

  created: function() {
    this.generateFields();
  },

  computed: {

    rows: function() {
      return [this.object];
    },

  },

  methods: {

    isTime: function(field) {
      return ['Start', 'Last'].indexOf(field.label) !== -1;
    },

    toggleField: function(field) {
      field.show = !field.show;
    },

    generateFields: function() {
      for (var i in this.keys) {
        var key = this.keys[i];
        var f = {
          name: [key],
          label: key,
          show: this.defaultKeys.indexOf(key) !== -1,
        };

        if (this.isTime(f) && f.show) {
          this.fields.splice(0, 0, f);
        } else {
          this.fields.push(f);
        }
      }
    },

  },


});

Vue.component('blockdev-metrics-table', {

  props: {

    object: {
      type: Object,
      required: true
    },

    keys: {
      type: Array,
      required: true
    },

    defaultKeys: {
      type: Array,
      default: ['Last','ReadsPerSec','WritesPerSec','ReadsKBPerSec','WritesKBPerSec','ReadsMergedPerSec','WritesMergedPerSec']
    }
  },

  template: '\
    <dynamic-table :rows="rows"\
                   :fields="fields"\
                   @toggleField="toggleField">\
    </dynamic-table>\
  ',

  data: function() {
    return {
      fields: []
    };
  },

  created: function() {
    this.generateFields();
  },

  computed: {

    rows: function() {
      return [this.object];
    },

  },

  methods: {

    isTime: function(field) {
      return ['Start', 'Last'].indexOf(field.label) !== -1;
    },

    toggleField: function(field) {
      field.show = !field.show;
    },

    generateFields: function() {
      for (var i in this.keys) {
        var key = this.keys[i];
        var f = {
          name: [key],
          label: key,
          show: this.defaultKeys.indexOf(key) !== -1,
        };

        if (this.isTime(f) && f.show) {
          this.fields.splice(0, 0, f);
        } else {
          this.fields.push(f);
        }
      }
    },

  },


});

Vue.component('sflow-metrics-table', {

  props: {

    object: {
      type: Object,
      required: true
    },

    keys: {
      type: Array,
      required: true
    },

    defaultKeys: {
      type: Array,
      default: ['Last', 'IfInUcastPkts', 'IfOutUcastPkts', 'IfInOctets', 'IfOutOctets', 'IfInDiscards', 'OvsdpNHit', 'OvsdpNMissed', 'OvsdpNMaskHit']
    }
  },

  template: '\
    <dynamic-table :rows="rows"\
                   :fields="fields"\
                   @toggleField="toggleField">\
    </dynamic-table>\
  ',

  data: function() {
    return {
      fields: []
    };
  },

  created: function() {
    this.generateFields();
  },

  computed: {

    rows: function() {
      return [this.object];
    },

  },

  methods: {

    isTime: function(field) {
      return ['Start', 'Last'].indexOf(field.label) !== -1;
    },

    toggleField: function(field) {
      field.show = !field.show;
    },

    generateFields: function() {
      for (var i in this.keys) {
        var key = this.keys[i];
        var f = {
          name: [key],
          label: key,
          show: this.defaultKeys.indexOf(key) !== -1,
        };

        if (this.isTime(f) && f.show) {
          this.fields.splice(0, 0, f);
        } else {
          this.fields.push(f);
        }
      }
    },

  },


});