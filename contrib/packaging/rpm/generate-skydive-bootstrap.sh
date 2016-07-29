#!/bin/bash

commit=$(git rev-parse --verify HEAD)
tagname=$(grep $commit <(git show-ref --tags))

if [ -n "$tagname" ]; then
    commit=$(echo $tagname | awk -F '/' '{print $NF}')
fi

set -e

git archive --format=tar --prefix=skydive-${commit}/src/github.com/skydive-project/skydive/ $commit > skydive-${commit}.tar

echo "go take a coffee, govendor sync takes time ..."
make govendor

# Append the vendor and govendor src to the archive then gzip it
tar -k -r --file=skydive-${commit}.tar --show-transformed --transform "s,vendor,skydive-${commit}/src/github.com/skydive-project/skydive/vendor," vendor
tar -P -k -r --file=skydive-${commit}.tar --exclude=.git --show-transformed --transform "s,$GOPATH/src/github.com/kardianos/govendor,skydive-${commit}/src/github.com/kardianos/govendor," $GOPATH/src/github.com/kardianos/govendor
gzip -f skydive-${commit}.tar

mkdir -p rpmbuild/SOURCES
mv -f skydive-${commit}.tar.gz rpmbuild/SOURCES
rpmbuild -ba --define "commit $commit" --define "_topdir $(pwd)/rpmbuild" contrib/packaging/rpm/skydive.spec
