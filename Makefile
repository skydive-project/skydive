VERSION?=0.9.0

# really Basic Makefile for Skydive
export GO15VENDOREXPERIMENT=1

PROTO_FILES=flow/flow.proto flow/set.proto flow/request.proto
VERBOSE_FLAGS?=-v
VERBOSE?=true
ifeq ($(VERBOSE), false)
	VERBOSE_FLAGS:=
endif
TIMEOUT?=1m
TEST_PATTERN?=
UT_PACKAGES=$(shell ${GOPATH}/bin/govendor list -no-status +local | grep -v '/tests')
FUNC_TESTS_CMD:="grep -e 'func Test${TEST_PATTERN}' tests/*.go | perl -pe 's|.*func (.*?)\(.*|\1|g' | shuf"
FUNC_TESTS:=$(shell sh -c $(FUNC_TESTS_CMD))
DOCKER_IMAGE?=skydive/skydive
DOCKER_TAG?=devel
DESTDIR?=$(shell pwd)

.proto: govendor builddep ${PROTO_FILES}
	protoc --go_out . ${PROTO_FILES}
	# always export flow.ParentUUID as we need to store this information to know
	# if it's a Outer or Inner packet.
	sed -e 's/ParentUUID\(.*\),omitempty\(.*\)/ParentUUID\1\2/' -e 's/int64\(.*\),omitempty\(.*\)/int64\1\2/' -i flow/flow.pb.go

.bindata: govendor builddep
	go-bindata -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/* statics/css/images/* statics/js/components/*
	gofmt -w -s statics/bindata.go

all: govendor genlocalfiles
	${GOPATH}/bin/govendor install ${GOFLAGS} ${VERBOSE_FLAGS} +local

install: govendor genlocalfiles
	${GOPATH}/bin/govendor install ${GOFLAGS} ${VERBOSE_FLAGS} +local

build: govendor genlocalfiles
	${GOPATH}/bin/govendor build ${GOFLAGS} ${VERBOSE_FLAGS} +local

static: genlocalfiles
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

test.functionals: test.functionals.compile
	set -e ; \
	for functest in ${FUNC_TESTS} ; do \
		make ARGS="-test.run $$functest$$\$$ ${ARGS}" test.functionals.run ; \
	done

test: govendor genlocalfiles
	${GOPATH}/bin/govendor test -tags "${TAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} ${UT_PACKAGES}

govendor:
	go get github.com/kardianos/govendor
	${GOPATH}/bin/govendor sync

fmt: govendor
	@echo "+ $@"
	@test -z "$$(${GOPATH}/bin/govendor fmt +local)" || \
		(echo "+ please format Go code with 'gofmt -s'" && /bin/false)

vet: govendor
	@echo "+ $@"
	govendor tool vet $$(ls -d */ | grep -v vendor)

ineffassign interfacer golint goimports varcheck structcheck aligncheck deadcode gotype errcheck gocyclo dupl:
	@go get github.com/alecthomas/gometalinter
	@command -v $@ >/dev/null || gometalinter --install --update

gometalinter: ineffassign interfacer golint goimports varcheck structcheck aligncheck deadcode gotype errcheck gocyclo dupl

lint: gometalinter
	@echo "+ $@"
	@gometalinter --disable=gotype --skip=vendor/... --skip=statics/... --deadline 5m --sort=path ./...  2>&1 | grep -v -e 'error return value not checked' -e 'should have comment or be unexported' -e 'declaration of err shadows declaration'

# dependency package need for building the project
builddep:
	go get github.com/golang/protobuf/proto
	go get github.com/golang/protobuf/protoc-gen-go
	go get github.com/jteeuwen/go-bindata/...

genlocalfiles: .proto .bindata

clean: test.functionals.cleanup
	grep path vendor/vendor.json | perl -pe 's|.*": "(.*?)".*|\1|g' | xargs -n 1 go clean -i >/dev/null 2>&1 || true

.PHONY: doc

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
