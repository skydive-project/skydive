#!/bin/bash

set -v
set -e

dir="$(dirname "$0")"

cd ${GOPATH}/src/github.com/skydive-project/skydive
make srpm
rpmlint contrib/packaging/rpm/skydive.spec

FULLVER=$(make -s version)
VERSION=$(make -s version | cut -d '-' -f 1)
TAG=$(make -s version | cut -d '-' -f 2- | tr '-' '.')
for srpm in $(ls rpmbuild/SRPMS/skydive-${VERSION}*${TAG}.src.rpm)
do
    for root in epel-7-x86_64 fedora-28-x86_64 fedora-29-x86_64
    do
        mock -r $root -D "fullver $FULLVER" --rebuild $srpm
    done
done
