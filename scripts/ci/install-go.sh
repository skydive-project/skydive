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

# pcap.h is required to build skydive
sudo yum -y install libpcap-devel

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
