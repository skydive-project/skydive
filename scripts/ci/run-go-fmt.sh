#!/bin/bash

set -v

dir="$(dirname "$0")"

go get -f -u github.com/golang/lint/golint

set -e

cd ${GOPATH}/src/github.com/skydive-project/skydive

make fmt
make vet

# check if unused package are listed in the vendor directory
if [ -n "$(${GOPATH}/bin/govendor list +u)" ]; then
   echo "You must remove these usused packages :"
   ${GOPATH}/bin/govendor list +u
   exit 1
fi

make lint GOMETALINTER_FLAGS="--disable-all --enable=golint"
nbnotcomment=$(grep '"linter":"golint"' lint.json | wc -l)
if [ $nbnotcomment -gt 0 ]; then
   cat lint.json
   echo "===> You should comment you code <==="
   exit 1
fi
