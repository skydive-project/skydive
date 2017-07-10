VERSION?=0.11.0

# really Basic Makefile for Skydive
export GO15VENDOREXPERIMENT=1

FLOW_PROTO_FILES=flow/flow.proto flow/set.proto flow/request.proto
FILTERS_PROTO_FILES=filters/filters.proto
VERBOSE_FLAGS?=-v
VERBOSE?=true
ifeq ($(VERBOSE), false)
  VERBOSE_FLAGS:=
endif
ifeq ($(COVERAGE), true)
  COVERAGE_ARGS:= -test.coverprofile=../functionals.cover
endif
TIMEOUT?=1m
TEST_PATTERN?=
UT_PACKAGES?=$(shell ${GOPATH}/bin/govendor list -no-status +local | grep -v '/tests')
FUNC_TESTS_CMD:="grep -e 'func Test${TEST_PATTERN}' tests/*.go | perl -pe 's|.*func (.*?)\(.*|\1|g' | shuf"
FUNC_TESTS:=$(shell sh -c $(FUNC_TESTS_CMD))
DOCKER_IMAGE?=skydive/skydive
DOCKER_TAG?=devel
DESTDIR?=$(shell pwd)
COVERAGE?=0
COVERAGE_MODE?=atomic

.PHONY: all
all: install

.proto: builddep ${FLOW_PROTO_FILES} ${FILTERS_PROTO_FILES}
	protoc --go_out . ${FLOW_PROTO_FILES}
	protoc --go_out . ${FILTERS_PROTO_FILES}
	# always export flow.ParentUUID as we need to store this information to know
	# if it's a Outer or Inner packet.
	sed -e 's/ParentUUID\(.*\),omitempty\(.*\)/ParentUUID\1\2/' -e 's/int64\(.*\),omitempty\(.*\)/int64\1\2/' -i flow/flow.pb.go

