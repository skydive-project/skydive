#!/bin/bash

set -v

function usage {
  echo "Usage: $0 [-b|-s|-a|-l] [-r tag_or_commit]"
}

gitdir=$(cd "$(dirname "$0")/../../.."; pwd)
rpmbuilddir=$gitdir/rpmbuild

target=dist
from=HEAD
build_opts=-ba

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

version=$(make -C $gitdir -s version)
define="fullver ${version}"

set -e

mkdir -p $rpmbuilddir/SOURCES
mkdir -p $rpmbuilddir/SPECS
make -C $gitdir $target DESTDIR=$rpmbuilddir/SOURCES
$(dirname "$0")/specfile-update-bundles $gitdir/vendor/vendor.json $gitdir/contrib/packaging/rpm/skydive.spec > $rpmbuilddir/SPECS/skydive.spec
set -x
rpmbuild --nodeps $build_opts --undefine dist --define "$define" --define "_topdir $rpmbuilddir" $rpmbuilddir/SPECS/skydive.spec
set +x
rm -rf $rpmbuilddir/BUILD/skydive-$version
