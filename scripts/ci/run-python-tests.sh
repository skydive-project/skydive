#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"
. "${dir}/install-static-requirements.sh"

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

pip install pycodestyle flake8
pip install -r api/requirements.txt

pushd api
python setup.py install

pycodestyle
flake8

python -m unittest discover tests
popd
