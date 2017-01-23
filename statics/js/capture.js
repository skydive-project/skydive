var CaptureAPI = {

  list: function() {
    return $.ajax({
      dataType: "json",
      url: '/api/capture',
      contentType: "application/json; charset=utf-8",
      method: 'GET',
    })
    .fail(function(e) {
        $.notify({
          message: 'Capture list error: ' + e.responseText
        }, {
          type: 'danger'
        });
    });
  },

  create: function(query, name, description) {
    return $.ajax({
      dataType: "json",
      url: '/api/capture',
      data: JSON.stringify({"GremlinQuery": query, 
                            "Name": name || null,
                            "Description": description || null}),
      contentType: "application/json; charset=utf-8",
      method: 'POST',
    })
    .then(function(data) {
      $.notify({
        message: 'Capture created'
      },{
        type: 'success'
      });
      return data;
    })
    .fail(function(e) {
      $.notify({
        message: 'Capture create error: ' + e.responseText
      },{
        type: 'danger'
      });
    });
  },

  delete: function(uuid) {
    return $.ajax({
      dataType: 'text',
      url: '/api/capture/' + uuid + '/',
      method: 'DELETE',
    })
    .fail(function(e) {
      $.notify({
        message: 'Capture delete error: ' + e.responseText
      },{
        type: 'danger'
      });
    });
  }

};
