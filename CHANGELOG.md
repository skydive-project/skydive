# Change Log
All notable changes to this project will be documented in this file.

## [0.7.0] - 2016-11-08
### Added
Flows:
  - Encapsulation support (VXLAN, GRE)
  - IPv6 support
Gremlin:
  - Add 'Sort' step to sort flows by uptime time
  - Add 'Nodes' step to get capture, source and destination nodes
Topology:
  - Add cache for gremlin, titangraph and orientdb backends
  - Add OpenContrail probe
WebUI:
  - Use shortest path to create captures
  - Support for tunneling
Client:
  - Add an 'allinone' command
  - Add a 'shell' command
Documentation:
  - Add quick start and contributing sections to README.md
  - Add rest, flow schema section and some fixes

### Changed
Flows:
  - Reworked flow structurem to simplify exploitation and storage
  - Use ANode, BNode, Node instead of old name for flows
Gremlin:
  - Optimize Dedup step for flows
  - Ensure error is passed for every step
WebUI:
  - Replace flow jsonview by a flow grid table
  - Change node selection behaviour
Docker:
  - Use scratch as Docker base image
  - Add missing configuration file for Docker image
OpenStack:
  - Add availability configuration variable
  - Retrieve metadatas when port metadatas are updated
Bugs fixes:
  - Properly handle default values in configuration
  - Fix fd leak when using http clients
  - Stop timer of gopacket
  - Fix frozen agent when a flow query was used while when stopping flow table

## [0.6.0] - 2016-10-15
### Added
- Use elasticsearch as a time series database
- Add elasticsearch graph backend
- New node ID that persists between agent restart
- New afpacket capture type
- New 'fabric' probe to register external equipment like
  Top Of Rack switches
- Gremlin improvements:
  - Add 'Range' and 'Limit' Gremlin steps
  - Add 'Hops' step to get the nodes traversed by a set of flows
- Add architecture section to the documentation
- Add devstack functional testing

### Changed
- Skydive binary is linked statically with pcap
- Renamed 'pcap' probe to 'gopacket'
- Web UI improvements:
  - Clearer representation using groups of nodes
  - Node pinning
  - Highlight traversed nodes in Web UI
- Bug fixes:
  - Wait for analyzer before starting agent
  - Avoid capture create with duplicate gremlin request (both thanks to
    Konstantin Dorfman)
  - Fix shortestPath and add a test to illustrate the bug (thanks to
    Antoine Eiche)
  - Limit neutron API calls (thanks to Jean-Philippe Braun)
  - Use authenticated request while waiting for the analyzer (thanks to
    Mathieu Rohon)

## [0.5.0] - 2016-09-15
### Added
- Gremlin improvements:
  - Allow filtering flows in Gremlin query
  - Add bandwidth calculation
  - New steps: Count, Both
  - New predicates: Lt, Gt, Lte, Gte, Between, Inside, Outside

### Changed
- Scaling improvements
- Use protobuf instead of JSON for analyzer - agent communications
- Switch Docker image to Ubuntu
- Improved release pipeline (executables, Docker image)
