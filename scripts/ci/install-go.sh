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
eval "$(gimme 1.10.3)"

export GOPATH=$WORKSPACE
export PATH=$PATH:$GOPATH/bin

# speedup govendor sync command
mkdir -p $HOME/.govendor $GOPATH/.cache
rm -rf $GOPATH/.cache/govendor
ln -s $HOME/.govendor $GOPATH/.cache/govendor

# share compile cache
mkdir -p $HOME/pkg
rm -rf $GOPATH/pkg
ln -s $HOME/pkg $GOPATH/pkg
