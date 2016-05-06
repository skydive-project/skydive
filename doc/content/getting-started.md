---
date: 2016-05-06T10:57:18+02:00
title: Getting started
---

## Dependencies

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

For a single node setup, the configuration file is optional. For a multiple
node setup, the analyzer IP/PORT need to be adapted.

Processes are bound to 127.0.0.1 by default, you can explicitly change binding
address with "listen: 0.0.0.0:port" in the proper configuration sections.

See the full list of configuration parameters in the sample configuration file
[etc/skydive.yml.default](etc/skydive.yml.default).

### Start

```console
$ skydive agent [--conf etc/skydive.yml]
```
```console
$ skydive analyzer [--conf etc/skydive.yml]
```

### WebUI

To access to the WebUI of agents or analyzer:

```console
http://<address>:<port>
```

### Skydive client

Skydive client can be used to interact with Skydive Analyzer and Agents.
Running it without any command will return all the commands available.

```console
$ skydive client

Usage:
  skydive client [command]

Available Commands:
  alert       Manage alerts
  capture     Manage captures

Flags:
  -h, --help[=false]: help for client
      --password="": password auth parameter
      --username="": username auth parameter
```

Specifying the subcommand will give the usage of the subcommand.

```console
$ skydive client capture
```

If an authentication mechanism is defined in the configuration file the username
and password parameter have to be used for each command. Environment variables
SKYDIVE_USERNAME and SKYDIVE_PASSWORD can be used as default value for the
username/password command line parameters.

## Start Flow captures

Skydive client allows you to start flow captures on topology Nodes/Interfaces

```console
$ skydive client capture create -p <probe path>
```

The probe path parameter references the interfaces where the flow probe will be
started, so where the capture will be done.

The format of a probe path follows the links between topology nodes from
a host node to a target node :

```console
host1[Type=host]/.../node_nameN[Type=node_typeN]
```

The node name can be the name of :

* a host
* an interface
* a namespace

The node types can be :

* host
* netns
* ovsbridge

Currently target node types supported are :

* ovsbridge
* veth
* device
* internal
* tun
* bridge

To start a capture on the OVS bridge br1 on the host host1 the following probe
path is used :

```console
$ skydive client capture create -p "host1[Type=host]/br1[Type=ovsbridge]""
```

A wilcard for the host node can be used in order to start a capture on
all hosts.

```console
$ skydive client capture create -p "*/br1[Type=ovsbridge]"
```

A capture can be defined in advance and will start when a topology node will
match.

To delete a capture :

```console
$ skydive client capture delete <probe path>
```

## Devstack

Skydive provides a DevStack plugin that can be used in order to have
Skydive Agents/Analyzer configured and started with the proper probes
by DevStack.

For a single node setup adding the following lines to your local.conf file
should be enough.

```console
enable_plugin skydive https://github.com/redhat-cip/skydive.git

enable_service skydive-agent skydive-analyzer
```

The plugin accepts the following parameters:

```console
# Address on which skydive analyzer process listens for connections.
# Must be in ip:port format
#SKYDIVE_ANALYZER_LISTEN=

# Inform the agent about the address on which analyzers are listening
# Must be in ip:port format
#SKYDIVE_AGENT_ANALYZERS=

# ip:port address on which skydive agent listens for connections.
#SKYDIVE_AGENT_LISTEN=

# Configure the skydive agent with the etcd server address
# http://IP_ADDRESS:2379
#SKYDIVE_AGENT_ETCD=

# The path for the generated skydive configuration file
#SKYDIVE_CONFIG_FILE=

# List of agent probes to be used by the agent
# Ex: netns netlink ovsdb
#SKYDIVE_AGENT_PROBES=

# Remote port for ovsdb server.
#SKYDIVE_OVSDB_REMOTE_PORT=6640

# Set the default log level, default: INFO
#SKYDIVE_LOGLEVEL=DEBUG
```
