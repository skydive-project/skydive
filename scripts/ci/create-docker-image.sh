#!/bin/sh

if [ -z "$REF" ]; then
    echo "The environment variable REF needs to be defined"
    exit 1
fi

set -v
set -e

ARCHES="amd64 ppc64le"
TAG=`echo $REF | awk -F '/' '{print $NF}'`
VERSION=`echo $TAG | tr -d [a-z]`
DOCKER_IMAGE=skydive/skydive
DOCKER_EMAIL=skydivesoftware@gmail.com
DOCKER_USERNAME=skydiveproject
[ -n "$VERSION" ] && DOCKER_TAG=$VERSION || DOCKER_TAG=latest

function cleanup {
    docker rm -f skydive-crosscompile
}

function cross_compile() {
    docker pull ubuntu:18.04
    docker run -tid --name skydive-crosscompile -v `pwd`:/root/go/src/github.com/skydive-project/skydive -v $GOPATH/.cache/govendor:/root/go/.cache/govendor ubuntu:18.04 /bin/bash
    trap cleanup ERR
    docker exec ${http_proxy:+--env http_proxy=$http_proxy} skydive-crosscompile /root/go/src/github.com/skydive-project/skydive/scripts/ci/create-docker-multiarch-image.sh $1 $2 $3
    docker build -t ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG} -f contrib/docker/Dockerfile.${arch} contrib/docker/
    docker rm -f skydive-crosscompile
}

for arch in $ARCHES
do
  case $arch in
    amd64)
      # x86_64 image
      make docker-image WITH_EBPF=true DOCKER_IMAGE=${DOCKER_IMAGE} DOCKER_TAG=amd64-${DOCKER_TAG}
      ;;
    ppc64le)
      cross_compile powerpc64le ppc64le ppc64el
      ;;
    \?)
      echo "Unsupported architecture $arch" >&2
      exit 1
      ;;
  esac
done

if [ -n "$DRY_RUN" ]; then
    echo "Running in dry run mode. Exiting."
    exit 0
fi

if [ -z "$DOCKER_PASSWORD" ]; then
    echo "The environment variable DOCKER_PASSWORD needs to be defined"
    exit 1
fi

docker login -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}"

for arch in $ARCHES
do
    docker tag ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG} ${DOCKER_IMAGE}:${arch}-latest
    docker push ${DOCKER_IMAGE}:${arch}-${DOCKER_TAG}
    [ -z "${platforms}" ] && platforms=linux/$arch || platforms=${platforms},linux/$arch
done

manifest-tool --debug push from-args --platforms $platforms --template skydive/skydive:ARCH-latest --target skydive/skydive:latest

token=$(curl -X POST \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d "{\"username\":\"${DOCKER_USERNAME}\",\"password\":\"${DOCKER_PASSWORD}\"}" \
  https://hub.docker.com/v2/users/login/ | jq .token | tr -d '"')

for arch in $ARCHES
do
    curl -i -X DELETE \
      -H "Accept: application/json" \
      -H "Authorization: JWT $token" \
      https://hub.docker.com/v2/repositories/skydive/skydive/tags/${arch}-${DOCKER_TAG}/
done
