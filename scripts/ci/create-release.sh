#!/bin/bash

dir="$(dirname "$0")"

# Must be provided by Jenkins credentials plugin:
# GITHUB_USERNAME
# GITHUB_TOKEN

if [ -z "$REF" ] || [ -z "$GOPATH" ] || [ -z "$GITHUB_USERNAME" ] || [ -z "$GITHUB_TOKEN" ];
then
    echo "The environment variables REF, GOPATH, GITHUB_USERNAME and GITHUB_TOKEN need to be defined"
    exit 1
fi

user=skydive-project
repo=skydive

cleanup() {
    git push --delete origin $TAG || true
    github-release delete --user $user --repo $repo --tag ${TAG} || true
}

cleanup_on_error() {
    local retcode=$?
    if [ $retcode != 0 ]; then
        cleanup
        exit $retcode
    fi
}

release() {
    local description="$1"
    github-release release ${FLAGS} --user $user --repo $repo --tag ${TAG} --description "$description"
    cleanup_on_error
}

upload() {
    local name=$1
    local file=$2
    github-release upload --user $user --repo $repo --tag ${TAG} --name $name --file $file
    cleanup_on_error
}

go get github.com/aktau/github-release
cd ${GOPATH}/src/github.com/skydive-project/skydive

if [ -n "$DRY_RUN" ]; then
    echo "Running in dry run mode. Creating draft release."
    TAG=v0.99.99
    VERSION=0.99.99
    CHANGELOG_VERSION=latest
    FLAGS=--draft
    git config credential.helper "store --file=.git/credentials"
    echo "https://${GITHUB_USERNAME}:${GITHUB_TOKEN}@github.com" > .git/credentials
    git tag $TAG
    git push origin $TAG
else
    TAG=`echo $REF | awk -F '/' '{print $NF}'`
    VERSION=`echo $TAG | tr -d [a-z]`
    CHANGELOG_VERSION=$VERSION
fi

set -v

changelog=$(scripts/ci/extract-changelog.py CHANGELOG.md $CHANGELOG_VERSION)
make static WITH_EBPF=true
${dir}/../../contrib/packaging/rpm/generate-skydive-bootstrap.sh -s -r ${TAG}

release "$changelog"

upload skydive $GOPATH/bin/skydive
upload skydive-${VERSION}.tar.gz rpmbuild/SOURCES/skydive-${VERSION}.tar.gz

if [ -n "$DRY_RUN" ]; then
    cleanup
fi
