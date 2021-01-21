[![GitHub license](https://img.shields.io/badge/license-Apache%20license%202.0-blue.svg)](https://github.com/skydive-project/skydive/blob/master/LICENSE)
[![Slack Invite](https://img.shields.io/badge/Slack:-%23skydive&hyphen;project%20invite-blue.svg?style=plastic&logo=slack)](https://slack.skydive.network)
[![Slack Channel](https://img.shields.io/badge/Slack:-%23skydive&hyphen;project-blue.svg?style=plastic&logo=slack)](https://skydive-project.slack.com)
[![Weekly minutes](https://img.shields.io/badge/Weekly%20Meeting%20Minutes-Thu%2010:30am%20CEST-blue.svg?style=plastic)](https://docs.google.com/document/d/1eri4vyjmAwxiWs2Kp4HYdCUDWACF_HXZDrDL8WcPF-o/edit?ts=5d946ad5#heading=h.g8f8gdfq0un9)
[![Go Report Card](https://goreportcard.com/badge/github.com/skydive-project/skydive)](https://goreportcard.com/badge/github.com/skydive-project/skydive)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2695/badge)](https://bestpractices.coreinfrastructure.org/projects/2695)
[![StackShare](https://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/skydive-project/skydive)
[![PyPI](https://img.shields.io/pypi/v/skydive-client.svg)](https://pypi.org/project/skydive-client/)

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
* Connectors to OpenStack, Docker, OpenContrail, Kubernetes

## Quick start

### All-in-one

The easiest way to get started is to download the latest binary and to run it using the `all-in-one` mode :

```console
curl -Lo - https://github.com/skydive-project/skydive-binaries/raw/jenkins-builds/skydive-latest.gz | gzip -d > skydive && chmod +x skydive && sudo mv skydive /usr/local/bin/

SKYDIVE_ETCD_DATA_DIR=/tmp SKYDIVE_ANALYZER_LISTEN=0.0.0.0:8082 sudo -E /usr/local/bin/skydive allinone
```

Open a browser to http://localhost:8082 to access the analyzer Web UI.

### Helm

If you are using Kubernetes then you can deploy skydive using helm directly from Git:

```console
helm plugin install https://github.com/aslafy-z/helm-git --version 0.10.0
helm repo add skydive git+https://github.com/skydive-project/skydive@contrib/charts
helm repo update
helm install skydive-analyzer skydive/skydive-analyzer
helm install skydive-agent skydive/skydive-agent
kubectl port-forward service/skydive-analyzer 8082:8082
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

## Get involved

* Weekly meeting
    * [General - Weekly meeting](https://meet.jit.si/skydive-project) - every Thursday at 5:00 - 5:30 PM CET ([Calendar](https://calendar.google.com/calendar/u/2?cid=c2t5ZGl2ZXNvZnR3YXJlQGdtYWlsLmNvbQ))
    * [Minutes](https://docs.google.com/document/d/1eri4vyjmAwxiWs2Kp4HYdCUDWACF_HXZDrDL8WcPF-o/edit?ts=5d946ad5#heading=h.g8f8gdfq0un9)

* Slack
    * Invite : https://slack.skydive.network
    * Workspace : https://skydive-project.slack.com

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
