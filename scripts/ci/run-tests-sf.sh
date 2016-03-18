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
sudo yum -y install make openvswitch unzip java-1.8.0-openjdk
sudo service openvswitch start
sudo ovs-appctl -t ovsdb-server ovsdb-server/add-remote ptcp:6400

rpm -qi openvswitch

# Install apache gremlin server
cd ${HOME}
curl -s -L http://apache.crihan.fr/dist/incubator/tinkerpop/3.1.1-incubating/apache-gremlin-server-3.1.1-incubating-bin.zip > gremlin-server.zip
unzip gremlin-server.zip
export GREMLINPATH=${HOME}/apache-gremlin-server-3.1.1-incubating

# Run tests
cd ${GOPATH}/src/github.com/redhat-cip/skydive
gofmt -s -l . | grep -v statics/bindata.go
make lint || true # (non-voting)
make test GOFLAGS=-race VERBOSE=true TIMEOUT=6m

# Run functionals test
make test.functionals GOFLAGS=-race VERBOSE=true TIMEOUT=6m
RET=$?
if [ ${RET} -ne 0 ]; then
  exit ${RET}
fi

# test with websocket gremlin server
cd ${GREMLINPATH}
${GREMLINPATH}/bin/gremlin-server.sh ${GREMLINPATH}/conf/gremlin-server.yaml &
GREMLINPID=$!
sleep 5
cd ${GOPATH}/src/github.com/redhat-cip/skydive
make test.functionals GOFLAGS=-race VERBOSE=true TIMEOUT=6m ARGS="-graph.backend gremlin-ws"
RET=$?
kill ${GREMLINPID}
if [ ${RET} -ne 0 ]; then
  exit ${RET}
fi

sleep 5

# test with rest gremlin server
cd ${GREMLINPATH}
${GREMLINPATH}/bin/gremlin-server.sh conf/gremlin-server-rest-modern.yaml &
GREMLINPID=$!
sleep 5
cd ${GOPATH}/src/github.com/redhat-cip/skydive
make test.functionals GOFLAGS=-race VERBOSE=true TIMEOUT=6m ARGS="-graph.backend gremlin-rest"
RET=$?
kill ${GREMLINPID}
if [ ${RET} -ne 0 ]; then
  exit ${RET}
fi
