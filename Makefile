define VERSION_CMD = 
eval ' \
	define=""; \
	version=`git rev-parse --verify HEAD`; \
	tagname=`git show-ref --tags | grep $$version`; \
	if [ -n "$$tagname" ]; then \
		define=`echo $$tagname | awk -F "/" "{print \\$$NF}" | tr -d [a-z]`; \
	else \
		define=`printf "%.12s" $$version`; \
	fi; \
	tainted=`git ls-files -m | wc -l` ; \
	if [ "$$tainted" -gt 0 ]; then \
		define="$${define}-tainted"; \
	fi; \
	echo "$$define" \
'
endef
VERSION?=$(shell $(VERSION_CMD))
$(info ${VERSION})

# really Basic Makefile for Skydive
export GO15VENDOREXPERIMENT=1

GOVENDOR:=${GOPATH}/bin/govendor 
SKYDIVE_GITHUB:=github.com/skydive-project/skydive
SKYDIVE_GITHUB_VERSION:=$(SKYDIVE_GITHUB)/version.Version=${VERSION}
SKYDIVE_PKG:=skydive-${VERSION}
FLOW_PROTO_FILES=flow/flow.proto flow/set.proto flow/request.proto
FILTERS_PROTO_FILES=filters/filters.proto
VERBOSE_FLAGS?=-v
VERBOSE_TESTS_FLAGS?=-test.v
VERBOSE?=true
ifeq ($(VERBOSE), false)
  VERBOSE_FLAGS:=
  VERBOSE_TESTS_FLAGS:=
endif
ifeq ($(COVERAGE), true)
  COVERAGE_ARGS:= -test.coverprofile=../functionals.cover
endif
TIMEOUT?=1m
TEST_PATTERN?=
UT_PACKAGES?=$(shell $(GOVENDOR) list -no-status +local | grep -v '/tests')
FUNC_TESTS_CMD:="grep -e 'func Test${TEST_PATTERN}' tests/*.go | perl -pe 's|.*func (.*?)\(.*|\1|g' | shuf"
FUNC_TESTS:=$(shell sh -c $(FUNC_TESTS_CMD))
DOCKER_IMAGE?=skydive/skydive
DOCKER_TAG?=devel
DESTDIR?=$(shell pwd)
COVERAGE?=0
COVERAGE_MODE?=atomic
COVERAGE_WD?="."
BOOTSTRAP_ARGS?=
GOTAGS?=

ifeq ($(WITH_DPDK), true)
  BUILDTAGS+=dpdk
endif

