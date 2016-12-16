---
date: 2016-05-06T11:02:01+02:00
title: How to deploy Skydive with Devstack
---

## Devstack plugin

Skydive provides a DevStack plugin that can be used in order to have
Skydive Agents/Analyzer set up with the proper probes
by DevStack.

For a single node setup adding the following lines to your local.conf file
should be enough.

```console
enable_plugin skydive https://github.com/skydive-project/skydive.git

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

# List of public interfaces for the agents to register in fabric
#SKYDIVE_PUBLIC_INTERFACES=devstack1/eth0 devstack2/eth1
```

## The classical two nodes deployment

Inside the Devstack folder of the Skydive sources there are two local.conf files
that can be used in order to deployment two Devstack with Skydive. The first
file will install a full Devstack with Skydive analyzer and agent. The second
one will install a compute Devstack with only the skydive agent.

For Skydive to create a TOR object that links both Devstack, add the following
line to your local.conf file:
```console
SKYDIVE_PUBLIC_INTERFACES=devstack1/eth0 devstack2/eth1
```
where `devstack1` and `devstack2` are the hostnames of the two nodes followed
by their respective public interface.

Skydive will be set with the probes for OpenvSwitch and Neutron. It will be set
to use Keystone as authentication mechanism, so the credentials will be the same
than the admin.

Once you have your environment set up, going to the Analyzer Web Interface
should show similar to the following capture.

![WebUI Capture](/images/devstack-two-nodes.png)

## Capture traffic

Now we have our two nodes up and running we may want to start capturing
packets. The following command can be used in order to start a capture on all
the `br-int` bridges.

```console
$ export SKYDIVE_USERNAME=admin
$ export SKYDIVE_PASSWORD=password
$ export SKYDIVE_AGENT_ANALYZERS=localhost:8082 # Should be the same as SERVICE_HOST in local.conf

$ skydive client capture create --gremlin "G.V().Has('Name', 'br-int')"
```

```console
$ skydive client capture list
{
  "d62b3176-ebc8-44ed-7001-191270dc4d76": {
    "UUID": "d62b3176-ebc8-44ed-7001-191270dc4d76",
    "GremlinQuery": "G.V().Has('Name', 'br-int')",
    "Count": 1
  }
}
```

To get Flows captured :

```console
skydive client topology query --gremlin "G.Flows()"
[
  {
    "ANodeTID": "422190f1-bbde-4eb0-4849-1fd1209229fe",
    "BNodeTID": "f3f1256b-7097-487c-7a02-38a32e009b3c",
    "LastUpdateMetric": {
      "ABBytes": 490,
      "ABPackets": 5,
      "BABytes": 490,
      "BAPackets": 5,
      "Last": 1477567166,
      "Start": 1477567161
    },
    "LayersPath": "Ethernet/IPv4/ICMPv4/Payload",
    "Link": {
      "A": "02:48:4f:c4:40:99",
      "B": "e2:d0:f0:61:e7:81",
      "Protocol": "ETHERNET"
    },
    "Metric": {
      "ABBytes": 364560,
      "ABPackets": 3720,
      "BABytes": 364560,
      "BAPackets": 3720,
      "Last": 1477567165,
      "Start": 1477563444
    },
    "Network": {
      "A": "192.168.0.1",
      "B": "192.168.0.2",
      "Protocol": "IPV4"
    },
    "NodeTID": "f3f1256b-7097-487c-7a02-38a32e009b3c",
    "TrackingID": "f745fb1f59298a1773e35827adfa42dab4f469f9",
    "UUID": "caa24da240cb3b40c84ebb708e2e5dcbe3c54784"
  }
]
```

For a complete description of the flow structure can be found
[here](/api/flows/).
