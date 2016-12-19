---
date: 2016-05-06T11:02:01+02:00
title: Installation
---

## Introduction

Skydive relies on two main components:

* skydive agent, has to be started on each node where the topology and flows
  informations will be captured
* skydive analyzer, the node collecting data captured by the agents

## Dependencies

* Go >= 1.6
* Elasticsearch >= 2.0
* libpcap
* libxml2
* protoc >= 3.0

## Install

Make sure you have a working Go environment. [See the install instructions]
(http://golang.org/doc/install.html).

```console
$ mkdir -p $GOPATH/src/github.com/skydive-project
$ git clone https://github.com/skydive-project/skydive.git $GOPATH/src/github.com/skydive-project/skydive
$ cd $GOPATH/src/github.com/skydive-project/skydive
$ make install
```

## Configuration

For a single node setup, the configuration file is optional. For a multiple
node setup, the analyzer IP/PORT need to be adapted.

Processes are bound to 127.0.0.1 by default, you can explicitly change binding
address with "listen: 0.0.0.0:port" in the proper configuration sections.

User can add host metadata to specify an extra host information in
"agent.metadata" configuration section. All the key value pairs given
under this configuration section will be added to host metadata.

See the full list of configuration parameters in the sample configuration file
[etc/skydive.yml.default](https://github.com/skydive-project/skydive/blob/master/etc/skydive.yml.default).

## Start

```console
$ skydive agent [--conf etc/skydive.yml]
```
```console
$ skydive analyzer [--conf etc/skydive.yml]
```

## All-in-one

The `all-in-one` mode can be used to start an Agent and an Analyzer at once.

```console
$ skydive allinone [--conf etc/skydive.yml]
```
