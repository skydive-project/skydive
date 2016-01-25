# really Basic Makefile for Skydive

PROTO_FILES=flow/flow.proto

.proto: godep builddep ${PROTO_FILES}
	protoc --go_out . ${PROTO_FILES}

.bindata: godep builddep
	go-bindata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/

all: genlocalfiles
	godep go install -v ./...

install: godep
	godep go install -v ./...

build: godep
	godep go build -v ./...

godep:
	go get github.com/tools/godep

# dependency package need for building the project
builddep:
	go get github.com/golang/protobuf/proto
	go get github.com/golang/protobuf/protoc-gen-go
	go get github.com/jteeuwen/go-bindata/...

genlocalfiles: .proto .bindata
