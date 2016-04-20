# really Basic Makefile for Skydive

PROTO_FILES=flow/flow.proto
VERBOSE_FLAGS?=-v
VERBOSE?=true
ifeq ($(VERBOSE), false)
	VERBOSE_FLAGS:=
endif
TIMEOUT?=6m
UT_PACKAGES=$(shell go list ./... | grep -v '/tests')

.proto: godep builddep ${PROTO_FILES}
	protoc --go_out . ${PROTO_FILES}

.bindata: godep builddep
	go-bindata -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/*

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
	godep go test ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} ${UT_PACKAGES}

godep:
	go get github.com/tools/godep

fmt:
	@echo "+ $@"
	@test -z "$$(gofmt -s -l . | grep -v Godeps/_workspace/src/ | grep -v statics/bindata.go | tee /dev/stderr)" || \
		echo "+ please format Go code with 'gofmt -s'"

ineffassign interfacer golint goimports varcheck structcheck aligncheck deadcode gotype errcheck gocyclo dupl:
	@go get github.com/alecthomas/gometalinter
	@command -v $@ >/dev/null || gometalinter --install --update

gometalinter: ineffassign interfacer golint goimports varcheck structcheck aligncheck deadcode gotype errcheck gocyclo dupl

lint: gometalinter
	@echo "+ $@"
	@gometalinter --disable=gotype --skip=Godeps/... --skip=statics/... --deadline 5m --sort=path ./...  2>&1 | grep -v -e 'error return value not checked' -e 'should have comment or be unexported' -e 'declaration of err shadows declaration'

# dependency package need for building the project
builddep:
	go get github.com/golang/protobuf/proto
	go get github.com/golang/protobuf/protoc-gen-go
	go get github.com/jteeuwen/go-bindata/...

genlocalfiles: .proto .bindata

clean:
	grep ImportPath Godeps/Godeps.json | perl -pe 's|.*": "(.*?)".*|\1|g' | xargs -n 1 go clean -i >/dev/null 2>&1 || true
	rm -rf Godeps/_workspace/pkg
