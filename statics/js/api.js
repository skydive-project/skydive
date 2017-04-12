var apiMixin = {

  methods: {

    $topologyQuery: function(gremlinQuery, cntx) {
      return $.ajax({
        dataType: "json",
        url: '/api/topology',
        data: JSON.stringify({"GremlinQuery": gremlinQuery}),
        contentType: "application/json; charset=utf-8",
        method: 'POST',
        context: cntx,
      })
      .then(function(data) {
        if (data === null)
          return [];
        // Result can be [Node] or [[Node, Node]]
        if (data.length > 0 && data[0] instanceof Array)
          data = data[0];
        return data;
      });
    },

    $captureList: function() {
      return $.ajax({
        dataType: "json",
        url: '/api/capture',
        contentType: "application/json; charset=utf-8",
        method: 'GET',
      })
      .fail(function(e) {
        self.$error({message: 'Capture list error: ' + e.responseText});
        return e;
      });
    },

    $captureCreate: function(query, name, description, bpf) {
      var self = this;
      return $.ajax({
        dataType: "json",
        url: '/api/capture',
        data: JSON.stringify({GremlinQuery: query,
                              Name: name || null,
                              Description: description || null,
                              BPFFilter: bpf || null}),
        contentType: "application/json; charset=utf-8",
        method: 'POST',
      })
      .then(function(data) {
        self.$success({message: 'Capture created'});
        return data;
      })
      .fail(function(e) {
        self.$error({message: 'Capture create error: ' + e.responseText});
        return e;
      });
    },

    $captureDelete: function(uuid) {
      var self = this;
      return $.ajax({
        dataType: 'text',
        url: '/api/capture/' + uuid + '/',
        method: 'DELETE',
      })
      .fail(function(e) {
        self.$error({message: 'Capture delete error: ' + e.responseText});
        return e;
      });
    },

    $getConfigValue: function(key) {
      return $.ajax({
        dataType: 'json',
        url: "/api/config/" + key,
        contentType: "application/json; charset=utf-8",
        method: 'GET',
      });
    }
  }

};
