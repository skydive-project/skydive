#!/bin/bash

set -v

dir="$(dirname "$0")"

. "${dir}/install-go.sh"

cd ${GOPATH}/src/github.com/redhat-cip/skydive
make install

git remote add github git@github.com:redhat-cip/skydive.git
git fetch github
git checkout -b nightly-builds github/nightly-builds

git config --global user.email "skydivesoftware@gmail.com"
git config --global user.name "Nightly Builds Bot"

git lfs install
git lfs track "*/skydive"

DIR=`date +%Y-%m-%d`
mkdir ${DIR}
mv ${GOPATH}/bin/skydive ${DIR}/
git add ${DIR}/skydive
[ -e latest ] && unlink latest
ln -s $DIR latest
git add latest
git commit -m "${DIR} nightly build"
git push github nightly-builds
