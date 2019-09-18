#!/bin/sh

# Must be provided by Jenkins credentials plugin:
# GITHUB_USERNAME
# GITHUB_TOKEN

if [ -z "$GITHUB_USERNAME" ] || [ -z "$GITHUB_TOKEN" ]
then
    echo "The environment variables GITHUB_USERNAME and GITHUB_TOKEN need to be defined"
    exit 1
fi

set -v
set -e

BUILD_TAG=$(date +%Y-%m-%d).${BUILD_NUMBER}

dir="$(dirname "$0")"

cd ${GOPATH}/src/github.com/skydive-project/skydive

echo "--- BINARIES ---"
make LDFLAGS="-s -w" static WITH_EBPF=true

# We need at least Go 1.11.0 to generate swagger spec
eval "$(gimme 1.11.13)"
make swagger

git reset --hard

cd /tmp
rm -rf skydive-binaries
git clone https://github.com/skydive-project/skydive-binaries.git
cd /tmp/skydive-binaries

git config --global user.email "builds@skydive.network"
git config --global user.name "Skydive CI"

add() {
    local dst=$1
    local src=$2
    cp $src $dst
    git add $dst
}

add_gz() {
    local dst=$1
    local src=$2
    gzip -c $src > $dst.gz
    git add $dst.gz
}

allinone=$GOPATH/src/github.com/skydive-project/skydive/contrib/exporters/allinone

add_gz skydive-latest ${GOPATH}/bin/skydive
add swagger.json ${GOPATH}/src/github.com/skydive-project/skydive/swagger.json
add_gz skydive-flow-exporter $allinone/allinone
add skydive-flow-exporter.yml $allinone/allinone.yml.default

git commit -m "${BUILD_TAG} Jenkins build" --amend --reset-author
git config credential.helper "store --file=.git/credentials"
echo "https://${GITHUB_USERNAME}:${GITHUB_TOKEN}@github.com" > .git/credentials

if [ -n "$DRY_RUN" ]; then
    echo "Running in dry run mode. Exiting."
    exit 0
fi

git push -f -q origin jenkins-builds
