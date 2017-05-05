# Change Log
All notable changes to this project will be documented in this file.

## [0.11.0] - 2017-05-05
### Added
- Elasticsearch:
  - Support Elasticsearch 5
  - Bulk indexing for graph and flows for improved performance
- WebUI:
  - Display bandwidth on L2 edges
  - Restore discovery and conversation views
- Allow loading multiple configuration files
- Support for logging to file and syslog
- Bash completion file

### Changed
- Keep all netlink events ordered
- Websocket:
  - Introduced namespace subscription mechanism
  - Validate messages using JSON schemas
  - Use bulk to reduce graph messages number
- Bug fixes:
  - Fix analyzer deadlock on agent stop
  - Return status error if capture registration failed

## [0.10.0] - 2017-03-30
### Added
- Support multiple analyzers for high availability and scalability
- Store interface metrics as node metadata
- Flows:
  - New 'pcapsocket' flow probe to inject pcap files
  - Support for VLANs
  - BPF support for afpacket, pcap and sFlow
  - Packet statistics counter for the 'pcap' capture type
- API:
  - New /api/config route to access configuration values
  - Introduce a WebSocket Python client
- Topology:
  - Report link speed
  - Allow link metadata definition in 'fabric' probe
  - Add Docker labels to metadata
- Gremlin:
  - Allow querying graph with a time slice: the Gremlin 'Context' step
    now accepts a second argument which is a duration. When specified, graph
    queries will return all the revisions of the matching nodes during this
    time period
  - Allow 'Metrics' step on graph queries
  - New 'HasNot' and 'HasKey' steps
  - Allow sorting in ascending and descending order for both flow and
    graph queries
- WebUI:
  - Add flow-table component
  - Allow to limit number of flows in flow-table
  - Allow collapsing all nodes belonging to a host
  - Show interface statistics in table

### Changed
- Switch all timestamps to milliseconds
- Changed analyzer.storage config section from a string to a dictionary
- Flows:
  - Consistent querying for live and stored flows
  - Remove analyzer flow table
- Gremlin:
  - 'Dedup' step now handles many keys
  - Remove since predicate as replace by context duration
- Topology:
  - Allow 'netns' probe to work within a containerized agent
- WebUI:
  - Speed up the topology display by reducing number of redraw
  - Migration to 'vuejs' JavaScript framework

## [0.9.0] - 2017-01-27
### Added
- Alerting:
  - Use Gremlin expression for alerts on both graph and flows
  - Allow writing alerts in JavaScript
  - Alerts can now execute scripts and trigger Webhooks
- New Grafana plugin (https://github.com/skydive-project/skydive-grafana-datasource)
- New OpenShift template
- Gremlin:
  - Add 'Aggregates' step
  - Allow sorting and deduplicating against a property
- Topology:
  - Handle IP adding / removal on interfaces
  - Add peering probe which creates a link based on MAC addresses
- Flows:
  - Support Geneve tunneling
  - Add Layer 3 tracking ID
  - Add support for Ethernet and IP over MPLS
- Neutron:
  - Adding Neutron attributed IPs to nodes
  - Add metadata on Neutron managed namespaces
- WebUI:
  - Keep position of nodes in local storage across browser refresh

### Changed
- Flows:
  - Use persistent ID in flows to handle agent restarts
- Topology:
  - Remove unsupported titan and gremlin backends
- Gremlin:
  - Convert 'Bandwidth' step to 'Metrics' and 'Sum'

## [0.8.0] - 2016-12-09
### Added
- Flows:
  - Add packet injector to generate traffic between two interfaces
  - Support Geneve tunneling
  - Add support for MPLS over both UDP and GRE
  - Add sFlow tunneling support
  - Add 'Application' metadata that contains the last layer type
- Gremlin:
  - New 'Metrics' Gremlin step to get all the metrics associated to a
    set of flows
  - New 'Values' step which returns values of a property:
    ex: G.V().Values('Name')
  - New 'Sum' step that returns aggregation over named node property:
    ex: G.Flows().Sum('Metric.ABBytes')
  - New 'Regex' predicated to use regular expression when querying graph
    and flows
  - New 'At' alias for 'Context' and allow more user friendly time definition
    ex: g.At('-5m').V()
  - New 'Within' filter for flows
  - New 'Since' predicate to select flows
  - New 'CaptureNode' step to get the capture nodes of a set of flows
- OpenStack:
  - Add support for Keystone V3
  - Autodetect Skydive agent running inside a virtual machine
- WebUI:
  - Add packet injector tab
  - Add support for zoom-in zoom-out and reset
  - Add the "famous" "mmmgnmm bob effect"
- Misc:
  - Experimental support for TLS communication between agents and analyzer
  - Vagrantfile to bootstrap a 3 nodes setup

### Changed
- Flows:
  - Manage captures from the analyzer to handle use case like setting capture
    point on all the interfaces between two nodes
  - Simplify packet processing and remove timeout mechanisms at the probe level
  - Correct packet length for outer/inner packets in tunnels
- Topology:
  - Move 'fabric' probe onto the analyzer and add a WebSocket API
- WebUI:
  - Update timeslider to use delta instead of date
  - Restore discovery and conversation views
  - Fix node selection issue after agent Resync
- Bugs fixes:
  - Many bugfixes around network namespaces that caused Goroutines to run
    in the wrong namespace that resulted in flow captures and packet injection
    errors
  - Fix leak of namespace fd in docker probe
- Build:
  - Do not integrate generated file anymore

## [0.7.0] - 2016-11-08
### Added
- Flows:
  - Encapsulation support (VXLAN, GRE)
  - IPv6 support
- Gremlin:
  - Add 'Sort' step to sort flows by uptime time
  - Add 'Nodes' step to get capture, source and destination nodes
- Topology:
  - Add cache for gremlin, titangraph and orientdb backends
  - Add OpenContrail probe
- WebUI:
  - Use shortest path to create captures
  - Support for tunneling
- Client:
  - Add an 'allinone' command
  - Add a 'shell' command
- Documentation:
  - Add quick start and contributing sections to README.md
  - Add rest, flow schema section and some fixes

### Changed
- Flows:
  - Reworked flow structurem to simplify exploitation and storage
  - Use ANode, BNode, Node instead of old name for flows
- Gremlin:
  - Optimize Dedup step for flows
  - Ensure error is passed for every step
- WebUI:
  - Replace flow jsonview by a flow grid table
  - Change node selection behaviour
- Docker:
  - Use scratch as Docker base image
  - Add missing configuration file for Docker image
- OpenStack:
  - Add availability configuration variable
  - Retrieve metadatas when port metadatas are updated
- Bugs fixes:
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
