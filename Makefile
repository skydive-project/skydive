# really Basic Makefile for Skydive

PROTO_FILES=flow/flow.proto
GOFLAGS?=-v

.proto: godep builddep ${PROTO_FILES}
	protoc --go_out . ${PROTO_FILES}

.bindata: godep builddep
	go-bindata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/

all: genlocalfiles
	godep go install ${GOFLAGS} ./...

install: godep
	godep go install ${GOFLAGS} ./...

build: godep
	godep go build ${GOFLAGS} ./...

test: godep
	godep go test ${GOFLAGS} ./...

godep:
	go get github.com/tools/godep

fmt:
	@echo "+ $@"
	@test -z "$$(gofmt -s -l . | grep -v Godeps/_workspace/src/ | grep -v statics/bindata.go | tee /dev/stderr)" || \
		echo "+ please format Go code with 'gofmt -s'"

lint:
	@echo "+ $@"
	@test -z "$$(golint ./... | grep -v Godeps/_workspace/src/ | tee /dev/stderr)"

# dependency package need for building the project
builddep:
	go get github.com/golang/protobuf/proto
	go get github.com/golang/protobuf/protoc-gen-go
	go get github.com/jteeuwen/go-bindata/...

genlocalfiles: .proto .bindata
