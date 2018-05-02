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

define VENDOR_BUILD
ln -s vendor src ; \
cd vendor/$1; \
GOPATH=$$GOPATH/src/${SKYDIVE_GITHUB} go build $1; \
cd -; \
unlink src
endef

define VENDOR_RUN
PATH=$${GOPATH}/src/${SKYDIVE_GITHUB}/vendor/$1:$$PATH
endef

VERSION?=$(shell $(VERSION_CMD))
$(info ${VERSION})

# really Basic Makefile for Skydive
export GO15VENDOREXPERIMENT=1

GOVENDOR:=${GOPATH}/bin/govendor
SKYDIVE_GITHUB:=github.com/skydive-project/skydive
SKYDIVE_GITHUB_VERSION:=$(SKYDIVE_GITHUB)/version.Version=${VERSION}
GO_BINDATA_GITHUB:=github.com/jteeuwen/go-bindata/go-bindata
PROTOC_GEN_GO_GITHUB:=github.com/golang/protobuf/protoc-gen-go
SKYDIVE_PKG:=skydive-${VERSION}
FLOW_PROTO_FILES=flow/flow.proto flow/set.proto flow/request.proto
FILTERS_PROTO_FILES=filters/filters.proto
HTTP_PROTO_FILES=http/wsstructmessage.proto
VERBOSE_FLAGS?=-v
VERBOSE_TESTS_FLAGS?=-test.v
VERBOSE?=true
ifeq ($(VERBOSE), false)
  VERBOSE_FLAGS:=
  VERBOSE_TESTS_FLAGS:=
endif
ifeq ($(COVERAGE), true)
  EXTRA_ARGS+= -test.coverprofile=../functionals.cover
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

ifeq ($(WITH_DPDK), true)
  BUILDTAGS+=dpdk
endif

