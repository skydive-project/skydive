---
date: 2016-09-29T11:02:01+02:00
title: Rest API
---

## Topology/Flow request

```console
POST /api/topology HTTP/1.1
Content-Type: application/json

{
  "GremlinQuery":"G.V()"
}
```

```console
HTTP/1.1 200 OK
Content-Type: application/json; charset=UTF-8

[
  {
    "Host": "localhost.localdomain",
    "ID": "d6759df3-d4e0-408b-64d3-c82ea6c9aeda",
    "Metadata": {
      "Name": "vm2",
      "Path": "/var/run/netns/vm2",
      "TID": "7daa39fe-92f7-5f9b-51b1-1dddcd41785c",
      "Type": "netns"
    }
  }
]
```

```console
POST /api/topology HTTP/1.1
Content-Type: application/json

{
  "GremlinQuery":"G.Flows().Limit(1)"
}
```

```console
[
  {
    "ANodeTID": "d9d6f8cf-4aa6-4a06-6785-3dc56032ef82",
    "BNodeTID": "488789f9-38be-4eba-704a-79996382de41",
    "LastUpdateMetric": {
      "ABBytes": 490,
      "ABPackets": 5,
      "BABytes": 490,
      "BAPackets": 5,
      "Last": 1477572621,
      "Start": 1477572616
    },
    "LayersPath": "Ethernet/IPv4/ICMPv4/Payload",
    "Link": {
      "A": "02:48:4f:c4:40:99",
      "B": "e2:d0:f0:61:e7:81",
      "Protocol": "ETHERNET"
    },
    "Metric": {
      "ABBytes": 1666,
      "ABPackets": 17,
      "BABytes": 1568,
      "BAPackets": 16,
      "Last": 1477572622,
      "Start": 1477572606
    },
    "Network": {
      "A": "192.168.0.1",
      "B": "192.168.0.2",
      "Protocol": "IPV4"
    },
    "NodeTID": "488789f9-38be-4eba-704a-79996382de41",
    "TrackingID": "f745fb1f59298a1773e35827adfa42dab4f469f9",
    "UUID": "ee29fc47f425d7a2e6de9379b0131f64a70fc991"
  }
]
```

## Capture

To create capture :

```console
POST /api/capture HTTP/1.1
Content-Type: application/json

{
  "GremlinQuery":"g.V().Has('TID', 'de0cba34-5d96-5ce6-698a-dffd2e674f95')"
}
```

```console
HTTP/1.1 200 OK
Content-Type: application/json; charset=UTF-8

{
  "UUID":"e2d9f084-4543-4f7e-6c2c-673f56ae4610",
  "GremlinQuery":"g.V().Has('TID', 'de0cba34-5d96-5ce6-698a-dffd2e674f95')"
}
```

To list captures :

```console
GET /api/capture HTTP/1.1
Content-Type: application/json
```

```console
{
  "104fc114-e153-4f67-692a-60c636ee1597":
  {
    "UUID": "104fc114-e153-4f67-692a-60c636ee1597"
    "GremlinQuery": "G.V().Has('TID', '2108e074-feac-5a3c-60ca-5963e89c4059')"
    "Count": 1
  },
  "e2d9f084-4543-4f7e-6c2c-673f56ae4610":
  {
    "UUID": "e2d9f084-4543-4f7e-6c2c-673f56ae4610"
    "GremlinQuery": "g.V().Has('TID', 'de0cba34-5d96-5ce6-698a-dffd2e674f95')"
    "Count": 1
  }
}
```

To delete a capture :

```console
DELETE /api/capture/7ca73f92-0547-475e-472d-d6e28664a117 HTTP/1.1
Content-Type: application/json
```

```
HTTP/1.1 200 OK
Content-Type: application/json; charset=UTF-8
```
