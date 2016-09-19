#!/bin/sh

ELASTICSEARCH_PORT="${ELASTICSEARCH_PORT:-127.0.0.1:9200}"
SKYDIVE_ANALYZER_PORT="${ANALYZER_PORT:-127.0.0.1:8082}"
SKYDIVE_ANALYZER_PORT_2379_ADDR="${ANALYZER_PORT_2379_ADDR:-127.0.0.1:2379}"
OVSDB="${OVSDB:-unix:///var/run/openvswitch/db.sock}"
SKYDIVE_NETNS_RUN_PATH="${SKYDIVE_NETNS_RUN_PATH:-/host/run}"
SKYDIVE_LOG_LEVEL="${SKYDIVE_LOG_LEVEL:-INFO}"
FLOWTABLE_UPDATE="${FLOWTABLE_UPDATE:-10}"
FLOWTABLE_EXPIRE="${FLOWTABLE_EXPIRE:-300}"

if [ ! -e /etc/skydive.yml ]
then
  cat > /etc/skydive.yml <<EOF
agent:
  listen: 0.0.0.0:8081
  analyzers: $SKYDIVE_ANALYZER_PORT
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
      - docker
      - neutron
  flow:
    probes:
      - ovssflow
      - gopacket
analyzer:
  listen: 0.0.0.0:8082
  storage: elasticsearch
  flowtable_update: $FLOWTABLE_UPDATE
  flowtable_expire: $FLOWTABLE_EXPIRE
etcd:
  embedded: true
  servers:
    - http://$SKYDIVE_ANALYZER_PORT_2379_ADDR
storage:
  elasticsearch:
    host: $ELASTICSEARCH_PORT
    maxconns: 10
    retry: 60
ovs:
  ovsdb: $OVSDB
docker:
  url: unix:///var/run/docker.sock
netns:
  run_path: $SKYDIVE_NETNS_RUN_PATH
logging:
  default: $SKYDIVE_LOG_LEVEL
EOF
fi

/usr/bin/skydive $1 --conf /etc/skydive.yml
