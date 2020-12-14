[![GitHub license](https://img.shields.io/badge/license-Apache%20license%202.0-blue.svg)](https://github.com/skydive-project/skydive/blob/master/LICENSE)

# Graffiti

Graffiti is the open source, event based graph engine that powers [Skydive](https://github.com/skydive-project/skydive).

## Key features

* Distributed
* High availability
* Event based
* Keep the history of every change on the graph
* Extendable Gremlin based query language
* Support both strict and loose schema for metadata
* REST API
* Publish and subscribe using WebSocket
* [Web](https://github.com/skydive-project/skydive-ui) and command line interfaces
* Embedded or standalone
* Easy to deploy (single executable)

## Architecture

The 2 main components of Graffiti are:

### Hub

Holds the whole graph in memory and optionally store it in a persistent backend
(elasticsearch or OrientDB). When configured as a cluster, hubs will replicate the
graph between them. They are also in charge of serving the REST API.

### Pod (optional)

Holds and maintain its own local graph - usually smaller than the hub.
This graph will be forwarded to the hub using WebSocket.

## Quick start

### Install

```console
go get github.com/skydive-project/skydive/graffiti
```

### Start a hub

```console
$ graffiti hub --embedded-etcd
```

### Inject nodes

```console
$ graffiti client node create --node-name TestNode --node-type mytype --metadata MyValue=123
{
  "ID": "bf912bde-66be-461b-7b2f-15c9e1a3de53",
  "Metadata": {
    "MyValue": "123",
    "Name": "TestNode"
  },
  "Host": "lezordi",
  "Origin": "cli.lezordi",
  "CreatedAt": 1607037595421,
  "UpdatedAt": 1607037595421,
  "DeletedAt": null,
  "Revision": 1
}
```

```console
$ graffiti client node create --node-name TestNode2 --node-type mytype --metadata MyValue=456
{
  "ID": "b107a00e-9119-464e-4dc2-4216926c4769",
  "Metadata": {
    "MyValue": "456",
    "Name": "TestNode2"
  },
  "Host": "lezordi",
  "Origin": "cli.lezordi",
  "CreatedAt": 1607037657642,
  "UpdatedAt": 1607037657642,
  "DeletedAt": null,
  "Revision": 1
}
```

### Link nodes with edges

```console
$ graffiti client edge create --parent bf912bde-66be-461b-7b2f-15c9e1a3de53 --child b107a00e-9119-464e-4dc2-4216926c4769 --edge-type myrelation --metadata MyValue=789
{
  "Parent": "bf912bde-66be-461b-7b2f-15c9e1a3de53",
  "Child": "b107a00e-9119-464e-4dc2-4216926c4769",
  "ID": "2fb400a4-671b-4640-73ab-bb025a01ea99",
  "Metadata": {
    "MyValue": "789",
    "RelationType": "myrelation"
  },
  "Host": "lezordi",
  "Origin": "cli.lezordi",
  "CreatedAt": 1607037881247,
  "UpdatedAt": 1607037881247,
  "DeletedAt": null,
  "Revision": 1
}
```

### Query the graph

```console
$ graffiti client query "G.V().Has('MyValue', 456)
[
	{
		"ID": "b107a00e-9119-464e-4dc2-4216926c4769",
		"Metadata": {
			"MyValue": "456",
			"Name": "TestNode2"
		},
		"Host": "lezordi",
		"Origin": "cli.lezordi",
		"CreatedAt": 1607037657642,
		"UpdatedAt": 1607037657642,
		"DeletedAt": null,
		"Revision": 1
	}
]
```

```console
$ graffiti client query "G.E().Has('RelationType', 'myrelation').InV('MyValue', LT(200))"
[
	{
		"ID": "bf912bde-66be-461b-7b2f-15c9e1a3de53",
		"Metadata": {
			"MyValue": "123",
			"Name": "TestNode"
		},
		"Host": "lezordi",
		"Origin": "cli.lezordi",
		"CreatedAt": 1607037595421,
		"UpdatedAt": 1607037595421,
		"DeletedAt": null,
		"Revision": 1
	}
]
```

### Visualize the graph

```console
$ docker run -p 8080:8080 skydive/skydive-ui
```

Open a browser to http://localhost:8080 to access the Graffiti UI.

## Contributing

Your contributions are more than welcome. Please check
https://github.com/skydive-project/skydive/blob/master/CONTRIBUTING.md
to know about the process.

## License

This software is licensed under the Apache License, Version 2.0 (the
"License"); you may not use this software except in compliance with the
License.
You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
