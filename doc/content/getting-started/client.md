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

## Topology requests

Skydive uses the Gremlin traversal language as a topology request language.
Requests on the topology can be done as following :

```console
$ skydive client topology query --gremlin "G.V().Has('Name', 'br-int', 'Type' ,'ovsbridge')"
[
  {
    "Host": "pc48.home",
    "ID": "1e4fc503-312c-4e4f-4bf5-26263ce82e0b",
    "Metadata": {
      "Name": "br-int",
      "Type": "ovsbridge",
      "UUID": "c80cf5a7-998b-49ca-b2b2-7a1d050facc8"
    }
  }
]
```
Refer to the [Gremlin section](/api/gremlin/) for further
explanations about the syntax and the functions available.

## Flow captures

Flow captures can be started from the WebUI or thanks to the Skydive client.
Skydive leverages the Gremlin language in order to select nodes on which a
capture will be started. The gremlin expression is continuously evaluated which
means that it is possible to define a capture on nodes that don't exist yet.
It useful when you want to start a capture on all OpenvSwitch whatever the
number of Skydive agents you will start.

The following command start a capture on all docker0 interfaces

```console
$ skydive client capture create --gremlin "G.V().Has('Name', 'docker0')"

{
  "UUID": "76de5697-106a-4f50-7455-47c2fa7a964f",
  "GremlinQuery": "G.V().Has('Name', 'docker0')"
}

```

While starting the capture, you can specify the capture name,
capture description and capture type optionally.
In order to know the list of supported capture types, see the usage doc of flow capture.

Node types that support captures are :

* ovsbridge
* veth
* device
* internal
* tun
* bridge

To delete a capture :

```console
$ skydive client capture delete <capture UUID>
```

The Flows Gremlin step can be used in order to see the flows captured. See the
[Gremlin section](/getting-started/gremlin/) for further explanations.

```console
skydive client topology query --gremlin "G.V().Has('Name', 'docker0').Flows()"
```
