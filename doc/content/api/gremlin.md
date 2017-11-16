---
date: 2016-06-22T11:02:01+02:00
title: Skydive Gremlin Query language
---

## Gremlin

Skydive uses a subset of the
[Gremlin language](https://en.wikipedia.org/wiki/Gremlin_\(programming_language\))
as query language for topology and flow requests.

A Gremlin expression is a chain of steps that are evaluated from left to right.
In the context of Skydive nodes stand for interfaces, ports, bridges,
namespaces, etc. Links stand for any kind of relation between two nodes,
ownership(host, netns, ovsbridge, containers), layer2, etc.

The following expression will return all the OpenvSwitch ports belonging to
an OpenvSwitch bridge named `br-int`.

```console
G.V().Has('Name', 'br-int', 'Type', 'ovsbridge').Out()

[
  {
    "Host": "test",
    "ID": "feae10c1-240e-48e0-4a13-c608ffd15700",
    "Metadata": {
      "Name": "vm2-eth0",
      "Type": "ovsport",
      "UUID": "f68f3593-68c4-4778-b47f-0ef291654fcf"
    }
  },
  {
    "Host": "test",
    "ID": "ca909ccf-203d-457d-70b8-06fe308221ef",
    "Metadata": {
      "Name": "br-int",
      "Type": "ovsport",
      "UUID": "e5b47010-f479-4def-b2d0-d55f5dbf7dad"
    }
  }
]
```

The query has to be read as :

1. `G` returns the topology Graph
2. `V` step returns all the nodes belonging the Graph
3. `Has` step returns only the node with the given metadata attributes
4. `Out` step returns outgoing nodes

## Traversal steps

The Skydive implements a subset of the Gremlin language steps and adds
"network analysis" specific steps.

### V Step

V step returns the nodes belonging to the graph.

```console
G.V()
```

A node ID can be passed to the V step which will return the corresponding node.

```console
G.V('ca909ccf-203d-457d-70b8-06fe308221ef')
```

### E Step

E step returns the edges belonging to the graph.

```console
G.E()
```

A edge ID can be passed to the E step which will return the corresponding edge.

```console
G.E('c8aeb26f-0962-4c46-b700-a12dfe720af1')
```

### Has Step

`Has` step filters out the nodes that don't match the given metadata list. `Has`
can be applied either on nodes or edges.

```console
G.V().Has('Name', 'test', 'Type', 'netns')
```

### In/Out/Both steps

`In/Out` steps returns either incoming, outgoing or neighbor nodes of
previously selected nodes.

```console
G.V().Has('Name', 'br-int', 'Type', 'ovsbridge').Out()
G.V().Has('Name', 'br-int', 'Type', 'ovsbridge').In()
G.V().Has('Name', 'br-int', 'Type', 'ovsbridge').Both()
```

Filters can be applied to these steps in order to select only the nodes
corresponding to the given metadata. In that case the step will act as a couple
of steps `Out/Has` for example.

```
G.V().Has('Name', 'br-int', 'Type', 'ovsbridge').Out('Name', 'intf1')
```

### InE/OutE/BothE steps

`InE/OutE/BothE` steps returns the incoming/ougoing links.

```console
G.V().Has('Name', 'test', 'Type', 'netns').InE()
G.V().Has('Name', 'test', 'Type', 'netns').OutE()
G.V().Has('Name', 'test', 'Type', 'netns').BothE()
```

Like for the `In/Out/Both` steps metadata list can be passed directly as
parameters in order to filter links.

### InV/OutV steps

`InV/OutV` steps returns incoming, outgoing nodes attached to the previously
selected links.

```console
G.V().OutE().Has('Type', 'layer2').InV()
```

### Dedup step

`Dedup` removes duplicated nodes/links or flows. `Dedup` can take a parameter
in order to specify the field used for the deduplication.

```console
G.V().Out().Both().Dedup()
G.V().Out().Both().Dedup('Type')
G.Flows().Dedup('NodeTID')
```

### Count step

`Count` returns the number of elements retrieved by the previous step.

```console
G.V().Count()
```

### Values step

`Values` returns the property value of elements retrieved by the previous step.

```console
G.V().Values('Name')
```

### Keys step

`Keys` returns the list of properties of the elements retrieved by the previous step.

```console
G.V().Keys()
```

### Sum step

`Sum` returns sum of elements, named 'Name', retrieved by the previous step.
When attribute 'Name' exists, must be integer type.

```console
G.V().Sum('Name')
```

### Limit step

`Limit` limits the number of elements returned.

```console
g.Flows().Limit(1)
```

### ShortestPathTo step

`ShortestPathTo` step returns the shortest path to node matching the given
`Metadata` predicate. This step returns a list of all the nodes traversed.

```console
G.V().Has('Type', 'netns').ShortestPathTo(Metadata('Type', 'host'))

[
  [
    {
      "Host": "test",
      "ID": "5221d3c3-3180-4a64-5337-f2f66b83ddd6",
      "Metadata": {
        "Name": "vm1",
        "Path": "/var/run/netns/vm1",
        "Type": "netns"
      }
    },
    {
      "Host": "test",
      "ID": "test",
      "Metadata": {
        "Name": "pc48.home",
        "Type": "host"
      }
    }
  ]
]
```

It is possible to filter the link traversed according to the given `Metadata`
predicate as a second parameter.

```
G.V().Has('Type', 'netns').ShortestPathTo(Metadata('Type', 'host'), Metadata('RelationType', 'layer2'))
```

### SubGraph step

`SubGraph` step returns a new Graph based on the previous steps. Step V or E can
be used to walk trough this new Graph.

```
G.E().Has('RelationType', 'layer2').SubGraph().V().Has('Name', 'eth0')
```

```
G.V().Has('Type', 'veth').SubGraph().E()
```

### GraphPath step

`GraphPath` step returns a path string corresponding to the reverse path
from the nodes to the host node they belong to.

```console
G.V().Has('Type', 'netns').GraphPath()

[
  "test[Type=host]/vm1[Type=netns]"
]
```

The format of the path returned is the following:
`node_name[Type=node_type]/.../node_name[Type=node_type]``

### At step

`At` allows to set the time context of the Gremlin request. It means that
we can contextualize a request to a specific point of time therefore being
able to see how was the graph in the past.
Supported formats for time argument are :

* Timestamp
* RFC1123 format
* [Go Duration format](https://golang.org/pkg/time/#ParseDuration)

```
G.At(1479899809).V()
G.At('-1m').V()
G.At('Sun, 06 Nov 2016 08:49:37 GMT').V()
```

`At` takes also an optional duration parameter which allows to specify a
period of time in second for the lookup. This is useful especially when retrieving
metrics. See [`Metrics` step](/api/gremlin#metrics-step) for more information.

```
G.At('-1m', 500).V()
G.At('-1m', 3600).Flows()
```

### Predicates

Predicates which can be used with `Has`, `In*`, `Out*` steps :

* `NE`, matches graph elements for which metadata don't match specified values

```console
G.V().Has('Type', NE('ovsbridge'))
```

* `Within`, matches graph elements for which metadata values match one member of
  the given array.

```console
G.V().Has('Type', Within('ovsbridge', 'ovsport'))
```

* `Without`, matches graph elements for which metadata values don't match any of
  the members of the given array.

```console
G.V().Has('Type', Without('ovsbridge', 'ovsport'))
```

* `Regex`, matches graph elements for which metadata matches the given regular
  expression.

```console
G.V().Has('Name', Regex('tap.*'))
```

Please note that the Regex step is always using anchors, ^ and $ don't have to
be provided in the expression.

### Flows step

Flows step returns flows of nodes where a capture has been started or of nodes
where the packets are coming from or going to.
The following Gremlin query returns the flows from the node `br-int` where
an sFlow capture has been started.
See the [client section](/getting-started/client/#flow-captures)
in order to know how to start a capture from a Gremlin query.

```console
G.V().Has('Name', 'br-int').Flows()
```

### Flows In/Out steps

From a flow step it is possible to get the node from where the packets are
coming or the node where packets are going to. Node steps are of course
applicable after `In/Out` flow steps.

```console
G.V().Has('Name', 'br-int').Flows().In()
G.V().Has('Name', 'br-int').Flows().Out()
```

### Flows Has step

`Has` step filters out the flows that don't match the given attributes list.

```console
G.Flows().Has('Network.A', '192.168.0.1')
```

Key can be any attributes of the Flow data structure :

* `UUID`
* `TrackingID`
* `NodeTID`
* `ANodeTID`
* `BNodeTID`
* `LayersPath`
* `Application`
* `Link`
* `Link.A`
* `Link.B`
* `Link.Protocol`
* `Network`
* `Network.A`
* `Network.B`
* `Network.Protocol`
* `Transport`
* `Transport.A`
* `Transport.B`
* `Transport.Protocol`
* `Metric.ABBytes`
* `Metric.BABytes`
* `Metric.ABPackets`
* `Metric.BAPackets`
* `Start`
* `Last`

Lt, Lte, Gt, Gte predicates can be used on numerical fields.
See [Flow Schema](/api/flows/) for further explanations.

Link, Network and Transport keys shall be matched with any of A or B by using OR operator.

### Flows Sort step

`Sort` step sorts flows by the given field and requested order.
By default, the flows are in ascending order by their `Last` field.
`ASC` and `DESC` predicates can be used to specify ascending and descending order respectively.

```console
G.Flows().Sort()
G.Flows().Sort("Metric.ABPackets")
G.Flows().Sort(DESC, "Metric.ABPackets")
```

### Flows Dedup step

`Dedup` step de-duplicates flows having the same TrackingID.

```console
G.Flows().Dedup()
```

### Metrics step

`Metrics` returns arrays of metrics of a set of flows or interfaces, grouped by
the flows UUIDs or Node IDs.

For flow metrics :

```console
G.Flows().Metrics()
[
  {
    "64249da029a25d09668ea4a61b14a02c3d083da0": [
      {
        "ABBytes": 980,
        "ABPackets": 10,
        "BABytes": 980,
        "BAPackets": 10,
        "Last": 1479899789,
        "Start": 1479899779
      }
  }
]
```

and for interface metrics :

```console
G.V().Metrics()
[
  {
    "fc2a6103-599e-4821-4c87-c8224bd0e84e": [
    {
      "Collisions": 0,
      "Last": 1489161773820,
      "Multicast": 0,
      "RxBytes": 7880,
      "RxCompressed": 0,
      "RxCrcErrors": 0,
      "RxDropped": 0,
      "RxErrors": 0,
      "RxFifoErrors": 0,
      "RxFrameErrors": 0,
      "RxLengthErrors": 0,
      "RxMissedErrors": 0,
      "RxOverErrors": 0,
      "RxPackets": 10,
      "Start": 1489161768820,
      "TxAbortedErrors": 0,
      "TxBytes": 7880,
      "TxCarrierErrors": 0,
      "TxCompressed": 0,
      "TxDropped": 0,
      "TxErrors": 0,
      "TxFifoErrors": 0,
      "TxHeartbeatErrors": 0,
      "TxPackets": 10,
      "TxWindowErrors": 0
    }

  }
]
```
