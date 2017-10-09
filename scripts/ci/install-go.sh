#!/bin/bash

set -v

# Set Environment
echo ${PATH} | grep -q "${HOME}/bin" || {
  echo "Adding ${HOME}/bin to PATH"
  export PATH="${PATH}:${HOME}/bin"
}

# Install Go
mkdir -p ~/bin
curl -sL -o ~/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme
chmod +x ~/bin/gimme

# before changing this be sure that it will not break the RHEL packaging
eval "$(gimme 1.8)"

export GOPATH=$WORKSPACE

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
