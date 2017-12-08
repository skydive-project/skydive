#!/bin/sh

# Must be provided by Jenkins credentials plugin:
# GITHUB_USERNAME
# GITHUB_TOKEN
# DOCKER_PASSWORD

if [ -z "$GITHUB_USERNAME" ] || [ -z "$GITHUB_TOKEN" ] ||
   [ -z "$DOCKER_PASSWORD" ]
then
    echo "The environment variables GITHUB_USERNAME, GITHUB_TOKEN, DOCKER_PASSWORD need to be defined"
    exit 1
fi

set -v
set -e

export DOCKER_IMAGE=skydive/skydive
export DOCKER_EMAIL=skydivesoftware@gmail.com
export DOCKER_USERNAME=skydiveproject
export DOCKER_TAG=latest
export BINARIES_REPO=https://github.com/skydive-project/skydive-binaries.git
export BUILD_TAG=$(date +%Y-%m-%d).${BUILD_NUMBER}

dir="$(dirname "$0")"

. "${dir}/install-go.sh"
. "${dir}/install-static-requirements.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive

echo "--- DOCKER IMAGE ---"
make docker-image WITH_EBPF=true DOCKER_IMAGE=${DOCKER_IMAGE} DOCKER_TAG=${DOCKER_TAG}
sudo docker login -e "${DOCKER_EMAIL}" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}"
sudo docker tag ${DOCKER_IMAGE}:${DOCKER_TAG} ${DOCKER_IMAGE}:latest
sudo docker push ${DOCKER_IMAGE}:${DOCKER_TAG}
sudo docker push ${DOCKER_IMAGE}:latest

echo "--- BINARIES ---"
make static WITH_EBPF=true
git reset --hard
git remote add binaries ${BINARIES_REPO}
git fetch binaries
git checkout -b jenkins-builds binaries/jenkins-builds
git config --global user.email "builds@skydive.network"
git config --global user.name "Skydive CI"
cp ${GOPATH}/bin/skydive skydive-latest
git add skydive-latest
git commit -m "${BUILD_TAG} Jenkins build" --amend --reset-author
git config credential.helper "store --file=.git/credentials"
echo "https://${GITHUB_USERNAME}:${GITHUB_TOKEN}@github.com" > .git/credentials
git push -f -q binaries jenkins-builds
