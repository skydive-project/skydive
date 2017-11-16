#!/bin/bash

set -v

function usage {
  echo "Usage: $0 [-b|-s|-a|-l] [-r tag_or_commit]"
}

from=HEAD
build_opts=-ba
target=dist

while getopts ":asblr:" opt; do
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
    l)
      target="localdist"
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
    define="commit ${version:0:12}"
fi

set -e

gitdir=$(cd "$(dirname "$0")/../../.."; pwd)
rpmbuilddir=$gitdir/rpmbuild

mkdir -p $rpmbuilddir/SOURCES
mkdir -p $rpmbuilddir/SPECS
make -C $gitdir $target DESTDIR=$rpmbuilddir/SOURCES
$(dirname "$0")/specfile-update-bundles $gitdir/vendor/vendor.json $gitdir/contrib/packaging/rpm/skydive.spec > $rpmbuilddir/SPECS/skydive.spec
echo rpmbuild --nodeps $build_opts --undefine dist --define "$define" --define "_topdir $rpmbuilddir" $rpmbuilddir/SPECS/skydive.spec
rpmbuild --nodeps $build_opts --undefine dist --define "$define" --define "_topdir $rpmbuilddir" $rpmbuilddir/SPECS/skydive.spec
