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
ownership(host), membership(containers), layer2, etc.

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
G.V('ca909ccf-203d-457d-70b8-06fe308221efca909ccf-203d-457d-70b8-06fe308221ef')
```

### Has Step

`Has` step filters out the nodes that don't match the given metadata list. `Has`
can be applied either on nodes or edges.

```console
G.V().Has('Name': test, 'Type': 'netns')
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

### InE/OutE steps

`InE/OutE` steps returns the incoming/ougoing links.

```console
G.V().Has('Name': 'test', 'Type': 'netns').InE()
G.V().Has('Name': 'test', 'Type': 'netns').OutE()
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

`Dedup` removes duplicated nodes/links.

```console
G.V().Out().Both().Dedup()
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
G.V().Has('Type', 'netns').ShortestPathTo(Metadata('Type', 'host'), Metadata('Type', 'layer2'))
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

### Flows step

Flows step returns flows of the nodes where a capture has been started.
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

### Predicates

Predicates can be used with `Has`, `In*``, `Out*`` steps.

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
