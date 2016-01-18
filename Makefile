# really Basic Makefile for Skydive

PROTO_FILES=flow/flow.proto

.proto: builddep ${PROTO_FILES}
	protoc --go_out . ${PROTO_FILES}

.bindata: builddep
	go-bindata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/

all: .proto .bindata
	godep go install -v ./...

install:
	godep go install -v ./...

build:
	godep go build -v ./...

# dependency package need for building the project
builddep:
	go get github.com/golang/protobuf/proto
	go get github.com/golang/protobuf/protoc-gen-go
	go get github.com/jteeuwen/go-bindata/...
