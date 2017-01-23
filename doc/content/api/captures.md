---
date: 2017-01-05T11:44:01+02:00
title: Captures
---


Flow captures can be started from the WebUI or thanks to the [Skydive client](/getting-started/client).
Skydive leverages the [Gremlin language](/api/gremlin/) in order to select nodes on which a
capture will be started. The Gremlin expression is continuously evaluated which
means that it is possible to define a capture on nodes that do not exist yet.
It useful when you want to start a capture on all OpenvSwitch whatever the
number of Skydive agents you will start.

While starting the capture, you can specify the capture name,
capture description and capture type optionally.

At this time, the following capture types are supported:

* `ovssflow`, for interfaces managed by OpenvSwitch such as OVS bridges
* `afpacket`, for interfaces suchs as Linux bridges, veth, devices, ...
* `pcap`, same as `afpacket`
* `pcapsocket`. This capture type allows you to inject traffic from a PCAP file.
  See [below](/api/captures#pcap-files) for more information.

Node types that support captures are :

* ovsbridge
* veth
* device
* internal
* tun
* bridge

### PCAP files

If the flow probe `pcapsocket` is enabled, you can create captures with the
type `pcapsocket`. Skydive will create a TCP socket where you can copy PCAP
files (using `nc` for instance). Traffic injected into this socket will have
its capture point set to the selected node. The TCP socket address can be
retrieved using the `PCAPSocket` attribute of the node or using the
`PCAPSocket` attribute of the capture.