ifeq ($(WITH_EBPF), true)
  BUILDTAGS+=ebpf
  EXTRABINDATA+=probe/ebpf/*.o
endif

ifeq ($(WITH_PROF), true)
  BUILDTAGS+=prof
endif

ifeq ($(WITH_SCALE), true)
  BUILDTAGS+=scale
endif

ifeq ($(WITH_NEUTRON), true)
  BUILDTAGS+=neutron
endif

ifeq ($(WITH_SELENIUM), true)
  BUILDTAGS+=selenium
endif

ifeq ($(WITH_CDD), true)
  BUILDTAGS+=cdd
endif

ifeq ($(WITH_K8S), true)
  BUILDTAGS+=k8s
  EXTRA_ARGS+=-analyzer.topology.probes=k8s
endif

.PHONY: all install
all install: skydive

skydive.yml: etc/skydive.yml.default
	[ -e $@ ] || cp $< $@

.PHONY: debug
debug: GOFLAGS+=-gcflags='-N -l'
debug: GO_BINDATA_FLAGS+=-debug
debug: skydive.cleanup skydive skydive.yml

define skydive_debug
sudo $$(which dlv) exec $$(which skydive) -- $1 -c skydive.yml
endef

.PHONY: debug.agent
debug.agent:
	$(call skydive_debug,agent)

.PHONY: debug.analyzer
debug.analyzer:
	$(call skydive_debug,analyzer)

.PHONY: .proto
.proto: builddep ${FLOW_PROTO_FILES} ${FILTERS_PROTO_FILES} ${HTTP_PROTO_FILES}
	$(call VENDOR_RUN,${PROTOC_GEN_GO_GITHUB}) protoc --go_out . ${FLOW_PROTO_FILES}
	$(call VENDOR_RUN,${PROTOC_GEN_GO_GITHUB}) protoc --go_out . ${FILTERS_PROTO_FILES}
	$(call VENDOR_RUN,${PROTOC_GEN_GO_GITHUB}) protoc --go_out . ${HTTP_PROTO_FILES}
	# always export flow.ParentUUID as we need to store this information to know
	# if it's a Outer or Inner packet.
	sed -e 's/ParentUUID\(.*\),omitempty\(.*\)/ParentUUID\1\2/' -e 's/int64\(.*\),omitempty\(.*\)/int64\1\2/' -i.bak flow/flow.pb.go
	# do not export LastRawPackets used internally
	sed -e 's/json:"LastRawPackets,omitempty"/json:"-"/g' -i.bak flow/flow.pb.go
	# add flowState to flow generated struct
	sed -e 's/type Flow struct {/type Flow struct { XXX_state flowState `json:"-"`/' -i.bak flow/flow.pb.go
	gofmt -s -w flow/flow.pb.go

BINDATA_DIRS := \
	statics/* \
	statics/css/images/* \
	statics/css/themes/*/* \
	statics/js/vendor/* \
	statics/js/components/* \
	${EXTRABINDATA}

.PHONY: .bindata
.bindata: builddep ebpf.build
	$(call VENDOR_RUN,${GO_BINDATA_GITHUB}) go-bindata ${GO_BINDATA_FLAGS} -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go $(BINDATA_DIRS)
	gofmt -w -s statics/bindata.go

.PHONY: compile
compile:
	$(GOVENDOR) install \
		-ldflags="-X $(SKYDIVE_GITHUB_VERSION)" \
		${GOFLAGS} -tags="${BUILDTAGS}" ${VERBOSE_FLAGS} \
		${SKYDIVE_GITHUB}

.PHONY: skydive
skydive: govendor genlocalfiles dpdk.build contribs compile

.PHONY: skydive.cleanup
skydive.cleanup:
	go clean -i $(SKYDIVE_GITHUB)

.PHONY: bench
bench:	skydive bench.flow

flow/pcaptraces/201801011400.pcap.gz:
	aria2c -s 16 -x 16 -o $@ "http://mawi.nezu.wide.ad.jp/mawi/samplepoint-F/2018/201801011400.pcap.gz"

flow/pcaptraces/%.pcap: flow/pcaptraces/%.pcap.gz
	gunzip -fk $<

flow/pcaptraces/201801011400.small.pcap: flow/pcaptraces/201801011400.pcap
	tcpdump -r $< -w $@ "ip host 203.82.244.188"

bench.flow.traces: flow/pcaptraces/201801011400.small.pcap

bench.flow: bench.flow.traces
	govendor test -bench=. ${SKYDIVE_GITHUB}/flow

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
	STATIC_DIR := $(shell dpkg-architecture --command sh -c 'echo /usr/lib/$$DEB_TARGET_MULTIARCH')
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

.PHONY: static
static: skydive.cleanup govendor genlocalfiles
	$(GOVENDOR) install \
		-ldflags "-X $(SKYDIVE_GITHUB_VERSION) \
		-extldflags \"-static $(STATIC_LIBS_ABS)\"" \
		${VERBOSE_FLAGS} -tags "netgo ${BUILDTAGS}" \
		-installsuffix netgo || true

.PHONY: contribs.cleanup
contribs.cleanup:
	$(MAKE) -C contrib/snort clean

.PHONY: contribs
contribs:
	$(MAKE) -C contrib/snort

.PHONY: dpdk.build
dpdk.build:
ifeq ($(WITH_DPDK), true)
	$(MAKE) -C dpdk
endif

.PHONY: dpdk.cleanup
dpdk.cleanup:
	$(MAKE) -C dpdk clean

.PHONY: ebpf.build
ebpf.build:
ifeq ($(WITH_EBPF), true)
	$(MAKE) -C probe/ebpf
endif

.PHONY: ebpf.clean
ebpf.clean:
	$(MAKE) -C probe/ebpf clean

.PHONY: test.functionals.cleanup
test.functionals.cleanup:
	rm -f tests/functionals

.PHONY: test.functionals.compile
test.functionals.compile: govendor genlocalfiles
	$(GOVENDOR) test -tags "${BUILDTAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -c -o tests/functionals ./tests/

.PHONY: test.functionals.run
test.functionals.run:
	cd tests && sudo -E ./functionals ${VERBOSE_TESTS_FLAGS} -test.timeout ${TIMEOUT} ${ARGS} ${EXTRA_ARGS}

.PHONY: tests.functionals.all
test.functionals.all: test.functionals.compile
	$(MAKE) TIMEOUT="8m" ARGS="${ARGS} ${EXTRA_ARGS}" test.functionals.run

.PHONY: test.functionals.batch
test.functionals.batch: test.functionals.compile
ifneq ($(TEST_PATTERN),)
	set -e ; $(MAKE) ARGS="${ARGS} ${EXTRA_ARGS} -test.run ${TEST_PATTERN}" test.functionals.run
else
	set -e ; $(MAKE) ARGS="${ARGS} ${EXTRA_ARGS}" test.functionals.run
endif

.PHONY: test.functionals
test.functionals: test.functionals.compile
	for functest in ${FUNC_TESTS} ; do \
		$(MAKE) ARGS="-test.run $$functest$$\$$ ${ARGS} ${EXTRA_ARGS}" test.functionals.run; \
	done

.PHONY: functional
functional:
	$(MAKE) test.functionals VERBOSE=true TIMEOUT=10m ARGS='-standalone -analyzer.topology.backend elasticsearch -analyzer.flow.backend elasticsearch'

.PHONY: test
test: govendor genlocalfiles
ifeq ($(COVERAGE), true)
	set -v ; \
	go get github.com/mattn/goveralls ; \
	for pkg in ${UT_PACKAGES}; do \
		if [ -n "$$pkg" ]; then \
			coverfile="${COVERAGE_WD}/$$(echo $$pkg | tr / -).cover"; \
			$(GOVENDOR) test -tags "${BUILDTAGS} test" -covermode=${COVERAGE_MODE} -coverprofile="$$coverfile" ${VERBOSE_FLAGS} -timeout ${TIMEOUT} $$pkg; \
		fi; \
	done
else
ifneq ($(TEST_PATTERN),)
	set -v ; \
	$(GOVENDOR) test -tags "${BUILDTAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -test.run ${TEST_PATTERN} ${UT_PACKAGES}
else
	set -v ; \
	$(GOVENDOR) test -tags "${BUILDTAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} ${UT_PACKAGES}
endif
endif

.PHONY: govendor
govendor:
	go get github.com/kardianos/govendor
	$(GOVENDOR) sync
	patch -p0 < dpdk/dpdk.govendor.patch
	rm -rf vendor/github.com/weaveworks/tcptracer-bpf/vendor/github.com/

.PHONY: fmt
fmt: govendor genlocalfiles
	@echo "+ $@"
	@test -z "$$($(GOVENDOR) fmt +local)" || \
		(echo "+ please format Go code with 'gofmt -s'" && /bin/false)

.PHONY: vet
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

.PHONY: check
check: govendor
	@test -z "$$($(GOVENDOR) list +u)" || \
		(echo -e "You must remove these unused packages:\n$$($(GOVENDOR) list +u)" && /bin/false)

LINTER_COMMANDS := \
	aligncheck \
	deadcode \
	dupl \
	errcheck \
	gocyclo \
	golint \
	goimports \
	gotype \
	ineffassign \
	interfacer \
	structcheck \
	varcheck

.PHONY: $(LINTER_COMMANDS)
$(LINTER_COMMANDS):
	@go get github.com/alecthomas/gometalinter
	@command -v $@ >/dev/null || gometalinter --install --update

.PHONY: gometalinter
gometalinter: $(LINTER_COMMANDS)

.PHONY: lint
lint: gometalinter
	@echo "+ $@"
	@gometalinter --disable=gotype --vendor -e '.*\.pb.go' --skip=statics/... --deadline 10m --sort=path ./... --json > lint.json || true
	cat lint.json

# dependency package need for building the project
.PHONY: builddep
builddep: govendor
	$(call VENDOR_BUILD,${PROTOC_GEN_GO_GITHUB})
	$(call VENDOR_BUILD,${GO_BINDATA_GITHUB})

.PHONY: genlocalfiles
genlocalfiles: .proto .bindata

.PHONY: clean
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

.PHONY: doctest
doctest:
	hugo server run -t hugo-material-docs -s doc -b http://localhost:1313/skydive

.PHONY: srpm
srpm:
	contrib/packaging/rpm/generate-skydive-bootstrap.sh -s ${BOOTSTRAP_ARGS}

.PHONY: rpm
rpm:
	contrib/packaging/rpm/generate-skydive-bootstrap.sh -b ${BOOTSTRAP_ARGS}

.PHONY: docker-image
docker-image: static
	cp $$GOPATH/bin/skydive contrib/docker/
	sudo -E docker build -t ${DOCKER_IMAGE}:${DOCKER_TAG} -f contrib/docker/Dockerfile contrib/docker/

.PHONY: vendor
vendor: govendor check
	tar cvzf vendor.tar.gz vendor/

.PHONY: localdist
localdist: govendor genlocalfiles
	tar -C $$GOPATH --transform "s/^src/$(SKYDIVE_PKG)\/src/" \
		--exclude=src/$(SKYDIVE_GITHUB)/rpmbuild \
		--exclude=src/$(SKYDIVE_GITHUB)/.git \
		-cvzf ${DESTDIR}/$(SKYDIVE_PKG).tar.gz src/$(SKYDIVE_GITHUB)

.PHONY: dist
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
