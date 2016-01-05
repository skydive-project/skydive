# really Basic Makefile for Skydive

PROTO_FILES=flow/flow.proto

.proto: ${PROTO_FILES}
	protoc --go_out . ${PROTO_FILES}

.bindata:
	go-bindata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/

all: .proto .bindata
	go install -v ./...

build: .proto .bindata
	go build -v ./...
