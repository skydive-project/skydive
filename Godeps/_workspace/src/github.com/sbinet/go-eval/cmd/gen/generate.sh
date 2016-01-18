#!/bin/bash
set -e
go build gen.go
./gen > expr1.go
gofmt -w expr1.go
