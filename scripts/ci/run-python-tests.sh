#!/bin/bash

set -v

dir="$(dirname "$0")"

cd ${GOPATH}/src/github.com/skydive-project/skydive

set -e

# make docker-image

docker_setup() {
    echo docker run --name skydive-docker-python-tests -p 8082:8082 -d $* skydive/skydive:devel allinone
    docker run --name skydive-docker-python-tests -p 8082:8082 -d $* skydive/skydive:devel allinone
    sleep 5
}

docker_cleanup() {
    docker stop skydive-docker-python-tests
    docker rm -f skydive-docker-python-tests
}

cd contrib/python

echo "Python2 tests"
virtualenv -p python2 venv2
source venv2/bin/activate

pip install -r api/requirements.txt

pushd api
python setup.py install

docker_setup && trap docker_cleanup EXIT

python -m unittest discover tests

popd

deactivate

echo "Python3 tests"
virtualenv -p python3 venv3
source venv3/bin/activate

pip install black
pip install -r api/requirements.txt

pushd api
python setup.py install

black skydive

python -m unittest discover tests

docker_cleanup

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

docker_setup -v $CONF:/etc/skydive.yml -v $PASSWD:/etc/skydive.htpasswd

export SKYDIVE_PYTHON_TESTS_USERPASS="admin:pass"
python -m unittest discover tests
unset SKYDIVE_PYTHON_TESTS_USERPASS

docker_cleanup

rm -f "$CONF" "$PASSWD"

echo "=== python SSL test"
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
commonName_max = 64

[ v3_req ]
# Extensions to add to a certificate request
basicConstraints = CA:TRUE
keyUsage = digitalSignature, keyEncipherment, keyCertSign
extendedKeyUsage = serverAuth,clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

CERT_DIR=$(mktemp -d /tmp/skydive-ssl.XXXXXX)
openssl genrsa -out $CERT_DIR/rootCA.key 4096
chmod 400 $CERT_DIR/rootCA.key
yes '' | openssl req -x509 -new -nodes -key $CERT_DIR/rootCA.key -days 365 -out $CERT_DIR/rootCA.crt
chmod 444 $CERT_DIR/rootCA.crt

openssl genrsa -out $CERT_DIR/analyzer.key 2048
chmod 400 $CERT_DIR/analyzer.key

yes '' | openssl req -new -key $CERT_DIR/analyzer.key -out $CERT_DIR/analyzer.csr -subj "/CN=analyzer" -config $CONF_SSL
openssl x509 -req -days 365 -in $CERT_DIR/analyzer.csr -CA $CERT_DIR/rootCA.crt -CAkey $CERT_DIR/rootCA.key -CAcreateserial -out $CERT_DIR/analyzer.crt -extfile $CONF_SSL -extensions v3_req
chmod 444 $CERT_DIR/analyzer.crt

CONF=$(mktemp /tmp/skydive.yml.XXXXXX)
cat <<EOF > "$CONF"
tls:
  ca_cert: /etc/skydive.ca.crt
  client_cert: /etc/skydive.analyzer.crt
  client_key: /etc/skydive.analyzer.key
  server_cert: /etc/skydive.analyzer.crt
  server_key: /etc/skydive.analyzer.key

agent:
  topology:
    probes:
      - ovsdb
      - docker

analyzer:
  listen: 0.0.0.0:8082

analyzers:
  - 127.0.0.1:8082
EOF

docker_setup -v $CONF:/etc/skydive.yml \
             -v $CERT_DIR/rootCA.crt:/etc/skydive.ca.crt \
             -v $CERT_DIR/analyzer.crt:/etc/skydive.analyzer.crt \
             -v $CERT_DIR/analyzer.key:/etc/skydive.analyzer.key

export SKYDIVE_PYTHON_TESTS_TLS="True"
export SKYDIVE_PYTHON_TESTS_CERTIFICATES="$CERT_DIR/rootCA.crt:$CERT_DIR/analyzer.crt:$CERT_DIR/analyzer.key"

python -m unittest discover tests

unset SKYDIVE_PYTHON_TESTS_CERTIFICATES
unset SKYDIVE_PYTHON_TESTS_TLS

rm -rf "$CERT_DIR" "$CONF"

popd
