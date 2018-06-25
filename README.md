[![Build Status](http://ci-logs.skydive.network/skydive-create-binaries/badge)](http://ci-logs.skydive.network/skydive-create-binaries/)
[![Go Report Card](https://goreportcard.com/badge/github.com/skydive-project/skydive)](https://goreportcard.com/report/github.com/skydive-project/skydive)
[![StackShare](https://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/skydive-project/skydive)

# Skydive

Skydive is an open source real-time network topology and protocols analyzer.
It aims to provide a comprehensive way of understanding what is happening in
the network infrastructure.

Skydive agents collect topology informations and flows and forward them to a
central agent for further analysis. All the informations are stored in an
Elasticsearch database.

Skydive is SDN-agnostic but provides SDN drivers in order to enhance the
topology and flows informations.

![](https://github.com/skydive-project/skydive.network/raw/images/overview.gif)

## Key features

* Captures network topology and flows
* Full history of network topology and flows
* Distributed
* Ability to follow a flow along a path in the topology
* Supports VMs and Containers infrastructure
* Unified query language for topology and flows (Gremlin)
* Web and command line interfaces
* REST API
* Easy to deploy (standalone executable)
* Connectors to OpenStack, Docker, OpenContrail

## Quick start

### Docker Compose

To quick set up a working environment, [Docker Compose](https://docs.docker.com/compose/)
can be used to automatically start an Elasticsearch container, a Skydive analyzer
container and a Skydive agent container.

```console
curl -o docker-compose.yml https://raw.githubusercontent.com/skydive-project/skydive/master/contrib/docker/docker-compose.yml
docker-compose up
```

Open a browser to http://localhost:8082 to access the analyzer Web UI.

You can also use the Skydive [command line client](https://skydive-project.github.io/skydive/getting-started/client/) with:
```console
docker run --net=host -ti skydive/skydive client query "g.V()"
```

### All-in-one

You can also download the latest release and use the `all-in-one` mode which
will start an Agent and an Analyzer at once.

```console
sudo skydive allinone [-c skydive.yml]
```

## Documentation

Skydive documentation can be found here:

* http://skydive.network/documentation

## Contributing

Your contributions are more than welcome. Please check
https://github.com/skydive-project/skydive/blob/master/CONTRIBUTING.md
to know about the process.

## Contact

* IRC: #skydive-project on irc.freenode.net
* Mailing list: https://www.redhat.com/mailman/listinfo/skydive-dev

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


