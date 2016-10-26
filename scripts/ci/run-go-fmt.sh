#!/bin/bash

set -v
dir="$(dirname "$0")"

. "${dir}/install-go.sh"
. "${dir}/install-requirements.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive

mkdir ${HOME}/protoc
pushd ${HOME}/protoc
wget https://github.com/google/protobuf/releases/download/v3.1.0/protoc-3.1.0-linux-x86_64.zip
unzip protoc-3.1.0-linux-x86_64.zip
popd
export PATH=${HOME}/protoc/bin:${PATH}

set -e
make .proto .bindata
difffiles=$(git diff --name-only)
if [ -n "$difffiles" ] ; then
    echo "error : make .proto/.bindata generate local files : $difffiles"
    exit 1
fi
make fmt
