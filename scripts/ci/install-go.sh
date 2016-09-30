#!/bin/bash

set -v

# Set Environment
echo ${PATH} | grep -q "${HOME}/bin" || {
  echo "Adding ${HOME}/bin to PATH"
  export PATH="${PATH}:${HOME}/bin"
}

# Install Go 1.6
mkdir -p ~/bin
curl -sL -o ~/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme
chmod +x ~/bin/gimme
eval "$(gimme 1.6)"

# pcap.h is required to build skydive
sudo yum -y install libpcap-devel
# for opencontrail probe
sudo yum -y install libxml2-devel

export GOPATH=$WORKSPACE

# Get the Go dependencies
go get -f -u github.com/axw/gocov/gocov
go get -f -u github.com/mattn/goveralls
go get -f -u golang.org/x/tools/cmd/cover
go get -f -u github.com/golang/lint/golint

export PATH=$PATH:$GOPATH/bin
