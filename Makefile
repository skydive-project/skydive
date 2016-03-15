# really Basic Makefile for Skydive

PROTO_FILES=flow/flow.proto
VERBOSE_FLAGS?=-v
VERBOSE?=true
ifeq ($(VERBOSE), false)
	VERBOSE_FLAGS:=
endif
TIMEOUT?=6m

.proto: godep builddep ${PROTO_FILES}
	protoc --go_out . ${PROTO_FILES}

.bindata: godep builddep
	go-bindata -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/

all: genlocalfiles
	godep go install ${GOFLAGS} ${VERBOSE_FLAGS} ./...

install: godep
	godep go install ${GOFLAGS} ${VERBOSE_FLAGS} ./...

build: godep
	godep go build ${GOFLAGS} ${VERBOSE_FLAGS} ./...

test.functionals: godep
	godep go test ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -c -o tests/functionals ./tests/
ifneq ($(VERBOSE_FLAGS),)
	cd tests && sudo ./functionals -test.v -test.timeout ${TIMEOUT} ${ARGS}
else
	cd tests && sudo ./functionals -test.timeout ${TIMEOUT} ${ARGS}
endif

test: godep
	godep go test ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -run "^tests" ./...

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

clean:
	grep ImportPath Godeps/Godeps.json | perl -pe 's|.*": "(.*?)".*|\1|g' | xargs -n 1 go clean -i >/dev/null 2>&1 || true
	rm -rf Godeps/_workspace/pkg
