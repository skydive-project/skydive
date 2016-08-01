libovsdb
========

[![Circle CI](https://circleci.com/gh/socketplane/libovsdb.png?style=badge&circle-token=17838d6362be941ed8478bf9d10de5307d4b917d)](https://circleci.com/gh/socketplane/libovsdb) [![Coverage Status](https://coveralls.io/repos/socketplane/libovsdb/badge.png?branch=master)](https://coveralls.io/r/socketplane/libovsdb?branch=master)

An OVSDB Library written in Go

## What is OVSDB?

OVSDB is the Open vSwitch Database Protocol.
It's defined in [RFC 7047](http://tools.ietf.org/html/rfc7047)
It's used mainly for managing the configuration of Open vSwitch, but it could also be used to manage your stamp collection. Philatelists Rejoice!

## Running the tests

To run only unit tests:

    make test

To run integration tests, you'll need access to docker to run an Open vSwitch container.
Mac users can use [boot2docker](http://boot2docker.io)

    export DOCKER_IP=192.168.59.103
    fig up -d
    make test-all
    fig stop

## Dependency Management

We use [godep](https://github.com/tools/godep) for dependency management with rewritten import paths.
This allows the repo to be `go get`able.

To bump the version of a dependency, follow these [instructions](https://github.com/tools/godep#update-a-dependency)
