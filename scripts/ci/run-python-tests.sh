#!/bin/bash

set -v

dir="$(dirname "$0")"
. "${dir}/install-go.sh"
. "${dir}/install-static-requirements.sh"

cd ${GOPATH}/src/github.com/skydive-project/skydive

set -e

make docker-image

cd contrib/python

virtualenv-3 venv
source venv/bin/activate

pip install pycodestyle flake8
pip install -r api/requirements.txt

cd api

python setup.py install

pycodestyle
flake8

python3 -m unittest discover tests
