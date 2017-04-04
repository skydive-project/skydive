#!/bin/bash

command -v swift >/dev/null 2>&1 || { echo "You need to have Swift client installed.  Aborting." >&2; exit 1; }
[ -n "$SKYDIVE_TEMP_URL_KEY" ] || { echo "SKYDIVE_TEMP_URL_KEY is undefined.  Aborting." >&2; exit 1; }
[ -e "vendor.tar.gz" ] || { echo "No file vendor.tar.gz was found.  Aborting." >&2; exit 1; }

SWIFT_HOST="http://46.231.132.68:8080"
SWIFT_BASE="/v1/AUTH_0ec9e4f4f3044236b4d18536ccfcb182/skydive/vendor/vendor.tar.gz"
EXPIRE=$(date --date '6 months' +%s)

set +x
echo swift tempurl PUT ${EXPIRE} ${SWIFT_BASE} ${SKYDIVE_TEMP_URL_KEY}
tempurl=$(swift tempurl PUT ${EXPIRE} ${SWIFT_BASE} ${SKYDIVE_TEMP_URL_KEY})
set -x

echo "Uploading vendor.tar.gz"
curl -f -i -X PUT --upload-file vendor.tar.gz "${SWIFT_HOST}${tempurl}"
