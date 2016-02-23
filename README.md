[![Build Status](https://travis-ci.org/redhat-cip/skydive.png)](https://travis-ci.org/redhat-cip/skydive)

# Skydive

Skydive is an open source real-time network topology and protocols analyzer.
It aims to provide a comprehensive way of understanding what is happening in
the network infrastructure.

Skydive agents collect topology informations and flows and forward them to a
central agent for further analysis. All the informations a stored in an
Elasticsearch database.

Skydive is SDN-agnostic but provides SDN drivers in order to enhance the
topology and flows informations. Currently only the Neutron driver is provided
but more drivers will come soon.

## Topology Probes

Topology probes currently implemented:

* OVSDB
* NetLINK
* NetNS
* Ethtool

## Flow Probes

Flow probes currently implemented:

* sFlow

# Dependencies

* Go >= 1.5
* Elasticsearch >= 2.0

## Install

Make sure you have a working Go environment. [See the install instructions]
(http://golang.org/doc/install.html).

Then make sure you have Godep installed. [See the install instructions]
(https://github.com/tools/godep).

```console
$ go get github.com/redhat-cip/skydive/cmd/skydive
```

## Getting started

Skydive relies on two main components:

* skydive agent, has to be started on each node where the topology and flows
  informations will be captured
* skydive analyzer, the node collecting data captured by the agents

### Configuration

A default configuration is present under etc/. For a single node setup only the
analyzer and agent port need probably to be changed. For a multiple node setup
the analyzer IP/PORT needs to be adapted.

Processes are binding 127.0.0.1 by default, you can explicitly change binding
address with "listen = 0.0.0.0:port" in any configuration section.
The [openstack] section is optional. You can declare it if you want the
analyzer to get informations from Openstack/Neutron.

```shell
cache:
  # expiration time in second
  expire: 300
  # cleanup interval in second
  cleanup: 30

openstack:
  auth_url: http://xxx.xxx.xxx.xxx:5000/v2.0
  username: admin
  password: password123
  tenant_name: admin
  region_name: RegionOne

analyzer:
  flowtable_expire: 5
  listen: 8082

agent:
  flowtable_expire: 5
  listen: 8081
  analyzers: 127.0.0.1:8082
  topology:
    # Probes used to capture topology informations like interfaces,
    # bridges, namespaces, etc. Available netlink, netns, ovsdb.
    # Default: netlink, netns
    probes:
      - netlink
      - netns
      # - ovsdb
  flow:
    # Probes used to capture traffic.
    probes:
      # - ovssflow

sflow:
  # listen parameter for the sflow agent, Format: addr:port.
  # Default addr is 127.0.0.1
  listen: 6345

ovs:
  # ovsdb connection, Format: addr:port
  # You need to authorize connexion to ovsdb agent at least locally
  # % sudo ovs-appctl -t ovsdb-server ovsdb-server/add-remote ptcp:6400:127.0.0.1
  ovsdb: 6400

graph:
  # graph backend memory, gremlin
  backend: memory
  gremlin: 127.0.0.1:8182

storage:
  elasticsearch: 127.0.0.1:9200
```
### Start

```console
$ skydive agent --conf etc/skydive.yml
```
```console
$ skydive analyzer --conf etc/skydive.yml
```

### WebUI

To access to the WebUI of agents or analyzer:

```console
http://<address>:<port>
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
This project accepts contributions. Skydive uses the Gerrit workflow
through Software Factory.

http://softwarefactory-project.io/r/#/q/project:skydive

### Setting up your environment

```console
git clone http://softwarefactory-project.io/r/skydive
```

git-review installation :

```console
yum install git-review

```

or


```console
apt-get install git-review
```

or to get the latest version

```console
sudo pip install git-review
```

### Starting a Change

Create a topic branch :

```console
git checkout -b TOPIC-BRANCH
```

Submit your change :

```console
git review
```

Updating your Change :

```console
git commit -a --amend
git review
```

For a more complete documentation about
[how to contribute to a gerrit hosted project](https://gerrit-documentation.storage.googleapis.com/Documentation/2.12/intro-quick.html#_the_life_and_times_of_a_change).


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
