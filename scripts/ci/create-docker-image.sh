#!/bin/sh

if [ -z "$REF" ]; then
    echo "The environment variable REF needs to be defined"
    exit 1
fi

set -v
set -e

TAG=`echo $REF | awk -F '/' '{print $NF}'`
VERSION=`echo $TAG | tr -d [a-z]`
DOCKER_IMAGE=skydive/skydive
DOCKER_EMAIL=skydivesoftware@gmail.com
DOCKER_USERNAME=skydiveproject
[ -n "$VERSION" ] && DOCKER_TAG=$VERSION || DOCKER_TAG=latest

make docker-image WITH_EBPF=true DOCKER_IMAGE=${DOCKER_IMAGE} DOCKER_TAG=${DOCKER_TAG}

if [ -n "$DRY_RUN" ]; then
    echo "Running in dry run mode. Exiting."
    exit 0
fi

if [ -z "$DOCKER_PASSWORD" ]; then
    echo "The environment variable DOCKER_PASSWORD needs to be defined"
    exit 1
fi

if [ -n "${DOCKER_USERNAME}" ] && [ -n "${DOCKER_PASSWORD}" ]; then
    sudo docker login -e "${DOCKER_EMAIL}" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}"
    sudo docker tag ${DOCKER_IMAGE}:${DOCKER_TAG} ${DOCKER_IMAGE}:latest
    sudo docker push ${DOCKER_IMAGE}:${DOCKER_TAG}
    sudo docker push ${DOCKER_IMAGE}:latest
fi