.bindata: builddep
	go-bindata ${GO_BINDATA_FLAGS} -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/* statics/css/images/* statics/js/vendor/* statics/js/components/*
	gofmt -w -s statics/bindata.go

skydive-bash-completion.sh: .proto govendor cmd/completion/completion.go
	go run cmd/completion/completion.go

.compile:
	${GOPATH}/bin/govendor install ${GOFLAGS} ${VERBOSE_FLAGS} +local

install: govendor genlocalfiles .compile

build: govendor genlocalfiles
	${GOPATH}/bin/govendor build ${GOFLAGS} ${VERBOSE_FLAGS} +local

static: govendor genlocalfiles
	rm -f $$GOPATH/bin/skydive
	test -f /etc/redhat-release && govendor install -tags netgo --ldflags '-extldflags "-static /usr/lib64/libz.a /usr/lib64/liblzma.a /usr/lib64/libm.a"' ${VERBOSE_FLAGS} +local || true
	test -f /etc/debian_version && govendor install -tags netgo --ldflags '-extldflags "-static /usr/lib/x86_64-linux-gnu/libz.a /usr/lib/x86_64-linux-gnu/liblzma.a /usr/lib/x86_64-linux-gnu/libicuuc.a /usr/lib/x86_64-linux-gnu/libicudata.a /usr/lib/x86_64-linux-gnu/libxml2.a /usr/lib/x86_64-linux-gnu/libc.a /usr/lib/x86_64-linux-gnu/libdl.a /usr/lib/x86_64-linux-gnu/libpthread.a /usr/lib/x86_64-linux-gnu/libc++.a /usr/lib/x86_64-linux-gnu/libm.a"' ${VERBOSE_FLAGS} +local || true

test.functionals.cleanup:
	rm -f tests/functionals

test.functionals.compile: govendor genlocalfiles
	${GOPATH}/bin/govendor test -tags "${TAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -c -o tests/functionals ./tests/

test.functionals.run:
ifneq ($(VERBOSE_FLAGS),)
	cd tests && sudo -E ./functionals -test.v -test.timeout ${TIMEOUT} ${ARGS}
else
	cd tests && sudo -E ./functionals -test.timeout ${TIMEOUT} ${ARGS}
endif

test.functionals.all: test.functionals.compile
	make TIMEOUT="8m" ARGS=${ARGS} test.functionals.run

test.functionals.batch: test.functionals.compile
ifneq ($(TEST_PATTERN),)
	set -e ; make ARGS="${ARGS} -test.run ${TEST_PATTERN}" test.functionals.run
else
	set -e ; make ARGS="${ARGS}" test.functionals.run
endif

test.functionals: test.functionals.compile
	for functest in ${FUNC_TESTS} ; do \
		make ARGS="-test.run $$functest$$\$$ ${ARGS} ${COVERAGE_ARGS}" test.functionals.run; \
	done

test: govendor genlocalfiles
ifeq ($(COVERAGE), true)
	set -v ; \
	for pkg in ${UT_PACKAGES}; do \
		if [ -n "$$pkg" ]; then \
			coverfile="$$(echo $$pkg | tr / -).cover"; \
			${GOPATH}/bin/govendor test -tags "${TAGS} test" -covermode=${COVERAGE_MODE} -coverprofile="$$coverfile" ${VERBOSE_FLAGS} -timeout ${TIMEOUT} $$pkg; \
		fi; \
	done
else
	set -v ; \
	${GOPATH}/bin/govendor test -tags "${TAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} ${UT_PACKAGES}
endif

govendor:
	go get github.com/kardianos/govendor
	${GOPATH}/bin/govendor sync

fmt: govendor
	@echo "+ $@"
	@test -z "$$(${GOPATH}/bin/govendor fmt +local)" || \
		(echo "+ please format Go code with 'gofmt -s'" && /bin/false)

vet: govendor
	@echo "+ $@"
	test -z "$$(${GOPATH}/bin/govendor tool vet $$(${GOPATH}/bin/govendor list -no-status +local | perl -pe 's|github.com/skydive-project/skydive/?||g' | grep -v '^tests') 2>&1 | tee /dev/stderr | grep -v '^flow/probes/afpacket/' | grep -v 'exit status 1')"

check: govendor
	@test -z "$$(${GOPATH}/bin/govendor list +u)" || \
		(echo -e "You must remove these unused packages:\n$$($${GOPATH}/bin/govendor list +u)" && /bin/false)

ineffassign interfacer golint goimports varcheck structcheck aligncheck deadcode gotype errcheck gocyclo dupl:
	@go get github.com/alecthomas/gometalinter
	@command -v $@ >/dev/null || gometalinter --install --update

gometalinter: ineffassign interfacer golint goimports varcheck structcheck aligncheck deadcode gotype errcheck gocyclo dupl

lint: gometalinter
	@echo "+ $@"
	@gometalinter -j 2 --disable=gotype --vendor -e '.*\.pb.go' --skip=statics/... --deadline 10m --sort=path ./... --json > lint.json || true
	cat lint.json

# dependency package need for building the project
builddep:
	go get github.com/golang/protobuf/proto
	go get github.com/golang/protobuf/protoc-gen-go
	go get github.com/jteeuwen/go-bindata/...

genlocalfiles: .proto .bindata skydive-bash-completion.sh

clean: test.functionals.cleanup
	grep path vendor/vendor.json | perl -pe 's|.*": "(.*?)".*|\1|g' | xargs -n 1 go clean -i >/dev/null 2>&1 || true

doc:
	mkdir -p /tmp/skydive-doc
	hugo --theme=hugo-material-docs -s doc -d /tmp/skydive-doc
	git checkout gh-pages
	cp -dpR /tmp/skydive-doc/* .
	git add -A
	git commit -a -m "Documentation update"
	git push -f gerrit gh-pages

doctest:
	hugo server run -t hugo-material-docs -s doc -b http://localhost:1313/skydive

srpm:
	contrib/packaging/rpm/generate-skydive-bootstrap.sh -s

rpm:
	contrib/packaging/rpm/generate-skydive-bootstrap.sh -b

docker-image: static
	cp $$GOPATH/bin/skydive contrib/docker/
	sudo -E docker build -t ${DOCKER_IMAGE}:${DOCKER_TAG} -f contrib/docker/Dockerfile contrib/docker/

vendor: govendor check
	tar cvzf vendor.tar.gz vendor/

dist:
	tmpdir=`mktemp -d -u --suffix=skydive-pkg`; \
	godir=$${tmpdir}/skydive-${VERSION}; \
	skydivedir=$${godir}/src/github.com/skydive-project/skydive; \
	mkdir -p `dirname $$skydivedir`; \
	git clone . $$skydivedir; \
	pushd $$skydivedir; \
	export GOPATH=$$godir; \
	cd $$skydivedir; \
	echo "go take a coffee, govendor sync takes time ..."; \
	make govendor genlocalfiles; \
	popd; \
	tar -C $$tmpdir --exclude=skydive-${VERSION}/src/github.com/skydive-project/skydive/.git -cvzf ${DESTDIR}/skydive-${VERSION}.tar.gz skydive-${VERSION}/src; \
	rm -rf $$tmpdir
