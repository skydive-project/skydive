#!/bin/bash

set -v

dir="$(dirname "$0")"

cd ${GOPATH}/src/github.com/skydive-project/skydive

set -e

make docker-image

cd contrib/python

echo "Python2 tests"
virtualenv-2 venv2
source venv2/bin/activate

pip install -r api/requirements.txt

pushd api
python setup.py install
python -m unittest discover tests
popd

deactivate


echo "Python3 tests"
virtualenv-3 venv3
source venv3/bin/activate

pip install flake8
pip install -r api/requirements.txt

pushd api
python setup.py install

pycodestyle
flake8

python -m unittest discover tests

echo "=== python auth test"
CONF=$(mktemp /tmp/skydive.yml.XXXXXX)
cat <<EOF > "$CONF"
agent:
  topology:
    probes:
      - ovsdb
      - docker

analyzer:
  listen: 0.0.0.0:8082
  auth:
    api:
      backend: basic

auth:
  basic:
    type: basic
    file: /etc/skydive.htpasswd
EOF
PASSWD=$(mktemp /tmp/skydive.passwd.XXXXXX)
# admin pass
echo 'admin:$apr1$i38WPF8K$0L1TzLo3cI4da8DFvrCLn1' > "$PASSWD"
export SKYDIVE_PYTHON_TESTS_MAPFILE="$CONF:/etc/skydive.yml,$PASSWD:/etc/skydive.htpasswd"
export SKYDIVE_PYTHON_TESTS_USERPASS="admin:pass"
python -m unittest discover tests

unset SKYDIVE_PYTHON_TESTS_MAPFILE
unset SKYDIVE_PYTHON_TESTS_USERPASS
rm -f "$CONF" "$PASSWD"



echo "=== python ssl test"
CONF_SSL=$(mktemp /tmp/skydive-ssl.cnf.XXXXXX)
cat <<EOF > "$CONF_SSL"
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
countryName_default = FR
stateOrProvinceName_default = Paris
localityName_default = Paris
organizationalUnitName_default = Skydive Team
commonName = skydive
commonName_max	= 64

[ v3_req ]
# Extensions to add to a certificate request
basicConstraints = CA:TRUE
keyUsage = digitalSignature, keyEncipherment, keyCertSign
extendedKeyUsage = serverAuth,clientAuth
subjectAltName = @alt_names

[alt_names]
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

CERT_DIR=$(mktemp -d /tmp/skydive-ssl.XXXXXX)
openssl genrsa -out $CERT_DIR/analyzer.key 2048
chmod 400 $CERT_DIR/analyzer.key
yes '' | openssl req -new -key $CERT_DIR/analyzer.key -out $CERT_DIR/analyzer.csr -subj "/CN=analyzer" -config $CONF_SSL
openssl x509 -req -days 365 -signkey $CERT_DIR/analyzer.key -in $CERT_DIR/analyzer.csr -out $CERT_DIR/analyzer.crt -extfile $CONF_SSL -extensions v3_req
chmod 444 $CERT_DIR/analyzer.crt

CONF=$(mktemp /tmp/skydive.yml.XXXXXX)
cat <<EOF > "$CONF"
agent:
  X509_cert: /etc/skydive.analyzer.crt
  X509_key: /etc/skydive.analyzer.key
  topology:
    probes:
      - ovsdb
      - docker

analyzer:
  listen: 0.0.0.0:8082
  X509_cert: /etc/skydive.analyzer.crt
  X509_key: /etc/skydive.analyzer.key
EOF

export SKYDIVE_PYTHON_TESTS_MAPFILE="$CONF:/etc/skydive.yml,$CERT_DIR/analyzer.crt:/etc/skydive.analyzer.crt,$CERT_DIR/analyzer.key:/etc/skydive.analyzer.key"
export SKYDIVE_PYTHON_TESTS_TLS="True"
python -m unittest discover tests

unset SKYDIVE_PYTHON_TESTS_MAPFILE
unset SKYDIVE_PYTHON_TESTS_TLS
rm -rf "$CERT_DIR" "$CONF"


popd
