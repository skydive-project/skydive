#!/bin/bash

set -v

dir="$(dirname "$0")"

go get -f -u github.com/golang/lint/golint

set -e

cd ${GOPATH}/src/github.com/skydive-project/skydive

make fmt
make vet

make check

make lint GOMETALINTER_FLAGS="--disable-all --enable=golint"
nbnotcomment=$(grep '"linter":"golint"' lint.json | wc -l)
if [ $nbnotcomment -gt 0 ]; then
   cat lint.json
   echo "===> You should comment you code <==="
   exit 1
fi