ifeq ($(WITH_EBPF), true)
  BUILDTAGS+=ebpf
  EXTRABINDATA+=probe/ebpf/*.o
endif

.PHONY: all install
all install: skydive

.proto: builddep ${FLOW_PROTO_FILES} ${FILTERS_PROTO_FILES}
	protoc --go_out . ${FLOW_PROTO_FILES}
	protoc --go_out . ${FILTERS_PROTO_FILES}
	# always export flow.ParentUUID as we need to store this information to know
	# if it's a Outer or Inner packet.
	sed -e 's/ParentUUID\(.*\),omitempty\(.*\)/ParentUUID\1\2/' -e 's/int64\(.*\),omitempty\(.*\)/int64\1\2/' -i flow/flow.pb.go
	# do not export LastRawPackets used internally
	sed -e 's/json:"LastRawPackets,omitempty"/json:"-"/g' -i flow/flow.pb.go
	# add flowState to flow generated struct
	sed -e 's/type Flow struct {/type Flow struct {\n\tXXX_state flowState `json:"-"`/' -i flow/flow.pb.go

BINDATA_DIRS := \
	statics/* \
	statics/css/images/* \
	statics/js/vendor/* \
	statics/js/components/* \
	${EXTRABINDATA}

.bindata: builddep ebpf.build
	go-bindata ${GO_BINDATA_FLAGS} -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go $(BINDATA_DIRS)
	gofmt -w -s statics/bindata.go

skydive: govendor genlocalfiles dpdk.build contribs
	$(GOVENDOR) install \
		-ldflags="-X $(SKYDIVE_GITHUB_VERSION)" \
		${GOFLAGS} -tags="${BUILDTAGS} ${GOTAGS}" ${VERBOSE_FLAGS} \
		+local

skydive.cleanup:
	go clean -i $(SKYDIVE_GITHUB)

STATIC_DIR :=
STATIC_LIBS :=

OS_RHEL := $(shell test -f /etc/redhat-release && echo -n Y)
ifeq ($(OS_RHEL),Y)
	STATIC_DIR := /usr/lib64
	STATIC_LIBS := \
		libz.a \
		liblzma.a \
		libm.a
endif

OS_DEB := $(shell test -f /etc/debian_version && echo -n Y)
ifeq ($(OS_DEB),Y)
	STATIC_DIR := /usr/lib/x86_64-linux-gnu
	STATIC_LIBS := \
		libz.a \
		liblzma.a \
		libicuuc.a \
		libicudata.a \
		libxml2.a \
		libc.a \
		libdl.a \
		libpthread.a \
		libc++.a \
		libm.a
endif

STATIC_LIBS_ABS := $(addprefix $(STATIC_DIR)/,$(STATIC_LIBS))

static: skydive.cleanup govendor genlocalfiles
	$(GOVENDOR) install \
		-ldflags "-X $(SKYDIVE_GITHUB_VERSION) \
		-extldflags \"-static $(STATIC_LIBS_ABS)\"" \
		${VERBOSE_FLAGS} -tags "netgo ${BUILDTAGS} ${GOTAGS}" \
		-installsuffix netgo +local || true

contribs.cleanup:
	$(MAKE) -C contrib/snort clean

contribs:
	$(MAKE) -C contrib/snort

dpdk.build:
ifeq ($(WITH_DPDK), true)
	$(MAKE) -C dpdk
endif

dpdk.cleanup:
	$(MAKE) -C dpdk clean

ebpf.build:
ifeq ($(WITH_EBPF), true)
	$(MAKE) -C probe/ebpf
endif

ebpf.clean:
	$(MAKE) -C probe/ebpf clean

test.functionals.cleanup:
	rm -f tests/functionals

test.functionals.compile: govendor genlocalfiles
	$(GOVENDOR) test -tags "${BUILDTAGS} ${GOTAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -c -o tests/functionals ./tests/

test.functionals.run:
	cd tests && sudo -E ./functionals ${VERBOSE_TESTS_FLAGS} -test.timeout ${TIMEOUT} ${ARGS}

test.functionals.all: test.functionals.compile
	$(MAKE) TIMEOUT="8m" ARGS=${ARGS} test.functionals.run

test.functionals.batch: test.functionals.compile
ifneq ($(TEST_PATTERN),)
	set -e ; $(MAKE) ARGS="${ARGS} -test.run ${TEST_PATTERN}" test.functionals.run
else
	set -e ; $(MAKE) ARGS="${ARGS}" test.functionals.run
endif

test.functionals: test.functionals.compile
	for functest in ${FUNC_TESTS} ; do \
		$(MAKE) ARGS="-test.run $$functest$$\$$ ${ARGS} ${COVERAGE_ARGS}" test.functionals.run; \
	done

functional:
	$(MAKE) test.functionals VERBOSE=true TIMEOUT=10m ARGS='-standalone'

test: govendor genlocalfiles
ifeq ($(COVERAGE), true)
	set -v ; \
	go get github.com/mattn/goveralls ; \
	for pkg in ${UT_PACKAGES}; do \
		if [ -n "$$pkg" ]; then \
			coverfile="${COVERAGE_WD}/$$(echo $$pkg | tr / -).cover"; \
			$(GOVENDOR) test -tags "${BUILDTAGS} ${GOTAGS} test" -covermode=${COVERAGE_MODE} -coverprofile="$$coverfile" ${VERBOSE_FLAGS} -timeout ${TIMEOUT} $$pkg; \
		fi; \
	done
else
	set -v ; \
	$(GOVENDOR) test -tags "${BUILDTAGS} ${GOTAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} ${UT_PACKAGES}
endif

govendor:
	go get github.com/kardianos/govendor
	$(GOVENDOR) sync
	patch -p0 < dpdk/dpdk.govendor.patch

fmt: govendor genlocalfiles
	@echo "+ $@"
	@test -z "$$($(GOVENDOR) fmt +local)" || \
		(echo "+ please format Go code with 'gofmt -s'" && /bin/false)

vet: govendor
	@echo "+ $@"
	test -z "$$($(GOVENDOR) tool vet $$( \
			$(GOVENDOR) list -no-status +local \
			| perl -pe 's|$(SKYDIVE_GITHUB)/?||g' \
			| grep -v '^tests') 2>&1 \
		| tee /dev/stderr \
		| grep -v '^flow/probes/afpacket/' \
		| grep -v 'exit status 1' \
		)"

check: govendor
	@test -z "$$($(GOVENDOR) list +u)" || \
		(echo -e "You must remove these unused packages:\n$$($(GOVENDOR) list +u)" && /bin/false)

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

genlocalfiles: .proto .bindata

clean: skydive.cleanup test.functionals.cleanup dpdk.cleanup contribs.cleanup
	grep path vendor/vendor.json | perl -pe 's|.*": "(.*?)".*|\1|g' | xargs -n 1 go clean -i >/dev/null 2>&1 || true

.PHONY: doc
doc:
	mkdir -p /tmp/skydive-doc
	touch /tmp/skydive-doc/.nojekyll
	hugo --theme=hugo-material-docs -s doc -d /tmp/skydive-doc
	git checkout gh-pages
	cp -dpR /tmp/skydive-doc/* .
	git add -A
	git commit -a -m "Documentation update"
	git push -f gerrit gh-pages

doctest:
	hugo server run -t hugo-material-docs -s doc -b http://localhost:1313/skydive

srpm:
	contrib/packaging/rpm/generate-skydive-bootstrap.sh -s ${BOOTSTRAP_ARGS}

rpm:
	contrib/packaging/rpm/generate-skydive-bootstrap.sh -b ${BOOTSTRAP_ARGS}

docker-image: static
	cp $$GOPATH/bin/skydive contrib/docker/
	sudo -E docker build -t ${DOCKER_IMAGE}:${DOCKER_TAG} -f contrib/docker/Dockerfile contrib/docker/

vendor: govendor check
	tar cvzf vendor.tar.gz vendor/

localdist: govendor genlocalfiles
	tar -C $$GOPATH --transform "s/^src/$(SKYDIVE_PKG)\/src/" \
		--exclude=src/$(SKYDIVE_GITHUB)/rpmbuild \
		--exclude=src/$(SKYDIVE_GITHUB)/.git \
		-cvzf ${DESTDIR}/$(SKYDIVE_PKG).tar.gz src/$(SKYDIVE_GITHUB)

dist:
	tmpdir=`mktemp -d -u --suffix=skydive-pkg`; \
	godir=$${tmpdir}/$(SKYDIVE_PKG); \
	skydivedir=$${godir}/src/$(SKYDIVE_GITHUB); \
	mkdir -p `dirname $$skydivedir`; \
	git clone . $$skydivedir; \
	pushd $$skydivedir; \
	mkdir -p $$godir/.cache; \
	[ -d $$GOPATH/.cache/govendor ] && ln -s $$GOPATH/.cache/govendor $$godir/.cache/govendor; \
	export GOPATH=$$godir; \
	cd $$skydivedir; \
	echo "go take a coffee, govendor sync takes time ..."; \
	$(MAKE) govendor genlocalfiles; \
	popd; \
	tar -C $$tmpdir --exclude=$(SKYDIVE_PKG)/src/$(SKYDIVE_GITHUB)/.git \
		-cvzf ${DESTDIR}/$(SKYDIVE_PKG).tar.gz $(SKYDIVE_PKG)/src; \
	rm -rf $$tmpdir
