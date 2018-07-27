#!/bin/sh

if [ -z "$REF" ]; then
    echo "The environment variable REF needs to be defined"
    exit 1
fi

set -v
set -x
set -e

# Arches are the docker arch names
: ${ARCHES:=amd64 ppc64le s390x}
# arm64 waiting on  golang 1.11 (https://github.com/skydive-project/skydive/pull/1188#discussion_r204336060)
: ${DOCKER_IMAGE:=skydive/skydive}
: ${DOCKER_USERNAME:=skydiveproject}

TAG=${REF##*/}
VERSION=${TAG//[a-z]/}

[ -n "$VERSION" ] && DOCKER_TAG=$VERSION || DOCKER_TAG=latest

# See if a server forms part of DOCKER_IMAGE, e.g. DOCKER_IMAGE=registry.ng.bluemix.net:8080/skydive/skydive
if [[ "$DOCKER_IMAGE" =~ /[^/]*/[^/]* ]]; then
    DOCKER_SERVER=${DOCKER_IMAGE%/[^/]*/[^/]*}
fi

if [ -n "$PUSH_RUN" ]; then
    echo "Running in push run mode. Skipping build."
else
    for arch in $ARCHES
    do
        case $arch in
          amd64)
            # x86_64 image
            make docker-build DOCKER_IMAGE=${DOCKER_IMAGE} DOCKER_TAG=amd64-${DOCKER_TAG}
            ;;
          ppc64le)
            make docker-cross-build TARGET_ARCH=powerpc64le TARGET_GOARCH=ppc64le DEBARCH=ppc64el DOCKER_IMAGE=${DOCKER_IMAGE} BASE=ppc64le/centos DOCKER_TAG=$arch-${DOCKER_TAG}
            ;;
          arm64)
            make docker-cross-build TARGET_ARCH=aarch64 TARGET_GOARCH=arm64 DOCKER_IMAGE=${DOCKER_IMAGE} BASE=aarch64/alpine DOCKER_TAG=$arch-${DOCKER_TAG}
            ;;
          s390x)
            make docker-cross-build TARGET_ARCH=s390x TARGET_GOARCH=s390x DOCKER_IMAGE=${DOCKER_IMAGE} BASE=s390x/clefos DOCKER_TAG=$arch-${DOCKER_TAG}
            ;;
          *)
            make docker-cross-build TARGET_ARCH=$arch TARGET_GOARCH=$arch DOCKER_IMAGE=${DOCKER_IMAGE} BASE=scratch DOCKER_TAG=$arch-${DOCKER_TAG}
            ;;
        esac
    done
fi

if [ -n "$DRY_RUN" ]; then
    echo "Running in dry run mode. Exiting."
    exit 0
fi

set +x
if [ -z "$DOCKER_PASSWORD" ]; then
    echo "The environment variable DOCKER_PASSWORD needs to be defined"
    exit 1
fi

echo "${DOCKER_PASSWORD}" | docker login  --username "${DOCKER_USERNAME}" --password-stdin ${DOCKER_SERVER}
set -x

platforms=""
for arch in $ARCHES
do
    docker push ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG}
    platforms="${platforms} ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG}"
done

docker manifest create --amend "${DOCKER_IMAGE}:${DOCKER_TAG}" ${platforms}

for arch in $ARCHES
do
    docker manifest annotate --arch $arch "${DOCKER_IMAGE}:${DOCKER_TAG}" ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG}
done

docker manifest inspect "${DOCKER_IMAGE}:${DOCKER_TAG}"
docker manifest push    "${DOCKER_IMAGE}:${DOCKER_TAG}"
