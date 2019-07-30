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
make static WITH_EBPF=true

# We need at least Go 1.11.0 to generate swagger spec
eval "$(gimme 1.12.7)"
make swagger

git reset --hard

cd /tmp
rm -rf skydive-binaries
git clone https://github.com/skydive-project/skydive-binaries.git
cd /tmp/skydive-binaries

git lfs install

git config --global user.email "builds@skydive.network"
git config --global user.name "Skydive CI"
cp ${GOPATH}/bin/skydive skydive-latest
cp ${GOPATH}/src/github.com/skydive-project/skydive/swagger.json .
git add skydive-latest
git add swagger.json
git commit -m "${BUILD_TAG} Jenkins build" --amend --reset-author
git config credential.helper "store --file=.git/credentials"
echo "https://${GITHUB_USERNAME}:${GITHUB_TOKEN}@github.com" > .git/credentials

if [ -n "$DRY_RUN" ]; then
    echo "Running in dry run mode. Exiting."
    exit 0
fi

git push -f -q binaries jenkins-builds
