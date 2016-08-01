#!/bin/bash

set -v
set -e

dir="$(dirname "$0")"

. "${dir}/install-go.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive
make install

git remote add github git@github.com:skydive-project/skydive-binaries.git
git fetch github
git checkout -b nightly-builds github/nightly-builds

git config --global user.email "skydivesoftware@gmail.com"
git config --global user.name "Nightly Builds Bot"

git lfs install
git lfs track "*/skydive"

# Keep only the last 10 builds
git lfs ls-files -l | awk '{ print $3 }' | tail -n +9 | xargs -r git rm --

# Add the lastest build and creates a symlink to link
DIR=`date +%Y-%m-%d`
mkdir ${DIR}
mv ${GOPATH}/bin/skydive ${DIR}/
git add ${DIR}/skydive
[ -L latest ] && unlink latest
ln -s $DIR latest
git add latest
git commit -m "${DIR} nightly build"
git push github nightly-builds
