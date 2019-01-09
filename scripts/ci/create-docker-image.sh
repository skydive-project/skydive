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

docker_build() {
    for arch in $ARCHES
    do
        flags="DOCKER_IMAGE=${DOCKER_IMAGE} DOCKER_TAG=${arch}-${DOCKER_TAG}"
        case $arch in
          amd64)
            # x86_64 image
            make docker-build ${flags} ${BASE:+BASE=$BASE}
            ;;
          ppc64le)
            make docker-cross-build ${flags} WITH_EBPF=true TARGET_ARCH=powerpc64le TARGET_GOARCH=$arch DEBARCH=ppc64el BASE=${BASE:-${arch}/ubuntu}
            ;;
          arm64)
            make docker-cross-build ${flags} WITH_EBPF=true TARGET_ARCH=aarch64 TARGET_GOARCH=$arch BASE=${BASE:-aarch64/ubuntu}
            ;;
          s390x)
            make docker-cross-build ${flags} WITH_EBPF=true TARGET_ARCH=$arch TARGET_GOARCH=$arch BASE=${BASE:-${arch}/ubuntu}
            ;;
          *)
            make docker-cross-build ${flags} WITH_EBPF=true TARGET_ARCH=$arch TARGET_GOARCH=$arch BASE=${BASE:-${arch}/ubuntu}
            ;;
        esac
    done
}

docker_login() {
    set +x
    if [ -z "$DOCKER_PASSWORD" ]; then
        echo "The environment variable DOCKER_PASSWORD needs to be defined"
        exit 1
    fi

    echo "${DOCKER_PASSWORD}" | docker login  --username "${DOCKER_USERNAME}" --password-stdin ${DOCKER_SERVER}
    set -x
}

docker_push() {
    for arch in $ARCHES
    do
        docker push ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG}
    done
}

docker_manifest() {
    digests=""
    for arch in $ARCHES
    do
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
        digest=$( docker inspect --format='{{index .RepoDigests 0}}' ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG} )
        docker manifest annotate --arch $arch "${DOCKER_IMAGE}:${DOCKER_TAG}" $digest
    done

    docker manifest inspect "${DOCKER_IMAGE}:${DOCKER_TAG}"
    docker manifest push --purge "${DOCKER_IMAGE}:${DOCKER_TAG}"
}

[ -n "$SKIP_BUILD" ] && echo "Skipping build." || docker_build

if [ -n "$DRY_RUN" ]; then
    echo "Running in dry run mode. Exiting."
    exit 0
fi

[ -n "$SKIP_PUSH" ] && echo "Skipping push." || (docker_login; docker_push)
[ -n "$SKIP_MANIFEST" ] && echo "Skipping manifest." || (docker_login; docker_manifest)
