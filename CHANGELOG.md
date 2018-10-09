# Change Log

All notable changes to this project will be documented in this file.

## [0.20.0] - 2018-10-08
### Changed
- Switch to gopacket master branch
- Revamp Web UI
- Remove user metadata API
- Disable elasticsearch by default in devstack plugin
- Bug fixes:
  - Fix Limit for topology Gremlin steps
  - Fix packets missed to due timeout when using afpacket
  - Use Capture.ID instead of capture gremlin expression
  - Fix IPv6 connection state when using eBPF
  - Fix openflow rule modification

## [0.19.1] - 2018-09-13
### Added
- Add node/edge rules API to register nodes and edges
- Add Ansible library to create nodes and edges

### Changed
- Bug fixes:
  - Fix WebSocket flow authentication
  - Fix deployment on RHEL using Ansible
  - Fix SElinux policy to connect to Keystone
  - Change Docker base image to Ubuntu
  - Add origin field in python api

## [0.19.0] - 2018-08-08
### Added
- Gremlin:
  - Add `descendants` to retrieve both parents and children
  - Add `As` and `Select` steps to get the union of a set of nodes
  - Allow queries on booleans
- New JavaScript API
  - Support for browsers, NPM or the Skydive embedded JS engine
  - Convert command line shell to JavaScript
- Add API to upload and execute workflows
- Add support for Power architecture:
  - Build Docker images
  - Test architecture on our CI
- Retrieve OpenContrail routing tables

### Changed
- Improved Elasticseach backend:
  - Add support for rolling indices
  - Bump minimal version to 5.5
- Retrieve more Kubernetes metadata and create dedicated section on the Web UI for it
- Performance improvements using `easyjson`
- Allow using different authentication backends for API and for internal communications
- TripleO: move to config-download mechanism

## [0.18.0] - 2018-06-18
### Added
- Add `RBAC` mechanism
- Provide development `Vagrant` boxes on `Vagrantcloud`, supporting `VirtualBox` and `libvirt`
- Report more `Kubernetes` objects: deployment, services, daemonsets, jobs and more
- Report interface features from `netlink`
- New `FOREVER` and `NOW` Gremlin predicates
- Add `SELinux` policy security module for RPM packages
- Add authentication and `etcd` clustering to the `Ansible` playbooks

### Changed
- Flows:
  - Parsing code rework for correctness and performances
  - Fix metrics with multipath
  - Provide a way to define the layers used (L2/L3 or L3 only)
- Split `Keystone` auth section : one for the probe and one for authentication
- Support different elasticsearch connections per index
- WebUI:
  - Nicer sidebar
  - Dedicated section for routing table
  - Allowing managing alerts
  - Group `OpenFlow` rules by priority and actions
- `OVS-DPDK` fixes

## [0.17.0] - 2018-04-03
### Added
- Add Latency to WebUI topology links
- Packet Injector now allows to increment ICMP `id` for each packet
- New `Light` WebUI theme
- Add `Has` Gremlin step to `SocketInfo` step allowing to filter socket information
- New socketinfo probe to retrieve active sockets of a host.
  The new `Sockets` Gremlin step can be used to retrieve socket information corresponding to flows.
- Add `Bandwidth` to WebUI `Metric` tables
- Add clustering support for embedded Etcd
- `Aggregates` Gremlin step now uses fixed time slices
- Add LXD topology support
- Python API now suports TLS and authentication

### Changed
- SocketInfo now supports kernel wihtout ePBF support
- Fixed Flow metric issue on large packets
- Add capture `Name` to node metadata
- Fixed `RTT` display on WebUI

## [0.16.0] - 2018-01-29
### Added
- Add Kubernetes probe
- Retrieve Open vSwitch port metrics
- Allow traffic capture on Open vSwitch ports
- Add host info, such as CPU, memory, OS, Open vSwitch options, to metadata
- Add `Preferences` pane to the Web UI
- Allow SSH to agents through the WebUI (thanks to Dede)
- Allow cross compilation of Skydive

### Changed
- Long-running packet injections can now be stopped
- Performance improvements:
  - Add gzip support for both API and WebSocket
  - JSON serialization optimizations
- The `skydive-client` module now supports Python 2.7

## [0.15.0] - 2017-12-05
### Added
- Flow capture with eBPF probe
- Add routing table to the node metadata
- Python module `skydive-client` available through pypi
- Allow dynamic peering between analyzers
- Allow customizing the WebUI through external JS and CSS files

### Changed
- Use Dijkstra as graph shortest path algorithm
- Fix use of domain name and IPV6 in service addresses
- Scalability improvements:
  - Improve ping mechanism for better disconnection handling
  - Reduce graph lock pressure for Neutron and alerts

## [0.14.0] - 2017-11-14
### Added
- New DPDK probe
- Topology:
  - Allow filtering per client
  - Add ARP table to the node metadata
- WebUI:
  - Add sliding panel for filtering, highlighting and time selection
  - Add legend to report filter and time
  - Allow selecting capture type
- Allow deploying multi analyzers and elasticsearch with Vagrant
- Add support for Snort alert messages
- Handle network namespaces created by `docker network`

### Changed
- Bump minimal Go version to 1.8

## [0.13.0] - 2017-10-21
### Added
- New probe to retrieve and display for Openflow rules on OVS switch
- Flows:
  - Enable SYN/FIN/RST capturing
  - Add RTT per layer
  - Map flows to process
  - Use WebSocket for flows from the agents to the analyzers
- Gremlin:
  - Add `BPF` step to filter raw packets
  - Add `BothV` step
  - Add `Subgraph` step to provide a way to get a restricted view of the Graph.
    For ex: only the "layer2" topology.
  - Add `Ipv4Range` predicate which matches ipv4 in a range
  - Make `HasKey` work with complex metadata value, ex: list
- Packet injector:
  - Add payload support for TCP packets
  - Add support for UDP packets
- Ansible module to deploy Skydive
- API:
  - Add and remove user metadata on nodes
  - Add a status API

### Changed
- Change etcd default listen port

## [0.12.0] - 2017-07-28
### Added
- Full HTTPS support
- WebUI:
  - New implementation of the topology for better readability and performances
  - Make time slider easier to use
  - Add zoom fit button
- Flows:
  - New 'sflow' probe to support capture on physical interfaces
  - Allow keeping raw packets for flows
  - Add ICMP layer to flow structure
- Graph:
  - Make metadata a real JSON object
  - Add forwarding database to the node metadata
- Gremlin:
  - Add CONTAINS predicate to test if a value is in an array
  - Add BothE step that returns the incoming and outcoming edges for a node
  - Add E() step to return all the edges of a graph
- Packet injector:
  - Add icmpv6 support
  - Add TCP support
  - Make use of raw sockets
- API:
  - Allow returning the graph as dot

### Changed
- Graph:
  - Ensure correctness of the nodes and edges timestamps on both agents and
    analyzers
- Moved to Github and dedicated CI infrastructure
- Trigger alert when a successful evaluation changes
- Flows:
  - Enable all flow probes by default
  - Do not include payload in layer path
- Agents do not use etcd any more
- Use zap logger
- Bug fixes:
  - Resync captures when becoming master
  - Fix orphaned VLAN interfaces thanks to Mark Goddard
  - Fix handling of Docker labels containing a dot

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
