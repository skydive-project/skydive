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

tmpdir=`mktemp -d -u --suffix=skydive-pkg`
godir=${tmpdir}/skydive-${version}
skydivedir=${godir}/src/github.com/skydive-project/skydive
gitdir=$(cd "$(dirname "$0")/../../.."; pwd)

mkdir -p `dirname $skydivedir`
git clone $gitdir $skydivedir

pushd $skydivedir
export GOPATH=$godir
cd $skydivedir
echo "go take a coffee, govendor sync takes time ..."
make govendor genlocalfiles
popd

mkdir -p rpmbuild/SOURCES
tar -C $tmpdir --exclude=skydive-${version}/src/github.com/skydive-project/skydive/.git -cvf rpmbuild/SOURCES/skydive-${version}.tar.gz skydive-${version}/src
rm -rf $tmpdir

rpmbuild -ba --define "$define" --define "_topdir $PWD/rpmbuild" contrib/packaging/rpm/skydive.spec
