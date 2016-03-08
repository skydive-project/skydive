#!/bin/bash

set -v

# Set Environment
echo ${PATH} | grep -q "${HOME}/bin" || {
  echo "Adding ${HOME}/bin to PATH"
  export PATH="${PATH}:${HOME}/bin"
}

# Install Go 1.5
mkdir -p ~/bin
curl -sL -o ~/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme
chmod +x ~/bin/gimme
eval "$(gimme 1.5)"

# Get the Go dependencies
export GOPATH=$HOME
go get -f -u github.com/axw/gocov/gocov
go get -f -u github.com/mattn/goveralls
go get -f -u golang.org/x/tools/cmd/cover
go get -f -u github.com/golang/lint/golint

export GOPATH=`pwd`/Godeps/_workspace
export PATH=$PATH:$GOPATH/bin

# Fake install of project
mkdir -p ${GOPATH}/src/github.com/redhat-cip/
ln -s $(pwd) ${GOPATH}/src/github.com/redhat-cip/skydive

# Install requirements
sudo yum install -y https://www.rdoproject.org/repos/rdo-release.rpm
sudo yum -y install make openvswitch
sudo service openvswitch start
sudo ovs-appctl -t ovsdb-server ovsdb-server/add-remote ptcp:6400

rpm -qi openvswitch

# Run tests
cd ${GOPATH}/src/github.com/redhat-cip/skydive
gofmt -s -l . | grep -v statics/bindata.go
make lint || true # (non-voting)
make test GOFLAGS=-race VERBOSE=true TIMEOUT=6m
