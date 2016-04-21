#!/bin/bash

set -v

# Install apache gremlin server
cd ${HOME}
curl -s -L http://archive.apache.org/dist/incubator/tinkerpop/3.1.1-incubating/apache-gremlin-server-3.1.1-incubating-bin.zip > gremlin-server.zip
unzip gremlin-server.zip
export GREMLINPATH=${HOME}/apache-gremlin-server-3.1.1-incubating
