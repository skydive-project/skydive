---
date: 2016-05-06T11:02:01+02:00
title: Skydive client, API & WebUI
---

## Client

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

## WebUI

To access to the WebUI of agents or analyzer:

```console
http://<address>:<port>
```

## Flow captures

Flow captures can be started from the WebUI or thanks to the Skydive client :

```console
$ skydive client capture create --probepath <probe path>
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
$ skydive client capture create --probepath "host1[Type=host]/br1[Type=ovsbridge]""
```

A wilcard for the host node can be used in order to start a capture on
all hosts.

```console
$ skydive client capture create --probepath "*/br1[Type=ovsbridge]"
```

A capture can be defined in advance and will start when a topology node will
match.

To delete a capture :

```console
$ skydive client capture delete <probe path>
```
