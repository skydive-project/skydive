#!/bin/sh

ELASTICSEARCH_PORT="${ELASTICSEARCH_PORT:-127.0.0.1:9200}"
SKYDIVE_ANALYZER_PORT="${ANALYZER_PORT:-127.0.0.1:8082}"
SKYDIVE_ANALYZER_PORT_2379_ADDR="${ANALYZER_PORT_2379_ADDR:-127.0.0.1:2379}"

if [ ! -e /etc/skydive.yml ]
then
  cat > /etc/skydive.yml <<EOF
agent:
  listen: 0.0.0.0:8081
  analyzers: $SKYDIVE_ANALYZER_PORT
analyzer:
  listen: 0.0.0.0:8082
etcd:
  embedded: true
  servers:
    - http://$SKYDIVE_ANALYZER_PORT_2379_ADDR
storage:
  elasticsearch: $ELASTICSEARCH_PORT
EOF
fi

/usr/bin/skydive $1 --conf /etc/skydive.yml
