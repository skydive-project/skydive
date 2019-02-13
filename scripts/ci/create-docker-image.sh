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

DOCKER_DIR=contrib/docker
GOVENDOR_VOL=govendor-cache
GOVENDOR_DIR=/root/go/.cache/govendor
GOBUILD_VOL=gobuild-cache
GOBUILD_DIR=/root/.cache/go-build
TOPLEVEL_VOL=$PWD
TOPLEVEL_DIR=/root/go/src/github.com/skydive-project/skydive

docker_tag() {
    local arch=$1
    echo ${DOCKER_TAG}-linux-${arch}
}

docker_skydive_builder() {
    local arch=$1
    local dockerfile=$2

    # create docker image of builder and build skydive
    local tag=skydive-compile
    local image=skydive-compile-build
    local uid=$( id -u )
    docker build -t $tag \
        ${TARGET_ARCH:+--build-arg TARGET_ARCH=${TARGET_ARCH}} \
        ${TARGET_GOARCH:+--build-arg TARGET_GOARCH=${TARGET_GOARCH}} \
        ${DEBARCH:+--build-arg DEBARCH=${DEBARCH}} \
        --build-arg UID=$uid \
        -f $DOCKER_DIR/$dockerfile $DOCKER_DIR
    docker volume create $GOVENDOR_VOL
    docker volume create $GOBUILD_VOL
    docker rm $image || true
    docker run --name $image \
        --env UID=$uid \
        --volume $TOPLEVEL_VOL:$TOPLEVEL_DIR \
        --volume $GOVENDOR_VOL:$GOVENDOR_DIR \
        --volume $GOBUILD_VOL:$GOBUILD_DIR \
        $tag

    # copy skydive executable our of builder docker image
    local src=/root/go/bin/${TARGET_GOARCH:+linux_${TARGET_GOARCH}/}skydive
    local dst=$DOCKER_DIR/skydive.$arch
    docker cp $image:$src $dst
    docker rm $image
}

docker_skydive_target() {
    local arch=$1
    local dockerfile=$2

    # build target skydive docker image
    local tag=$( docker_tag ${arch} )
    docker build -t ${DOCKER_IMAGE}:$tag \
        --label "Version=${VERSION}" \
        --build-arg ARCH=$arch \
        ${BASE:+--build-arg BASE=${BASE}} \
        -f $DOCKER_DIR/$dockerfile $DOCKER_DIR
}

docker_native_build() {
    local arch=$1

    docker_skydive_builder $arch Dockerfile.compile
    docker_skydive_target $arch Dockerfile
}

docker_cross_build() {
    local arch=$1

    docker_skydive_builder $arch Dockerfile.crosscompile
    docker_skydive_target $arch Dockerfile.static
}

docker_build() {
    for arch in $ARCHES
    do
        case $arch in
          amd64)
            docker_native_build $arch
            ;;
          ppc64le)
            TARGET_ARCH=powerpc64le TARGET_GOARCH=$arch DEBARCH=ppc64el BASE=${BASE:-${arch}/ubuntu} docker_cross_build $arch
            ;;
          arm64)
            TARGET_ARCH=aarch64 TARGET_GOARCH=$arch DEBARCH=$arch BASE=${BASE:-aarch64/ubuntu} docker_cross_build $arch
            ;;
          s390x)
            TARGET_ARCH=$arch TARGET_GOARCH=$arch DEBARCH=$arch BASE=${BASE:-${arch}/ubuntu} docker_cross_build $arch
            ;;
          *)
            TARGET_ARCH=$arch TARGET_GOARCH=$arch DEBARCH=$arch BASE=${BASE:-${arch}/ubuntu} docker_cross_build $arch
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

docker_image() {
    local arch=$1
    echo ${DOCKER_IMAGE}:$( docker_tag ${arch} )
}

docker_inspect() {
    local arch=$1
    docker inspect --format='{{index .RepoDigests 0}}' $( docker_image ${arch} )
}

docker_push() {
    for arch in $ARCHES
    do
        docker push $( docker_image ${arch} )
    done
}

docker_manifest() {
    digests=""
    for arch in $ARCHES
    do
        digest=$( docker_inspect ${arch} )
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
        digest=$( docker_inspect ${arch} )
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
