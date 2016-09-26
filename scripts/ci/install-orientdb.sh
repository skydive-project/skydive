#!/bin/bash

set -v

# Install OrientDB server
cd ${HOME}
curl -s -L "http://orientdb.com/download.php?email=unknown@unknown.com&file=orientdb-community-2.2.10.tar.gz&os=linux" > orientdb-server.tar.gz
tar xf orientdb-server.tar.gz
export ORIENTDBPATH=${HOME}/orientdb-community-2.2.10
