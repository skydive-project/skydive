#!/bin/sh

# Must be provided by Jenkins credentials plugin:
# GITHUB_USERNAME
# GITHUB_TOKEN
# DOCKER_PASSWORD
# COPR_LOGIN
# COPR_TOKEN

if [ -z "$GITHUB_USERNAME" ] || [ -z "$GITHUB_TOKEN" ] ||
   [ -z "$DOCKER_PASSWORD" ] || [ -z "$COPR_LOGIN" ] ||
   [ -z "$COPR_TOKEN" ]
then
    echo "The environment variables GITHUB_USERNAME, GITHUB_TOKEN, DOCKER_PASSWORD, COPR_LOGIN and COPR_TOKEN need to be defined"
    exit 1
fi

set -v

export TAG=`echo $ZUUL_REF | awk -F '/' '{print $NF}'`
export VERSION=`echo $TAG | tr -d [a-z]`
export DOCKER_IMAGE=skydive/skydive
export DOCKER_EMAIL=skydivesoftware@gmail.com
export DOCKER_USERNAME=skydiveproject
export DOCKER_TAG=$VERSION
export BINARIES_REPO=https://github.com/skydive-project/skydive-binaries.git
export COPR_USERNAME=skydive

dir="$(dirname "$0")"

. "${dir}/install-go.sh"
. "${dir}/install-requirements.sh"
. "${dir}/install-static-requirements.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive

echo "--- DOCKER IMAGE ---"
make docker-image DOCKER_IMAGE=${DOCKER_IMAGE} DOCKER_TAG=${DOCKER_TAG}
sudo docker login -e "${DOCKER_EMAIL}" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}"
sudo docker tag ${DOCKER_IMAGE}:${DOCKER_TAG} ${DOCKER_IMAGE}:latest
sudo docker push ${DOCKER_IMAGE}:${DOCKER_TAG}
sudo docker push ${DOCKER_IMAGE}:latest

echo "--- BINARIES ---"
rev=`git rev-parse HEAD`
git remote add binaries ${BINARIES_REPO}
git fetch binaries
git checkout -b travis-builds binaries/travis-builds
git config --global user.email "skydivesoftware@gmail.com"
git config --global user.name "Skydive Build Bot"
mkdir ${TAG}
cp ${GOPATH}/bin/skydive ${TAG}/
git add ${TAG}/skydive
test -L latest && unlink latest
ln -s ${TAG} latest
git add latest
git commit -m "${TAG} SF build"
git config credential.helper "store --file=.git/credentials"
echo "https://${GITHUB_USERNAME}:${GITHUB_TOKEN}@github.com" > .git/credentials
git push -f -q binaries travis-builds
git checkout -f $rev

echo "--- COPR ---"
sudo dnf -y install copr-cli rpm-build
mkdir -p ~/.config
cat > ~/.config/copr <<EOF
[copr-cli]
username = skydive
login = $COPR_LOGIN
token = $COPR_TOKEN
copr_url = https://copr.fedorainfracloud.org
EOF

contrib/packaging/rpm/generate-skydive-bootstrap.sh -s -r ${TAG}
copr build skydive/skydive rpmbuild/SRPMS/skydive-${VERSION}-1.src.rpm

echo "--- GITHUB RELEASE ---"
changelog=$(scripts/ci/extract-changelog.py CHANGELOG.md $VERSION)
go get github.com/aktau/github-release
github-release release --user skydive-project --repo skydive --tag ${TAG} --description "$changelog"
github-release upload --user skydive-project --repo skydive --tag ${TAG} --name skydive --file $GOPATH/bin/skydive
github-release upload --user skydive-project --repo skydive --tag ${TAG} --name skydive --file rpmbuild/SOURCES/skydive-${VERSION}.tar.gz
