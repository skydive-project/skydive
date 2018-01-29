Skydive provides a Packet injector, which is helpful to verify the successful packet flow between two network devices by injecting a packet from one device and capture the same in the other device.

The packet injector can be used with either the command line or the WebUI.

### How to use

## Create packet injector

To use the packet injector we need to provide the below parameters,

* Source node, needs to be expressed in gremlin query format.
* Destination node, needs to be expressed in gremlin query format, optional if dstIP and dstMAC given.
* Source IP of the packet, optional if source node given.
* Source MAC of the packet, optional if source node given.
* Destination IP of the packet, optional if destination node given.
* Destination MAC of the packet, optional if destination node given.
* Type of packet. currently ICMP, TCP and UDP are supported.
* Number of packets to be generated, default is 1.
* IMCP ID, used only for ICMP type packets.
* Interval, the delay between two packets in milliseconds.
* Payload
* Source port, used only for TCP and UDP packets, if not given generates one randomly.
* Destination port, used only for TCP and UDP packets, if not given generates one randomly.

```console
$ skydive client inject-packet create [flags]

Flags:
      --count int        number of packets to be generated (default 1)
      --dst string       destination node gremlin expression
      --dstIP string     destination node IP
      --dstMAC string    destination node MAC
      --dstPort int      destination port for TCP packet
      --id int           ICMP identification
      --interval int     wait interval milliseconds between sending each packet (default 1000)
      --payload string   payload
      --src string       source node gremlin expression (mandatory)
      --srcIP string     source node IP
      --srcMAC string    source node MAC
      --srcPort int      source port for TCP packet
      --type string      packet type: icmp4, icmp6, tcp4, tcp6, udp4 and udp6 (default "icmp4")
```

## Example
```console
$ skydive client inject-packet create --src="G.V().Has('TID', 'b0acebfb-9cd0-5de0-787f-366fcccc6651')" --dst="G.V().Has('TID', 'c94a12fd-f159-5228-7c05-e49b3c2bbb04')" --type="icmp4" --count=50 --interval=5000

{
  "UUID": "5b269b65-df92-42c6-4e69-4f8aa30f6110",
  "Src": "G.V().Has('TID', 'b0acebfb-9cd0-5de0-787f-366fcccc6651')",
  "Dst": "G.V().Has('TID', 'c94a12fd-f159-5228-7c05-e49b3c2bbb04')",
  "SrcIP": "",
  "DstIP": "",
  "SrcMAC": "",
  "DstMAC": "",
  "SrcPort": 0,
  "DstPort": 0,
  "Type": "icmp4",
  "Payload": "",
  "TrackingID": "e66c4b372e312be791238099538d8dda1949836b",
  "ICMPID": 0,
  "Count": 50,
  "Interval": 5000,
  "StartTime": "0001-01-01T00:00:00Z"
} 
```

## Delete packet injection
Deleting the active packet injector can be done by using the `delete` sub-command with the UUID of the injector.

UUID of the injection will be returend as a response of `create`

## Example
```console
$ skydive client inject-packet delete 5b269b65-df92-42c6-4e69-4f8aa30f6110
```
