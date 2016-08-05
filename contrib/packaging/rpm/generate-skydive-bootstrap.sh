#!/bin/bash


from=HEAD
if [ -n "$1" ]; then
    from=$1
fi

define=""
version=$(git rev-parse --verify $from)
if [ $? -ne 0 ]; then
    echo "commit revision $from didn't exist"
    exit 1
fi
tagname=$(grep $version <(git show-ref --tags))
if [ -n "$tagname" ]; then
    version=$(echo $tagname | awk -F '/' '{print $NF}' | tr -d [a-z])
    define="tagversion $version"
else
    define="commit $version"
fi

set -e

echo "git archive --format=tar --prefix=skydive-${version}/src/github.com/skydive-project/skydive/ $from > skydive-${version}.tar"
git archive --format=tar --prefix=skydive-${version}/src/github.com/skydive-project/skydive/ $from > skydive-${version}.tar

echo "go take a coffee, govendor sync takes time ..."
make govendor

# Append the vendor and govendor src to the archive then gzip it
tar -k -r --file=skydive-${version}.tar --show-transformed --transform "s,vendor,skydive-${version}/src/github.com/skydive-project/skydive/vendor," vendor
tar -P -k -r --file=skydive-${version}.tar --exclude=.git --show-transformed --transform "s,$GOPATH/src/github.com/kardianos/govendor,skydive-${version}/src/github.com/kardianos/govendor," $GOPATH/src/github.com/kardianos/govendor
gzip -f skydive-${version}.tar

mkdir -p rpmbuild/SOURCES
mv -f skydive-${version}.tar.gz rpmbuild/SOURCES

rpmbuild -ba --define "$define" --define "_topdir $PWD/rpmbuild" contrib/packaging/rpm/skydive.spec
