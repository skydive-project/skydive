#!/bin/bash

if [ -n "$(sudo virt-what)" ]; then
    echo "This test must running on baremetal host"
    exit 1
fi

set -e
set -v

dir="$(dirname "$0")"

cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

cd contrib/vagrant

function vagrant_cleanup {
    vagrant destroy --force
}
trap vagrant_cleanup EXIT

for mode in dev binary package container
do
  DEPLOYMENT_MODE=$mode vagrant box update
  DEPLOYMENT_MODE=$mode vagrant up --provision-with common
  DEPLOYMENT_MODE=$mode vagrant provision
  vagrant destroy --force
done
