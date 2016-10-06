# Change Log
All notable changes to this project will be documented in this file.

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
