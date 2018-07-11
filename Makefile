define VERSION_CMD =
eval ' \
	define=""; \
	version=`git describe --abbrev=0 --tags | tr -d "[a-z]"` ; \
	commit=`git rev-parse --verify HEAD`; \
	tagname=`git show-ref --tags | grep $$commit`; \
	if [ -n "$$tagname" ]; then \
		define=`echo $$tagname | awk -F "/" "{print \\$$NF}" | tr -d "[a-z]"`; \
	else \
		define=`printf "$$version-%.12s" $$commit`; \
	fi; \
	tainted=`git ls-files -m | wc -l` ; \
	if [ "$$tainted" -gt 0 ]; then \
		define="$${define}-tainted"; \
	fi; \
	echo "$$define" \
'
endef

define VENDOR_BUILD
ln -s vendor src || mv vendor src; \
cd src/$1; \
(unset GOARCH; unset CC; GOPATH=$$GOPATH/src/${SKYDIVE_GITHUB} go build $1); \
cd -; \
unlink src || mv src vendor
endef

define VENDOR_RUN
PATH=$${GOPATH}/src/${SKYDIVE_GITHUB}/vendor/$1:$$PATH
endef

VERSION?=$(shell $(VERSION_CMD))
GO_GET:=CC= GOARCH= go get
GOVENDOR:=${GOPATH}/bin/govendor
BUILD_CMD?=${GOVENDOR}
BUILD_ID:=$(shell echo 0x$$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \n'))
SKYDIVE_GITHUB:=github.com/skydive-project/skydive
SKYDIVE_PKG:=skydive-${VERSION}
SKYDIVE_PATH:=$(SKYDIVE_PKG)/src/$(SKYDIVE_GITHUB)/
SKYDIVE_GITHUB_VERSION:=$(SKYDIVE_GITHUB)/version.Version=${VERSION}
GO_BINDATA_GITHUB:=github.com/jteeuwen/go-bindata/go-bindata
PROTOC_GEN_GO_GITHUB:=github.com/golang/protobuf/protoc-gen-go
FLOW_PROTO_FILES=flow/flow.proto flow/set.proto flow/request.proto
FILTERS_PROTO_FILES=filters/filters.proto
HTTP_PROTO_FILES=http/wsstructmessage.proto
EASYJSON_GITHUB:=github.com/mailru/easyjson/easyjson
EASYJSON_FILES_ALL=flow/flow.pb.go
EASYJSON_FILES_TAG=flow/storage/elasticsearch/elasticsearch.go topology/graph/elasticsearch.go
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
BOOTSTRAP:=contrib/packaging/rpm/generate-skydive-bootstrap.sh
BOOTSTRAP_ARGS?=
BUILD_TAGS?=$(TAGS)
WITH_LXD?=true
WITH_OPENCONTRAIL?=true

STATIC_DIR?=
STATIC_LIBS?=

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
	STATIC_DIR := $(shell dpkg-architecture -c 'sh' -c 'echo /usr/lib/$$DEB_HOST_MULTIARCH')
	STATIC_LIBS := \
		libz.a \
		liblzma.a \
		libc.a \
		libdl.a \
		libpthread.a \
		libc++.a \
		libm.a
endif

ifeq ($(WITH_DPDK), true)
  BUILD_TAGS+=dpdk
endif

ifeq ($(WITH_EBPF), true)
  BUILD_TAGS+=ebpf
  EXTRABINDATA+=probe/ebpf/*.o
endif

ifeq ($(WITH_PROF), true)
  BUILD_TAGS+=prof
endif

ifeq ($(WITH_SCALE), true)
  BUILD_TAGS+=scale
endif

ifeq ($(WITH_NEUTRON), true)
  BUILD_TAGS+=neutron
endif

ifeq ($(WITH_SELENIUM), true)
  BUILD_TAGS+=selenium
endif

ifeq ($(WITH_CDD), true)
  BUILD_TAGS+=cdd
endif

ifeq ($(WITH_MUTEX_DEBUG), true)
  BUILD_TAGS+=mutexdebug
endif

ifeq ($(WITH_K8S), true)
  BUILD_TAGS+=k8s
  EXTRA_ARGS+=-analyzer.topology.probes=k8s
endif

ifeq ($(WITH_HELM), true)
  BUILD_TAGS+=helm
endif

ifeq ($(WITH_OPENCONTRAIL), true)
  BUILD_TAGS+=opencontrail
  STATIC_LIBS+=libxml2.a
ifeq ($(OS_DEB),Y)
  STATIC_LIBS+=libicuuc.a \
               libicudata.a
endif
endif

ifeq ($(WITH_LXD), true)
  BUILD_TAGS+=lxd
endif

STATIC_LIBS_ABS := $(addprefix $(STATIC_DIR)/,$(STATIC_LIBS))

.PHONY: all install
all install: skydive

.PHONY: version
version:
	@echo -n ${VERSION}

skydive.yml: etc/skydive.yml.default
	[ -e $@ ] || cp $< $@

.PHONY: debug
debug: GOFLAGS+=-gcflags='-N -l'
debug: GO_BINDATA_FLAGS+=-debug
debug: skydive.clean skydive skydive.yml

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
	sed -e 's/ParentUUID\(.*\),omitempty\(.*\)/ParentUUID\1\2/' \
		-e 's/Protocol\(.*\),omitempty\(.*\)/Protocol\1\2/' \
		-e 's/ICMPType\(.*\),omitempty\(.*\)/ICMPType\1\2/' \
		-e 's/int64\(.*\),omitempty\(.*\)/int64\1\2/' \
		-i.bak flow/flow.pb.go
	# do not export LastRawPackets used internally
	sed -e 's/json:"LastRawPackets,omitempty"/json:"-"/g' -i.bak flow/flow.pb.go
	# add flowState to flow generated struct
	sed -e 's/type Flow struct {/type Flow struct { XXX_state flowState `json:"-"`/' -i.bak flow/flow.pb.go
	gofmt -s -w flow/flow.pb.go

.PHONY: .easyjson
.easyjson: builddep .proto ${EASYJSON_FILES_ALL} ${EASYJSON_FILES_TAG}
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson -all ${EASYJSON_FILES_ALL}
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson ${EASYJSON_FILES_TAG}

BINDATA_DIRS := \
	js/*.js \
	rbac/policy.csv \
	statics/index.html \
	statics/css/* \
	statics/css/themes/*/* \
	statics/fonts/* \
	statics/img/* \
	statics/js/* \
	statics/schemas/* \
	statics/workflows/*.yaml \
	${EXTRABINDATA}

.PHONY: npm.install
npm.install:
	cd js && npm install

.PHONY: typescript
typescript: npm.install
	cd js && PATH=`npm bin`:$$PATH make all

.PHONY: .bindata
.bindata: builddep ebpf.build typescript
	$(call VENDOR_RUN,${GO_BINDATA_GITHUB}) go-bindata ${GO_BINDATA_FLAGS} -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go $(BINDATA_DIRS)
	gofmt -w -s statics/bindata.go

.PHONY: compile
compile:
	CGO_CFLAGS_ALLOW='.*' CGO_LDFLAGS_ALLOW='.*' $(BUILD_CMD) install \
		-ldflags="-X $(SKYDIVE_GITHUB_VERSION) \
		-B $(BUILD_ID)" \
		${GOFLAGS} -tags="${BUILD_TAGS}" ${VERBOSE_FLAGS} \
		${SKYDIVE_GITHUB}

.PHONY: compile.static
compile.static:
	$(BUILD_CMD) install \
		-ldflags "-X $(SKYDIVE_GITHUB_VERSION) \
		-extldflags \"-static $(STATIC_LIBS_ABS)\" \
		-B $(BUILD_ID)" \
		${VERBOSE_FLAGS} -tags "netgo ${BUILD_TAGS}" \
		-installsuffix netgo || true

.PHONY: skydive
skydive: govendor genlocalfiles dpdk.build contribs compile

.PHONY: skydive.clean
skydive.clean:
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

.PHONY: static
static: skydive.clean govendor genlocalfiles compile.static

.PHONY: contribs.clean
contribs.clean:
	$(MAKE) -C contrib/snort clean

.PHONY: contribs
contribs:
	$(MAKE) -C contrib/snort

.PHONY: dpdk.build
dpdk.build:
ifeq ($(WITH_DPDK), true)
	$(MAKE) -C dpdk
endif

.PHONY: dpdk.clean
dpdk.clean:
	$(MAKE) -C dpdk clean

.PHONY: ebpf.build
ebpf.build:
ifeq ($(WITH_EBPF), true)
	$(MAKE) -C probe/ebpf
endif

.PHONY: ebpf.clean
ebpf.clean:
	$(MAKE) -C probe/ebpf clean

.PHONY: test.functionals.clean
test.functionals.clean:
	rm -f tests/functionals

.PHONY: test.functionals.compile
test.functionals.compile: govendor genlocalfiles
	$(GOVENDOR) test -tags "${BUILD_TAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -c -o tests/functionals ./tests/

.PHONY: test.functionals.static
test.functionals.static: govendor genlocalfiles
	$(GOVENDOR) test -tags "netgo ${BUILD_TAGS} test" \
		-ldflags "-X $(SKYDIVE_GITHUB_VERSION) -extldflags \"-static $(STATIC_LIBS_ABS)\"" \
		-installsuffix netgo \
		${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} \
		-c -o tests/functionals ./tests/

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
	$(MAKE) test.functionals VERBOSE=true TIMEOUT=10m ARGS='-standalone -analyzer.topology.backend elasticsearch -analyzer.flow.backend elasticsearch' TEST_PATTERN=${TEST_PATTERN}

.PHONY: test
test: govendor genlocalfiles
ifeq ($(COVERAGE), true)
	set -v ; \
	$(GO_GET) github.com/mattn/goveralls ; \
	for pkg in ${UT_PACKAGES}; do \
		if [ -n "$$pkg" ]; then \
			coverfile="${COVERAGE_WD}/$$(echo $$pkg | tr / -).cover"; \
			$(GOVENDOR) test -tags "${BUILD_TAGS} test" -covermode=${COVERAGE_MODE} -coverprofile="$$coverfile" ${VERBOSE_FLAGS} -timeout ${TIMEOUT} $$pkg; \
		fi; \
	done
else
ifneq ($(TEST_PATTERN),)
	set -v ; \
	$(GOVENDOR) test -tags "${BUILD_TAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -test.run ${TEST_PATTERN} ${UT_PACKAGES}
else
	set -v ; \
	$(GOVENDOR) test -tags "${BUILD_TAGS} test" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} ${UT_PACKAGES}
endif
endif

.PHONY: govendor
govendor:
	$(GO_GET) github.com/kardianos/govendor
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
	@$(GO_GET) github.com/alecthomas/gometalinter
	@command -v $@ >/dev/null || gometalinter --install --update

.PHONY: gometalinter
gometalinter: $(LINTER_COMMANDS)

.PHONY: lint
lint: gometalinter
	@echo "+ $@"
	@gometalinter --disable=gotype --vendor -e '.*\.pb.go' --skip=statics/... --deadline 10m --sort=path ./... --json | tee lint.json || true

# dependency package need for building the project
.PHONY: builddep
builddep: govendor
	$(call VENDOR_BUILD,${PROTOC_GEN_GO_GITHUB})
	$(call VENDOR_BUILD,${GO_BINDATA_GITHUB})
	$(call VENDOR_BUILD,${EASYJSON_GITHUB})

.PHONY: genlocalfiles
genlocalfiles: .proto .bindata .easyjson

.PHONY: clean
clean: skydive.clean test.functionals.clean dpdk.clean contribs.clean
	grep path vendor/vendor.json | perl -pe 's|.*": "(.*?)".*|\1|g' | xargs -n 1 go clean -i >/dev/null 2>&1 || true

.PHONY: srpm
srpm:
	$(BOOTSTRAP) -s ${BOOTSTRAP_ARGS}

.PHONY: rpm
rpm:
	$(BOOTSTRAP) -b ${BOOTSTRAP_ARGS}

.PHONY: docker-image
docker-image: static
	cp $$GOPATH/bin/skydive contrib/docker/
	docker build -t ${DOCKER_IMAGE}:${DOCKER_TAG} -f contrib/docker/Dockerfile contrib/docker/

SKYDIVE_PROTO_FILES:= \
	${FLOW_PROTO_FILES} \
	${FILTERS_PROTO_FILES} \
	${HTTP_PROTO_FILES}

SKYDIVE_TAR_INPUT:= \
	vendor \
	statics/bindata.go \
	$(patsubst %.proto,%.pb.go,$(SKYDIVE_PROTO_FILES))

SKYDIVE_TAR:=${DESTDIR}/$(SKYDIVE_PKG).tar

define TAR_CMD
tar $1 -f $(SKYDIVE_TAR) --transform="s||$(SKYDIVE_PATH)|" $2
endef

.PHONY: localdist
localdist: govendor genlocalfiles
	git ls-files | $(call TAR_CMD,--create,--files-from -)
	$(call TAR_CMD,--append,$(SKYDIVE_TAR_INPUT))
	gzip -f $(SKYDIVE_TAR)

.PHONY: dist
dist: govendor genlocalfiles
	git archive -o $(SKYDIVE_TAR) --prefix $(SKYDIVE_PATH) HEAD
	$(call TAR_CMD,--append,$(SKYDIVE_TAR_INPUT))
	gzip -f $(SKYDIVE_TAR)
