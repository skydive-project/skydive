#!/bin/bash

set -v

export GOPATH=$WORKSPACE
export PATH=$PATH:$GOROOT/bin

# Get the Go dependencies
go get -f -u github.com/axw/gocov/gocov
go get -f -u github.com/mattn/goveralls
go get -f -u golang.org/x/tools/cmd/cover
go get -f -u github.com/golang/lint/golint
go get -f -u github.com/tebeka/go2xunit

export PATH=$PATH:$GOPATH/bin

# speedup govendor sync command
mkdir -p $HOME/.govendor $GOPATH/.cache
rm -rf $GOPATH/.cache/govendor
ln -s $HOME/.govendor $GOPATH/.cache/govendor
