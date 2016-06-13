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
```

## The classical two nodes deployment

Inside the Devstack folder of the Skydive sources there are two local.conf files
that can be used in order to deployment two Devstack with Skydive. The first
file will install a full Devstack with Skydive analyzer and agent. The second
one will install a compute Devstack with only the skydive agent.

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

$ skydive client -c /tmp/skydive.yaml capture create \
  --probepath "*/br-int[Type=ovsbridge]"
```

```console
$ skydive client -c /tmp/skydive.yaml capture list
{
  "*/br-int[Type=ovsbridge]": {
    "ProbePath": "*/br-int[Type=ovsbridge]"
  }
}
```

A request on Elasticsearch will give us the traffic captured. Here after a ping
between the qrouter and the qdhcp namespaces.

```console
curl -s http://localhost:9200/_search | jq .hits.hits[0]
{
  "_source": {
    "IfDstGraphPath": "devstack-1[Type=host]/qdhcp-cb5e9887-0779-48d1-aac6-b48e4d688056[Type=netns]/tap8ce0a68c-38[Type=internal]",
    "IfSrcGraphPath": "devstack-1[Type=host]/qrouter-c616325e-f3d9-497b-adfa-afe06a5a6245[Type=netns]/qr-26fe372c-b6[Type=internal]",
    "ProbeGraphPath": "devstack-1[Type=host]/br-int[Type=ovsbridge]",
    "TrackingID": "684924892c321f69599f826eb151acb103c4e8a3",
    "Statistics": {
      "Endpoints": [
        {
          "BA": {
            "Bytes": 4700,
            "Packets": 47,
            "Value": "fa:16:3e:85:ca:e1"
          },
          "AB": {
            "Bytes": 4700,
            "Packets": 47,
            "Value": "fa:16:3e:9c:49:a5"
          },
          "Type": "ETHERNET"
        },
        {
          "BA": {
            "Bytes": 3948,
            "Packets": 47,
            "Value": "10.0.0.2"
          },
          "AB": {
            "Bytes": 3948,
            "Packets": 47,
            "Value": "10.0.0.1"
          },
          "Type": "IPV4"
        }
      ],
      "Last": 1464279198,
      "Start": 1464279141
    },
    "LayersPath": "Ethernet/IPv4/ICMPv4/Payload",
    "UUID": "a24497056a12484c181585b7a1344b95e3197955"
  },
  "_score": 1,
  "_id": "a24497056a12484c181585b7a1344b95e3197955",
  "_type": "flow",
  "_index": "skydive_v1"
}
```
