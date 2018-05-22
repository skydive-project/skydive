#!/bin/sh

if [ -z "$REF" ] || [ -z "$COPR_LOGIN" ] || [ -z "$COPR_TOKEN" ]; then
    echo "The environment variables REF, COPR_LOGIN and COPR_TOKEN need to be defined"
    exit 1
fi

set -v
set -e

TAG=`echo $REF | awk -F '/' '{print $NF}'`
VERSION=`echo $TAG | tr -d [a-z]`
[ -z "$VERSION" ] && TAG=HEAD || true
COPR_USERNAME=skydive

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

if [ -n "$DRY_RUN" ]; then
    echo "Running in dry run mode. Do not build package."
    exit 0
fi

copr build skydive/skydive rpmbuild/SRPMS/skydive-${VERSION}-1.src.rpm
