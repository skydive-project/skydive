[![Build Status](https://travis-ci.org/skydive-project/skydive.png)](https://travis-ci.org/skydive-project/skydive)
[![Go Report Card](https://goreportcard.com/badge/github.com/skydive-project/skydive)](https://goreportcard.com/report/github.com/skydive-project/skydive)
[![Coverage Status](https://coveralls.io/repos/github/skydive-project/skydive/badge.svg?branch=master)](https://coveralls.io/github/skydive-project/skydive?branch=master)

# Skydive

Skydive is an open source real-time network topology and protocols analyzer.
It aims to provide a comprehensive way of understanding what is happening in the network infrastructure.

Skydive agents collect topology informations and flows and forward them to a central agent for further analysis. All the informations are stored in an Elasticsearch database.

Skydive is SDN-agnostic but provides SDN drivers in order to enhance the topology and flows informations.

![](https://github.com/skydive-project/skydive.network/raw/images/overview.gif)

## Prerequisites

* IBM Cloud Private 2.1 or higher
* Kubernetes cluster 1.7 or higher

## Installing the Chart

To install the chart with the release name `my-release`:

```bash
$ helm install --name my-release stable/skydive
```

The command deploys skydive on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```bash
$ helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Security implications 

This chart deploys privileged kubernetes daemon-set. The implications are automatically creation of privileged container per kubernetes node capable of monitoring network and system behavior and used to capture Linux OS level information. The daemon-set also uses hostpath feature interacting with Linux OS, capturing info on network components.

## Configuration
The following tables lists the configurable parameters of skydive chart and their default values.

| Parameter                            | Description                                     | Default                                                    |
| ----------------------------------   | ---------------------------------------------   | ---------------------------------------------------------- |
| `image.repository`                   | Skydive image repository                        | `ibmcom/skydive`                                           |
| `image.tag`                          | Image tag                                       | `latest`                                                   |
| `image.imagePullPolicy`              | Image pull policy                               | `Always` if `imageTag` is `latest`, else `IfNotPresent`    |
| `resources`                          | CPU/Memory resource requests/limits             | Memory: `512Mi`, CPU: `100m`                               |
| `service.name`                       | service name                                    | `skydive`                                                  |
| `service.type`                       | k8s service type (e.g. NodePort, LoadBalancer)  | `NodePort`                                                 |
| `service.port`                       | TCP port                                        | `8082`                                                     |
| `analyzer.topology.fabric`           | Fabric connecting k8s nodes                     | `TOR1->*[Type=host]/eth0`                                  |
| `storage.elasticsearch.host`         | ElasticSearch end-point                         | `127.0.0.1:9200`                                           |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

## Topology fabric

The chart allows definition of static interfaces and links to be added to skydive topology view by setting the `analyzer.topology.fabric` parameter. This is useful to define external fabric resources like : TOR, Router, etc.
Details on this parameter field are available under the analyzer.topology.Fabric section in the following link: 
[https://github.com/skydive-project/skydive/blob/master/etc/skydive.yml.default](https://github.com/skydive-project/skydive/blob/master/etc/skydive.yml.default)
 

## Documentation

Skydive documentation can be found here:

* [http://skydive-project.github.io/skydive](http://skydive-project.github.io/skydive)


## Contributing

Your contributions are more than welcome. Please check
[https://github.com/skydive-project/skydive/blob/master/CONTRIBUTING.md](https://github.com/skydive-project/skydive/blob/master/CONTRIBUTING.md)
to know about the process.

## Contact and Support

* IRC: #skydive-project on [irc.freenode.net](https://webchat.freenode.net/)
* Mailing list: [https://www.redhat.com/mailman/listinfo/skydive-dev](https://www.redhat.com/mailman/listinfo/skydive-dev)
* Issues: [https://github.com/skydive-project/skydive/issues](https://github.com/skydive-project/skydive/issues)
