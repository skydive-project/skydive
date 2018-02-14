var apiMixin = {

  methods: {

    // see : https://stackoverflow.com/questions/16086162/handle-file-download-from-ajax-post
    $downloadRawPackets: function(UUID, datastore){
      var self = this;

      var xhr = new XMLHttpRequest();
      xhr.open('POST', '/api/topology', true);
      xhr.responseType = 'arraybuffer';
      xhr.onload = function () {
        if (this.status === 404 && !datastore) {
          // retry as rawpacket can be in the datastore and not any more in
          // the agent
          return self.$downloadRawPackets(UUID, true);
        } else if (this.status === 200) {
          var filename = UUID + 'pcap';
          var type = xhr.getResponseHeader('Content-Type');

          var blob = new Blob([this.response], { type: type });
          if (typeof window.navigator.msSaveBlob !== 'undefined') {
            window.navigator.msSaveBlob(blob, filename);
          } else {
            var URL = window.URL || window.webkitURL;
            var downloadUrl = URL.createObjectURL(blob);

            var a;
            if (filename) {
              // use HTML5 a[download] attribute to specify filename
              a = document.createElement("a");
              // safari doesn't support this yet
              if (typeof a.download === 'undefined') {
                window.location = downloadUrl;
              } else {
                a.href = downloadUrl;
                a.download = filename;
                document.body.appendChild(a);
                a.click();
              }
            } else {
              window.location = downloadUrl;
            }

            setTimeout(function () {
              URL.revokeObjectURL(downloadUrl);
              if (a)
                document.body.removeChild(a);
              }, 100); // cleanup
          }
        }
      };
      xhr.setRequestHeader('Content-type', 'application/json; charset=utf-8');
      xhr.setRequestHeader('Accept', 'vnd.tcpdump.pcap');

      var query = 'G';
      if (datastore) {
        query += '.At("-1s")';
      }
      query += '.Flows().Has("UUID", "' + UUID + '").RawPackets()';
      xhr.send(JSON.stringify({"GremlinQuery": query}));
    },

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
      var self = this;
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

    $captureGet: function(id) {
      var self = this;
      return $.ajax({
        dataType: "json",
        url: '/api/capture/' + id,
        contentType: "application/json; charset=utf-8",
        method: 'GET',
      })
      .fail(function(e) {
        self.$error({message: 'Capture get error: ' + e.responseText});
        return e;
      });
    },

    $captureCreate: function(query, name, description, bpf, headerSize, rawPackets, tcpMetric, socketInfo, type, port) {
      var self = this;
      return $.ajax({
        dataType: "json",
        url: '/api/capture',
        data: JSON.stringify({GremlinQuery: query,
                              Name: name || null,
                              Description: description || null,
                              BPFFilter: bpf || null,
                              HeaderSize: headerSize || 0,
                              RawPacketLimit: rawPackets || 0,
                              ExtraTCPMetric: tcpMetric,
                              SocketInfo: socketInfo,
                              Type: type || null,
                              Port: port,
                             }),
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
    },

    $allowedTypes: function() {
      return ["ovsbridge", "device", "internal", "veth", "tun", "bridge", "dummy",
        "gre", "bond", "can", "hsr", "ifb", "macvlan", "macvtap", "vlan", "vxlan",
        "gretap", "ip6gretap", "geneve", "ipoib", "vcan", "ipip", "ipvlan", "lowpan",
        "ip6tnl", "ip6gre", "sit", "dpdkport", "ovsport"];
    }
  }

};
