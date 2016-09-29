---
date: 2016-09-29T11:02:01+02:00
title: Architecture
---

![Skydive Architecture](/images/architecture.png)

## Graph

Skydive relies on a event based graph engine, which means that notifications
are sent for each modification. Graphs expose notifications over WebSocket
connections. Skydive support multiple graph backends for the Graph. The `memory`
backend will be always used by agents while the backend for analyzers can be
choosen. Each modification is kept in the datastore so that we have a full
history of the graph. This is really useful to troubleshoot even if
interfaces do not exist anymore.

## Forwarder

Forwards graph messages from agents to analyzers so that analyzers can build
an aggregation of all agent graphs.

## Topology probes

Fill the graph with topology informations collected. Multiple probes fill the
graph in parallel. As an example there are probes filling graph with
network namespaces, netlink or OVSDB information.

## Flow table

Skydive keep a track of packets captured in flow tables. It allows Skydive to
keep metrics for each flows. At a given frequency or when the flow expires
(see the config file) flows are forwarded from agents to analyzers and then
to the datastore.

## Flow enhancer

Each time a new flow is received by the analyzer the flow is enhanced with
topology informations like where it has been captured, where it originates from,
where the packet is going to.

## Flow probes

Flow probes capture packets and fill agent flow tables. There are different
ways to capture packets like sFlow, afpacket, PCAP, etc.

## Gremlin engine

Skydive uses Gremlin language as its graph traversal language. The Skydive
Gremlin implementation allows to use Gremlin for flow traversal purpose.
The Gremlin engine can either retrieve informations from the datastore or from
agents depending whether the request is about something is the past or for live
monitoring/troubleshooting.

## Etcd

Skydive uses Etcd to store API objects like captures. Agents are watching Etcd
so that they can react on API calls.

## On-demand probe

This component watches Etcd and the graph in order to start captures. So when a
new capture is created by the API on-demande probe looks for graph nodes
matching the Gremlin expression, and if so, start capturing traffic.
