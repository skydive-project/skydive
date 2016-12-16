---
date: 2016-09-29T11:02:01+02:00
title: Flows
---

The Flow Schema is described in a
[protobuf file](https://github.com/skydive-project/skydive/blob/master/flow/flow.proto).

A typical Gremlin request on Flows will return a JSON version of the Flow
structure.

```console
$ skydive client topology query --gremlin "G.Flows().Limit(1)"
[
  {
    "ANodeTID": "422190f1-bbde-4eb0-4849-1fd1209229fe",
    "BNodeTID": "f3f1256b-7097-487c-7a02-38a32e009b3c",
    "LastUpdateMetric": {
      "ABBytes": 490,
      "ABPackets": 5,
      "BABytes": 490,
      "BAPackets": 5,
      "Last": 1477563666,
      "Start": 1477563661
    },
    "LayersPath": "Ethernet/IPv4/ICMPv4/Payload",
    "Link": {
      "A": "02:48:4f:c4:40:99",
      "B": "e2:d0:f0:61:e7:81",
      "Protocol": "ETHERNET"
    },
    "Metric": {
      "ABBytes": 21658,
      "ABPackets": 221,
      "BABytes": 21658,
      "BAPackets": 221,
      "Last": 1477563666,
      "Start": 1477563444
    },
    "Network": {
      "A": "192.168.0.1",
      "B": "192.168.0.2",
      "Protocol": "IPV4"
    },
    "NodeTID": "f3f1256b-7097-487c-7a02-38a32e009b3c",
    "TrackingID": "f745fb1f59298a1773e35827adfa42dab4f469f9",
    "UUID": "caa24da240cb3b40c84ebb708e2e5dcbe3c54784"
  }
]
```

Below the description of the fields :

* `UUID`, Unique ID of the flow. The ID is unique per capture point, meaning
  that a same flow will get a different ID for a different capture.
* `TrackingID`, ID of the Flow which is the same across all the
   captures point. This ID can be used to follow a Flow on each capture points.
* `NodeTID`, TID metadata of the interface node in the topology where the flow was
  captured.
* `ANodeTID`, TID metadata of the interface node in the topology where the packet is
  coming from.
* `BNodeTID`, TID metadata of the interface node in the topology where the packet is
  going to.
* `LayersPath`, All the layers composing the packets.
* `Link`, Link layer of the flow. A, B and Protocol describing the endpoints and
  the protocol of this layer.
* `Network`, Network layer of the flow. A, B and Protocol describing the
  endpoints and the protocol of this layer.
* `Transport`, Transport layer of the flow. A, B and Protocol describing the
  endpoints and the protocol of this layer.
* `Metric`, Current metrics of the flow. `AB*` stands for metrics from
  endpoint `A` to endpoint `B`, and `BA*` for the reverse path.
