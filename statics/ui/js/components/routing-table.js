/* jshint multistr: true */

Vue.component('routing-table', {

  props: {

    rt: {
      type: Object,
      required: true
    },

  },

  template: '\
    <dynamic-table :rows="rows"\
                   :fields="fields"\
                   :sortOrder="sortOrder"\
                   :sortBy="sortBy"\
                   @order="order"\
                   @toggleField="toggleField">\
      <template slot="actions">\
        <div class="dynmic-table-actions-item">\
          <input v-model="filter" placeholder="Filter..." class="form-control input-sm input-xs"></input>\
        </div>\
      </template>\
    </dynamic-table>\
  ',

  data: function() {
    return {
      filter: "",
      sortBy: null,
      sortOrder: 1,
      fields: [
        {
          name: ['prefix'],
          label: 'Prefix',
          show: true,
        },
        {
          name: ['nhs'],
          label: 'Nexthops',
          show: true,
        },
        {
          name: ['protocol'],
          label: 'Protocol',
          show: true,
        },
      ]
    };
  },

  created: function() {
    // sort by prefix by default
    this.sortBy = this.fields[0].name;
  },

  computed: {

    routes: function() {
      return this.rt.Routes.reduce(function(routes, routeObj) {
        var route = {
          prefix: routeObj.Prefix || "",
          protocol: routeObj.Protocol || "",
        };
        if ('NextHops' in routeObj) {
          route.nhs = routeObj.NextHops.reduce(function(nhs, nhObj) {
            if ('IP' in nhObj)
              nhs.push(nhObj.IP);
            else
              nhs.push("IfIndex: " + nhObj.IfIndex);
            return nhs;
          }, []).join(", ");
        }
        routes.push(route);
        return routes;
      }, []);
    },

    ipv4Routes: function() {
      return this.routes.filter(function(r) {
        return r.prefix.indexOf('.') !== -1;
      });
    },

    ipv6Routes: function() {
      return this.routes.filter(function(r) {
        return r.prefix.indexOf(':') !== -1;
      });
    },

    sortedRoutes: function() {
      return this.ipv4Routes.sort(this.sortIPsV4).concat(this.ipv6Routes.sort(this.sortIPsV6));
    },

    filteredRoutes: function() {
      if (this.filter !== "") {
        return this.sortedRoutes.filter(function(row) {
          return row.prefix.match(this.filter) !== null;
        }, this);
      }
      return this.sortedRoutes;
    },

    rows: function() {
      return this.filteredRoutes;
    },

  },

  methods: {

    sortIPsV6: function(r1, r2) {
      if (r1[this.sortBy] < r2[this.sortBy])
        return 1 * this.sortOrder;
      if (r1[this.sortBy] > r2[this.sortBy])
        return -1 * this.sortOrder;
    },

    ipV4toInt: function(ip) {
      return ip.split('/')[0].split('.').reduce(function(ipInt, octet) {
        return (parseInt(ipInt)<<8) + parseInt(octet, 10);
      }, 0) >>> 0;
    },

    sortIPsV4: function(r1, r2) {
      if (this.ipV4toInt(r1[this.sortBy]) > this.ipV4toInt(r2[this.sortBy]))
        return 1 * this.sortOrder;
      if (this.ipV4toInt(r1[this.sortBy]) < this.ipV4toInt(r2[this.sortBy]))
        return -1 * this.sortOrder;
    },

    order: function(sortOrder) {
      this.sortOrder = sortOrder;
    },

    toggleField: function(field) {
      field.show = !field.show;
    },

  },

});
