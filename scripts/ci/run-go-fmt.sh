#!/bin/bash

set -v
dir="$(dirname "$0")"

. "${dir}/install-requirements.sh"
. "${dir}/install-go.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive

make fmt
make vet

# check if unused package are listed in the vendor directory
if [ -n "$(${GOPATH}/bin/govendor list +u)" ]; then
   echo "You must remove these usused packages :"
   ${GOPATH}/bin/govendor list +u
   exit 1
fi
