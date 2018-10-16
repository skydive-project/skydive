#!/bin/sh

set -v
set -x
set -e

# Arches are the docker arch names
: ${ARCHES:=amd64 ppc64le s390x}
# arm64 waiting on  golang 1.11 (https://github.com/skydive-project/skydive/pull/1188#discussion_r204336060)
: ${DOCKER_IMAGE:=skydive/skydive}
: ${DOCKER_USERNAME:=skydiveproject}
: ${REF:=latest}

TAG=${REF##*/}
VERSION=${TAG#v}

[ -n "$VERSION" ] && DOCKER_TAG=$VERSION || DOCKER_TAG=latest

# See if a server forms part of DOCKER_IMAGE, e.g. DOCKER_IMAGE=registry.ng.bluemix.net:8080/skydive/skydive
if [ "${DOCKER_IMAGE%/*/*}" != "${DOCKER_IMAGE}" ]; then
    DOCKER_SERVER=${DOCKER_IMAGE%/*/*}
fi

if [ -n "$PUSH_RUN" ]; then
    echo "Running in push run mode. Skipping build."
else
    for arch in $ARCHES
    do
        case $arch in
          amd64)
            # x86_64 image
            make docker-build DOCKER_IMAGE=${DOCKER_IMAGE} DOCKER_TAG=amd64-${DOCKER_TAG} ${BASE:+BASE=$BASE}
            ;;
          ppc64le)
            make docker-cross-build WITH_EBPF=true TARGET_ARCH=powerpc64le TARGET_GOARCH=ppc64le DEBARCH=ppc64el DOCKER_IMAGE=${DOCKER_IMAGE} BASE=${BASE:-${arch}/ubuntu} DOCKER_TAG=$arch-${DOCKER_TAG}
            ;;
          arm64)
            make docker-cross-build WITH_EBPF=true TARGET_ARCH=aarch64 TARGET_GOARCH=arm64 DOCKER_IMAGE=${DOCKER_IMAGE} BASE=${BASE:-aarch64/ubuntu} DOCKER_TAG=$arch-${DOCKER_TAG}
            ;;
          s390x)
            make docker-cross-build WITH_EBPF=true TARGET_ARCH=s390x TARGET_GOARCH=s390x DOCKER_IMAGE=${DOCKER_IMAGE} BASE=${BASE:-${arch}/ubuntu} DOCKER_TAG=$arch-${DOCKER_TAG}
            ;;
          *)
            make docker-cross-build WITH_EBPF=true TARGET_ARCH=$arch TARGET_GOARCH=$arch DOCKER_IMAGE=${DOCKER_IMAGE} BASE=${BASE:-${arch}/ubuntu} DOCKER_TAG=$arch-${DOCKER_TAG}
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

digests=""
for arch in $ARCHES
do
    docker push ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG}

    digest=$( docker inspect --format='{{index .RepoDigests 0}}' ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG} )
    digests="${digests} $digest"
done

res=0
for i in {1..6}
do
    docker manifest create --amend "${DOCKER_IMAGE}:${DOCKER_TAG}" ${digests} && break || res=$?
    sleep 10
done
[ $res != 0 ] && exit $res

for arch in $ARCHES
do
    digest=$( docker inspect --format='{{index .Id}}' ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG} )
    docker manifest annotate --arch $arch "${DOCKER_IMAGE}:${DOCKER_TAG}" ${DOCKER_IMAGE}@$digest
done

docker manifest inspect "${DOCKER_IMAGE}:${DOCKER_TAG}"
docker manifest push    "${DOCKER_IMAGE}:${DOCKER_TAG}"
