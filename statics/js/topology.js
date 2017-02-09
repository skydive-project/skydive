var TopologyAPI = {

    query: function(gremlinQuery) {
      return $.ajax({
        dataType: "json",
        url: '/api/topology',
        data: JSON.stringify({"GremlinQuery": gremlinQuery}),
        contentType: "application/json; charset=utf-8",
        method: 'POST',
      })
      .then(function(data) {
        if (data === null)
          return [];
        // Result can be [Node] or [[Node, Node]]
        if (data.length > 0 && data[0] instanceof Array)
          data = data[0];
        return data;
      });
    }
};
