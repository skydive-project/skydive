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
make ansible-tarball
${dir}/../../contrib/packaging/rpm/generate-skydive-bootstrap.sh -s -r ${TAG}

function cleanup {
    git push --delete origin $TAG
    github-release delete --user skydive-project --repo skydive --tag ${TAG}
}
trap cleanup ERR

github-release release ${FLAGS} --user skydive-project --repo skydive --tag ${TAG} --description "$changelog"
github-release upload --user skydive-project --repo skydive --tag ${TAG} --name skydive --file $GOPATH/bin/skydive
github-release upload --user skydive-project --repo skydive --tag ${TAG} --name skydive-${VERSION}.tar.gz --file rpmbuild/SOURCES/skydive-${VERSION}.tar.gz
github-release upload --user skydive-project --repo skydive --tag ${TAG} --name skydive-ansible-${VERSION}.tar.gz --file rpmbuild/SOURCES/skydive-${VERSION}.tar.gz

if [ -n "$DRY_RUN" ]; then
    cleanup
fi
