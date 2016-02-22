#!/bin/bash

set -e

function usage {
  echo "Usage: $0 [-d] [-p] tag"
}

command -v github-release
if [ $? -ne 0 ]; then
  go get github.com/aktau/github-release
fi

while getopts ":dpu:" opt; do
  case $opt in
    u)
      org=$OPTARG
      ;;
    d)
      release_opts="$release_opts --draft"
      ;;
    p)
      release_opts="$release_opts --pre-release"
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      usage
      exit 1
      ;;
  esac
done

org=${org-redhat-cip}
tag=${@:$OPTIND:1}

if [ -z "$tag" ]; then
  usage
  exit 1
fi

# Create a temporary GOPATH
export GOPATH=`mktemp -d`
mkdir -p $GOPATH/src/github.com/redhat-cip

# Get the Skydive sources
git clone https://github.com/$org/skydive $GOPATH/src/github.com/redhat-cip/skydive

cd $GOPATH/src/github.com/redhat-cip/skydive
git checkout $tag || (echo "Unknown tag $tag"; exit 1)

# Run the test suite
make test GOFLAGS="-race -timeout 6m"

# Build the executables
make install

# Create the release
github-release release --user $org --repo skydive --tag $tag $release_opts --description "Skydive $tag"

# Upload the executable
github-release upload --user $org --repo skydive --tag $tag --name skydive --file $GOPATH/bin/skydive
