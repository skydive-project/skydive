[![Go Report Card](https://goreportcard.com/badge/github.com/skydive-project/skydive)](https://goreportcard.com/badge/github.com/skydive-project/skydive)
[![GitHub license](https://img.shields.io/badge/license-Apache%20license%202.0-blue.svg)](https://github.com/networkservicemesh/networkservicemesh/blob/master/LICENSE)
[![StackShare](https://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/skydive-project/skydive)
[![PyPI](https://img.shields.io/pypi/v/skydive-client.svg)](https://pypi.org/project/skydive-client/)
[![IRC](https://www.irccloud.com/invite-svg?channel=%23skydive-project&amp;hostname=irc.freenode.net&amp;port=6697&amp;ssl=1)](http://webchat.freenode.net/?channels=skydive-project)

# Skydive

Skydive is an open source real-time network topology and protocols analyzer.
It aims to provide a comprehensive way of understanding what is happening in
the network infrastructure.

Skydive agents collect topology information and flows and forward them to a
central agent for further analysis. All the information is stored in an
Elasticsearch database.

Skydive is SDN-agnostic but provides SDN drivers in order to enhance the
topology and flows information.

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

### All-in-one

The easiest way to get started is to download the latest binary and to run it using the `all-in-one` mode :

```console
curl -Lo skydive https://github.com/skydive-project/skydive-binaries/raw/jenkins-builds/skydive-latest && \
chmod +x skydive && sudo mv skydive /usr/local/bin/

SKYDIVE_ETCD_DATA_DIR=/tmp SKYDIVE_ANALYZER_LISTEN=0.0.0.0:8082 sudo -E /usr/local/bin/skydive allinone
```

Open a browser to http://localhost:8082 to access the analyzer Web UI.

### Docker

```console
docker run -d --privileged --pid=host --net=host -p 8082:8082 -p 8081:8081 \
    -e SKYDIVE_ANALYZER_LISTEN=0.0.0.0:8082 \
    -v /var/run/docker.sock:/var/run/docker.sock -v /run/netns:/var/run/netns \
    skydive/skydive allinone
```

Open a browser to http://localhost:8082 to access the analyzer Web UI.

### Docker Compose

To quick set up a more complete working environment (with history support), [Docker Compose](https://docs.docker.com/compose/)
can be used to automatically start an Elasticsearch container, a Skydive analyzer
container and a Skydive agent container.

```console
curl -o docker-compose.yml https://raw.githubusercontent.com/skydive-project/skydive/master/contrib/docker/docker-compose.yml
docker-compose up
```

You can also use the Skydive [command line client](https://skydive-project.github.io/skydive/getting-started/client/) with:
```console
docker run --net=host -ti skydive/skydive client query "g.V()"
```

Open a browser to http://localhost:8082 to access the analyzer Web UI.

## Documentation

Skydive documentation can be found here:

* http://skydive.network/documentation

The Skydive REST API is described using swagger [here](http://skydive.network/swagger).

## Tutorials

Skydive tutorials can be found here:

* http://skydive.network/tutorials/first-steps-1.html

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
