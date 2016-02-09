[![Build Status](https://travis-ci.org/redhat-cip/skydive.png)](https://travis-ci.org/redhat-cip/skydive)

# Skydive

Skydive is an open source real-time network topology and protocols analyzer. It aims to provide a comprehensive way of what is happening in the network infrastrure.

Skydive agents collect topology informations and flows and forward them to a central agent for further analysis. All the informations a stored in an Elasticsearch database.

Skydive is SDN-agnostic but provides SDN drivers in order to enhance the topology and flows informations. Currently only the Neutron driver is provided but more drivers will come soon.

## Topology Probes

Topology probes currently implemented:

* OVSDB
* NetLINK
* NetNS
* Ethtool

## Flow Probes

Flow probes currenlty implemented:

* sFlow

# Dependencies

* Go >= 1.5
* Elasticsearch

## Install

Make sure you have a working Go environment. [See the install instructions](http://golang.org/doc/install.html).

Then make sure you have Godep installed. [See the install instructions](https://github.com/tools/godep).

```console
$ go get github.com/redhat-cip/skydive/cmd/skydive_agent
$ go get github.com/redhat-cip/skydive/cmd/skydive_analyzer
```

## Getting started

Skydive relies on two main components:

* skydive_agent, has to be started on each node where the topology and flows informations will be captured
* skydive_analyzer, the node collecting data captured by the agents

### Configuration

A default configuration is present under etc/. For a single node setup only the analyzer and agent port need probably to be changed. For a multiple node setup the analyzer IP/PORT needs to be adapted.
Processes are binding 127.0.0.1 by default, you can explicitly change binding address with "listen = 0.0.0.0:port" in any configuration section.
The [openstack] section is optional. You can declare it if you want the analyzer to get informations from Openstack/Neutron.

```shell
[cache]
# expiration time in second
expire = 300

# cleanup interval in second
cleanup = 30

[openstack]
auth_url = http://xxx.xxx.xxx.xxx:5000/v2.0
username = admin
password = password123
tenant_name = admin
region_name = RegionOne

[analyzer]
flowtable_expire = 5
listen = 8082

[agent]
flowtable_expire = 5
listen = 8081
analyzers = 127.0.0.1:8082

[sflow]
# listen parameter for the sflow agent, Format: addr:port.
# Default addr is 127.0.0.1
listen = 6345

[ovs]
# ovsdb connection, Format: addr:port
# You need to authorize connexion to ovsdb agent at least locally
# % sudo ovs-appctl -t ovsdb-server ovsdb-server/add-remote ptcp:6400:127.0.0.1
ovsdb = 6400

[graph]
# graph backend memory, gremlin
backend = memory
gremlin = 127.0.0.1:8182

[storage]
elasticsearch = 127.0.0.1:9200
```
### Start

```console
$ skydive_agent -conf etc/skydive.ini
```
```console
$ skydive_analyzer -conf etc/skydive.ini
```

### WebUI

To access to the WebUI of agents or analyzer:

```console
http://<address>:<port>/static/topology
```

### API

Topology informations are accessible through a RPC API or a WebSocket API

RPC:

```console
curl http://<address>:<port>/rpc/topology
```

WebSocket endpoint:

```console
ws://<address>:<port>/ws/graph
```

Messages:

* NodeUpdated
* NodeAdded
* NodeDeleted
* EdgeUpdated
* EdgeAdded
* EdgeDeleted

## Contributing
This project accepts contributions. Just fork the repo and submit a pull request!

## License
This software is licensed under the Apache License, Version 2.0 (the "License");
you may not use this software except in compliance with the License.
You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
