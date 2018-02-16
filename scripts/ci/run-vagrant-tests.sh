#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

cd contrib/vagrant

for mode in dev binary package container
do
  DEPLOYMENT_MODE=$mode vagrant box update
  DEPLOYMENT_MODE=$mode vagrant up --provision-with common
  DEPLOYMENT_MODE=$mode vagrant provision
  retcode=$?
  vagrant destroy
  [ $retcode -ne 0 ] && exit $retcode || true
done

exit 0
