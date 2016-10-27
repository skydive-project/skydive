---
date: 2016-05-04T17:48:22+02:00
title: Overview
type: index
---

Skydive is an open source real-time network topology and protocols analyzer.
It aims to provide a comprehensive way of understanding what is happening in
the network infrastructure.

Skydive agents collect topology informations and flows and forward them to a
central agent for further analysis. All the informations are stored in an
Elasticsearch database.

Skydive is SDN-agnostic but provides SDN drivers in order to enhance the
topology and flows informations.

## Topology Probes supported

Topology probes currently implemented:

* OVSDB
* NetLINK
* NetNS
* Ethtool

Topology connectors:

* Neutron
* Docker
* Opencontrail

## Flow Probes supported

Flow probes currently implemented:

* sFlow
* PCAP
