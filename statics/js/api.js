var client = new api.Client();

var apiMixin = {

  created: function() {
    this.alertAPI = new api.API(client, "alert", api.Alert);
    this.captureAPI = new api.API(client, "capture", api.Capture);
    this.injectAPI = new api.API(client, "injectpacket", api.PacketInjection);
    this.workflowAPI = new api.API(client, "workflow", api.Workflow);
    this.gremlinAPI = new api.GremlinAPI(client);
    this.noderuleAPI = new api.API(client, "noderule", api.NodeRule);
    this.edgeruleAPI = new api.API(client, "edgerule", api.EdgeRule);
  },

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
          var filename = UUID + '.pcap';
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
      xhr.setRequestHeader('Accept', 'application/vnd.tcpdump.pcap');

      var query = 'G';
      if (datastore) {
        query += '.At("-1s")';
      }
      query += '.Flows().Has("UUID", "' + UUID + '").RawPackets()';
      xhr.send(JSON.stringify({"GremlinQuery": query}));
    },

    $topologyQuery: function(gremlinQuery, cntx) {
      return this.gremlinAPI.query(gremlinQuery);
    },

    $allowedTypes: function() {
      return ["ovsbridge", "device", "internal", "veth", "tun", "bridge", "dummy",
        "gre", "bond", "can", "hsr", "ifb", "macvlan", "macvtap", "vlan", "vxlan",
        "gretap", "ip6gretap", "geneve", "ipoib", "vcan", "ipip", "ipvlan", "lowpan",
        "ip6tnl", "ip6gre", "sit", "dpdkport", "ovsport"];
    }
  }

};
