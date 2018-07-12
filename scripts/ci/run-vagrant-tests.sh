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
    make srpm
    VERSION=$(make -s version | cut -d '-' -f 1)
    TAG=$(make -s version | cut -s -d '-' -f 2- | tr '-' '.')
    mock -r centos-7-x86_64 -D "fullver $(make -s version)" --resultdir . --rebuild rpmbuild/SRPMS/skydive-${VERSION}*${TAG}.src.rpm
    mkdir -p rpmbuild/RPMS/{x86_64,noarch}
    mv skydive*-${VERSION}-*${TAG}*.x86_64.rpm rpmbuild/RPMS/x86_64/
    mv skydive*-${VERSION}-*${TAG}*.noarch.rpm rpmbuild/RPMS/noarch/
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
export SKYDIVE_RELEASE=master

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
  if [ "$mode" != "container" ]; then
      vagrant ssh analyzer1 -c 'set -e; skydive client query "g.V()"'
  else
      CONTAINER=$(vagrant ssh analyzer1 -- sudo docker ps | grep 'skydive/skydive:latest' | awk '{print $1}')
      vagrant ssh analyzer1 -- sudo docker exec -t $CONTAINER skydive client query "'g.V()'"
  fi

  vagrant destroy --force
done
