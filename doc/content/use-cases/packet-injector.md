Skydive provides a Packet injector, which is helpful to verify the successful packet flow between two network devices by injecting a packet from one device and capture the same in the other device.

The packet injector can be used with either the command line or the WebUI.

### How to use

To use the packet injector we need to provide the below parameters,

* Source node, needs to be expressed in gremlin query format.
* Destination node, needs to be expressed in gremlin query format, optional if dstIP and dstMAC given.
* Source IP of the packet, optional if source node given.
* Source MAC of the packet, optional if source node given.
* Destination IP of the packet, optional if destination node given.
* Destination MAC of the packet, optional if destination node given.
* Type of packet. currently only ICMP is supported.
* Number of packets to be generated, default is 1.

```console
$ skydive client inject-packet [flags]

Flags:
      --count int        number of packets to be generated (default 1)
      --dst string       destination node gremlin expression
      --dstIP string     destination node IP
      --dstMAC string    destination node MAC
      --payload string   payload
      --src string       source node gremlin expression
      --srcIP string     source node IP
      --srcMAC string    source node MAC
      --type string      packet type: icmp (default "icmp")
```

### Example
```console
$ skydive client inject-packet --src="G.V().Has('TID', 'feae10c1-240e-48e0-4a13-c608ffd157ab')" --dst="G.V().Has('TID', 'feae10c1-240e-48e0-4a13-c608ffd15700')" --type="icmp" --count=15
```
