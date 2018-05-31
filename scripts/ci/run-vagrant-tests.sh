#!/bin/bash

if [ -n "$(sudo virt-what)" ]; then
    echo "This test must running on baremetal host"
    exit 1
fi

[ -z "$MODES" ] && MODES="binary package container"

set -e
set -v

dir="$(dirname "$0")"

cd ${GOPATH}/src/github.com/skydive-project/skydive

if [ "$DEVMODE" == "true" ]; then
    make static
    make rpm BOOTSTRAP_ARGS=-l
    make docker-image
    docker save skydive/skydive:devel -o skydive-docker-devel.tar
fi

cd contrib/vagrant

function vagrant_cleanup {
    vagrant destroy --force
}
trap vagrant_cleanup EXIT

export ANALYZER_COUNT=2
export AGENT_COUNT=1

for mode in $MODES
do
  DEPLOYMENT_MODE=$mode vagrant box update
  DEPLOYMENT_MODE=$mode vagrant up --provision-with common
  DEPLOYMENT_MODE=$mode vagrant provision

  vagrant ssh analyzer1 -- sudo cat /etc/skydive/skydive.yml
  vagrant ssh analyzer2 -- sudo cat /etc/skydive/skydive.yml

  vagrant ssh analyzer1 -- sudo journalctl -n 100 -u skydive-analyzer
  vagrant ssh analyzer2 -- sudo journalctl -n 100 -u skydive-analyzer
  vagrant ssh agent1 -- sudo journalctl -n 100 -u skydive-agent

  vagrant ssh analyzer1 -- curl http://localhost:8082
  retcode=$?
  vagrant destroy --force
done
