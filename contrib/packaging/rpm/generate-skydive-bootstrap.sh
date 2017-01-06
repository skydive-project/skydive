#!/bin/bash

function usage {
  echo "Usage: $0 [-b|-s|-a] [-r tag_or_commit]"
}

from=HEAD
build_opts=-ba

while getopts ":asb:r:" opt; do
  case $opt in
    a)
      build_opts="-ba"
      ;;
    s)
      build_opts="-bs"
      ;;
    b)
      build_opts="-bb"
      ;;
    r)
      from=$OPTARG
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      usage
      exit 1
      ;;
  esac
done

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
tar -C $tmpdir --exclude=skydive-${version}/src/github.com/skydive-project/skydive/.git -cvzf rpmbuild/SOURCES/skydive-${version}.tar.gz skydive-${version}/src
$(dirname "$0")/specfile-update-bundles $gitdir/vendor/vendor.json > $tmpdir/skydive.spec

rpmbuild --nodeps $build_opts --undefine dist --define "$define" --define "_topdir $PWD/rpmbuild" $tmpdir/skydive.spec

echo $tmpdir/skydive.spec
rm -rf $tmpdir
