# Skydive Helm Chart
Skydive is an open source real-time network topology and protocols analyzer.

## About this chart

This charts deploys a typical Skydive Project containing the following:
* per-host *agent* component.
* per-cluster *analyzer* component.

## Prerequisites

* Kubernetes 1.7+
* RedHat OpenShift 3.6+
* IBM Cloud Private 2.1+

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

## Configuration

The following table lists the configurable parameters of skydive chart and their default values.

| Parameter                            | Description                                     | Default                                                    |
| ----------------------------------   | ---------------------------------------------   | ---------------------------------------------------------- |
| `image.repository`                   | Skydive image repository                        | `skydive/skydive`                                          |
| `image.tag`                          | Image tag                                       | `latest`                                                   |
| `image.imagePullPolicy`              | Image pull policy                               | `Always` if `imageTag` is `latest`, else `IfNotPresent`    |
| `resources`                          | CPU/Memory resource requests/limits             | Memory: `512Mi`, CPU: `100m`                               |
| `service.name`                       | service name                                    | `skydive`                                                  |
| `service.type`                       | k8s service type (e.g. NodePort, LoadBalancer)  | `NodePort`                                                 |
| `service.port`                       | TCP port                                        | `8082`                                                     |
| `analyzer.topology.fabric`           | Statically created interfaces and links, typically external fabric resources like: TOP, Router.  | `TOR1->*[Type=host]/eth0`                                  |
[https://github.com/skydive-project/skydive/blob/master/etc/skydive.yml.default](https://github.com/skydive-project/skydive/blob/master/etc/skydive.yml.default)
| `storage.elasticsearch.host`         | ElasticSearch end-point                         | `127.0.0.1:9200`                                           |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example,

```bash
$ helm install --name my-release -f values.yaml stable/skydive
```

## Testing

Helm tests are included and they confirm that the components are operating correctly:

```bash
helm test my-release
```
## Documentation

Skydive documentation can be found here:

* [http://skydive-project.github.io/skydive](http://skydive-project.github.io/skydive)

## Contact and Support

* IRC: #skydive-project on [irc.freenode.net](https://webchat.freenode.net/)
* Mailing list: [https://www.redhat.com/mailman/listinfo/skydive-dev](https://www.redhat.com/mailman/listinfo/skydive-dev)
* Issues: [https://github.com/skydive-project/skydive/issues](https://github.com/skydive-project/skydive/issues)

## Security implications 

This chart deploys privileged kubernetes daemon-set. The implicationsareautomatically creation of privileged container per kubernetes nodecapable ofmonitoring network and system behavior and used to captureLinux OS levelinformation. The daemon-set also uses hostpath featureinteracting with LinuxOS, capturing info on network components.
