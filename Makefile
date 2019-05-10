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

define VENDOR_RUN
mv ${BUILD_TOOLS} bin || true; \
ln -s vendor src || mv vendor src; \
cd src/$1; \
(unset GOARCH; unset CC; GOPATH=$$GOPATH/src/${SKYDIVE_GITHUB} go install $1); \
cd -; \
mv bin ${BUILD_TOOLS}; \
unlink src || mv src vendor;
endef

define PROTOC_GEN
$(call VENDOR_RUN,${PROTOC_GEN_GOFAST_GITHUB})
$(call VENDOR_RUN,${PROTOC_GEN_GO_GITHUB}) protoc -Ivendor -I. --plugin=${BUILD_TOOLS}/protoc-gen-gogofaster --gogofaster_out $$GOPATH/src $1
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
BUILD_TOOLS:=${GOPATH}/.skydive-build-tool
GO_BINDATA_GITHUB:=github.com/jteeuwen/go-bindata/go-bindata
PROTOC_GEN_GO_GITHUB:=github.com/golang/protobuf/protoc-gen-go
PROTOC_GEN_GOFAST_GITHUB:=github.com/gogo/protobuf/protoc-gen-gogofaster
PROTEUS_GITHUB:=gopkg.in/src-d/proteus.v1/cli/proteus
EASYJSON_GITHUB:=github.com/mailru/easyjson/easyjson
EASYJSON_FILES_ALL=flow/flow.pb.go
EASYJSON_FILES_TAG=\
	flow/storage/elasticsearch/elasticsearch.go \
	flow/storage/orientdb/orientdb.go \
	graffiti/graph/elasticsearch.go \
	sflow/sflow.go \
	topology/metrics.go \
	topology/probes/docker/metadata.go \
	topology/probes/lldp/metadata.go \
	topology/probes/netlink/metadata.go \
	topology/probes/nsm/nsm.go \
	topology/probes/ovsdb/ovsdb.go \
	topology/probes/socketinfo/connection.go
EASYJSON_FILES_TAG_OPENCONTRAIL=\
	topology/probes/opencontrail/routing_table.go
VPPBINAPI_GITHUB:=git.fd.io/govpp.git/cmd/binapi-generator
VERBOSE_FLAGS?=-v
VERBOSE_TESTS_FLAGS?=-test.v
VERBOSE?=true
ifeq ($(VERBOSE), false)
  VERBOSE_FLAGS:=
  VERBOSE_TESTS_FLAGS:=
endif
ifeq ($(COVERAGE), true)
  TEST_COVERPROFILE?=../functionals.cover
  EXTRA_ARGS+=-test.coverprofile=${TEST_COVERPROFILE}
endif
TIMEOUT?=1m
TEST_PATTERN?=
UT_PACKAGES?=$(shell $(GOVENDOR) list -no-status +local | grep -Ev '/tests|/contrib')
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
WITH_LIBVIRT_GO?=true
WITH_EBPF_DOCKER_BUILDER?=false
WITH_VPP?=false

export PATH:=$(BUILD_TOOLS):$(PATH)

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

ifeq ($(WITH_CDD), true)
  BUILD_TAGS+=cdd
endif

ifeq ($(WITH_MUTEX_DEBUG), true)
  BUILD_TAGS+=mutexdebug
endif

ifeq ($(WITH_K8S), true)
  BUILD_TAGS+=k8s
  ANALYZER_TEST_PROBES+=k8s
endif

ifeq ($(WITH_ISTIO), true)
  BUILD_TAGS+=k8s istio
  ANALYZER_TEST_PROBES+=istio
endif

ifeq ($(WITH_OVN), true)
  ANALYZER_TEST_PROBES+=ovn
endif

ifeq ($(WITH_HELM), true)
  BUILD_TAGS+=helm
endif

ifeq ($(WITH_OPENCONTRAIL), true)
  BUILD_TAGS+=opencontrail
  AGENT_TEST_EXTRA_PROBES+=opencontrail
ifeq ($(OS_RHEL),Y)
  STATIC_LIBS+=libxml2.a
endif
ifeq ($(OS_DEB),Y)
  STATIC_LIBS+=libicuuc.a \
               libicudata.a \
               libxml2.a
endif
endif

ifeq ($(WITH_LXD), true)
  BUILD_TAGS+=lxd
endif

ifeq ($(WITH_LIBVIRT_GO), true)
  BUILD_TAGS+=libvirt
endif

ifeq ($(WITH_VPP), true)
  BUILD_TAGS+=vpp
  AGENT_TEST_EXTRA_PROBES+=vpp
endif

ifeq (${DEBUG}, true)
GOFLAGS=-gcflags='-N -l'
GO_BINDATA_FLAGS+=-debug
export DEBUG
endif

comma:= ,
empty:=
space:= $(empty) $(empty)
EXTRA_ARGS+=-analyzer.topology.probes=$(subst $(space),$(comma),$(ANALYZER_TEST_PROBES)) -agent.topology.probes=$(subst $(space),$(comma),$(AGENT_TEST_EXTRA_PROBES))
STATIC_LIBS_ABS := $(addprefix $(STATIC_DIR)/,$(STATIC_LIBS))
STATIC_BUILD_TAGS := $(filter-out libvirt,$(BUILD_TAGS))

.PHONY: all install
all install: skydive

.PHONY: version
version:
	@echo -n ${VERSION}

skydive.yml: etc/skydive.yml.default
	[ -e $@ ] || cp $< $@

DLV_FLAGS=--check-go-version=false

ifeq (${DEBUG}, true)
define skydive_run
sudo -E $$(which dlv) $(DLV_FLAGS) exec $$(which skydive) -- $1 -c skydive.yml
endef
else
define skydive_run
sudo -E $$(which skydive) $1 -c skydive.yml
endef
endif

.PHONY: debug.agent
run.agent:
	$(call skydive_run,agent)

.PHONY: debug.analyzer
run.analyzer:
	$(call skydive_run,analyzer)

%.pb.go: %.proto
	$(call PROTOC_GEN,$<)

flow/flow.pb.go: flow/flow.proto filters/filters.proto
	$(call PROTOC_GEN,flow/flow.proto)

	# always export flow.ParentUUID as we need to store this information to know
	# if it's a Outer or Inner packet.
	sed -e 's/ParentUUID\(.*\),omitempty\(.*\)/ParentUUID\1\2/' \
		-e 's/Protocol\(.*\),omitempty\(.*\)/Protocol\1\2/' \
		-e 's/ICMPType\(.*\),omitempty\(.*\)/ICMPType\1\2/' \
		-e 's/int64\(.*\),omitempty\(.*\)/int64\1\2/' \
		-i $@
	# add omitempty to RTT as it is not always filled
	sed -e 's/json:"RTT"/json:"RTT,omitempty"/' -i $@
	# do not export LastRawPackets used internally
	sed -e 's/json:"LastRawPackets,omitempty"/json:"-"/g' -i $@
	# add flowState to flow generated struct
	sed -e 's/type Flow struct {/type Flow struct { XXX_state flowState `json:"-"`/' -i $@
	# to fix generated layers import
	sed -e 's/layers "flow\/layers"/layers "github.com\/skydive-project\/skydive\/flow\/layers"/' -i $@
	sed -e 's/type FlowMetric struct {/\/\/ gendecoder\ntype FlowMetric struct {/' -i $@
	sed -e 's/type FlowLayer struct {/\/\/ gendecoder\ntype FlowLayer struct {/' -i $@
	sed -e 's/type TransportLayer struct {/\/\/ gendecoder\ntype TransportLayer struct {/' -i $@
	sed -e 's/type ICMPLayer struct {/\/\/ gendecoder\ntype ICMPLayer struct {/' -i $@
	sed -e 's/type IPMetric struct {/\/\/ gendecoder\ntype IPMetric struct {/' -i $@
	sed -e 's/type TCPMetric struct {/\/\/ gendecoder\ntype TCPMetric struct {/' -i $@
	gofmt -s -w $@

flow/layers/generated.proto: flow/layers/layers.go
	$(call VENDOR_RUN,${PROTEUS_GITHUB}) proteus proto -f $${GOPATH}/src -p github.com/skydive-project/skydive/flow/layers
	sed -e 's/^package .*;/package layers;/' -i $@
	sed -e 's/^option go_package = "layers"/option go_package = "github.com\/skydive-project\/skydive\/flow\/layers"/' -i $@
	sed -e 's/^message Layer/message /' -i $@
	sed -e 's/option (gogoproto.typedecl) = false;//' -i $@
	sed 's/\((gogoproto\.customname) = "\([^\"]*\)"\)/\1, (gogoproto.jsontag) = "\2,omitempty"/' -i $@

websocket/structmessage.pb.go: websocket/structmessage.proto
	$(call PROTOC_GEN,$<)

	sed -e 's/type StructMessage struct {/type StructMessage struct { XXX_state structMessageState `json:"-"`/' -i websocket/structmessage.pb.go
	gofmt -s -w $@

.proto: govendor flow/layers/generated.pb.go flow/flow.pb.go filters/filters.pb.go websocket/structmessage.pb.go

.PHONY: .proto.clean
.proto.clean:
	find . \( -name *.pb.go ! -path './vendor/*' \) -exec rm {} \;
	rm -rf flow/layers/generated.proto

GEN_EASYJSON_FILES_ALL := $(patsubst %.go,%_easyjson.go,$(EASYJSON_FILES_ALL))
GEN_EASYJSON_FILES_TAG := $(patsubst %.go,%_easyjson.go,$(EASYJSON_FILES_TAG))
GEN_EASYJSON_FILES_TAG_OPENCONTRAIL := $(patsubst %.go,%_easyjson.go,$(EASYJSON_FILES_TAG_OPENCONTRAIL))

%_easyjson.go: %.go
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson $<

flow/flow.pb_easyjson.go: flow/flow.pb.go
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson -all $<

topology/probes/lldp/lldp_easyjson.go: topology/probes/lldp/lldp.go
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson -build_tags linux $<

topology/probes/netlink/netlink_easyjson.go: topology/probes/netlink/netlink.go
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson -build_tags linux $<

topology/probes/socketinfo/connection_easyjson.go: topology/probes/socketinfo/connection.go
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson -build_tags linux $<

topology/probes/opencontrail/routing_table_easyjson.go: $(EASYJSON_FILES_TAG_OPENCONTRAIL)
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson -build_tags opencontrail $<

topology/probes/ovsdb/ovsdb.pb_easyjson.go: topology/probes/ovsdb/ovsdb.go
	$(call VENDOR_RUN,${EASYJSON_GITHUB}) easyjson $<

.PHONY: .easyjson
.easyjson: flow/flow.pb_easyjson.go $(GEN_EASYJSON_FILES_TAG) $(GEN_EASYJSON_FILES_TAG_OPENCONTRAIL)

.PHONY: .easyjson.clean
.easyjson.clean:
	find . \( -name *_easyjson.go ! -path './vendor/*' \) -exec rm {} \;

.PHONY: .binapigenerator
binapigenerator: vendor/${VPPBINAPI_GITHUB}
	$(call VENDOR_RUN,${VPPBINAPI_GITHUB})

.PHONY: .vppbinapi.clean
.vppbinapi.clean:
	rm -rf topology/probes/vpp/bin_api

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

.PHONY: .typescript
.typescript: npm.install
	cd js && PATH=`npm bin`:$$PATH make all

.PHONY: .typescript.clean
.typescript.clean: npm.install
	cd js && PATH=`npm bin`:$$PATH make clean

.PHONY: .bindata
.bindata: statics/bindata.go

statics/bindata.go: .typescript ebpf.build $(shell find statics -type f \( ! -iname "bindata.go" \))
	$(call VENDOR_RUN,${GO_BINDATA_GITHUB}) go-bindata ${GO_BINDATA_FLAGS} -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go $(BINDATA_DIRS)
	gofmt -w -s statics/bindata.go

.PHONY: .vppbinapi
.vppbinapi: binapigenerator
	$(GOVENDOR) generate -tags "${BUILD_TAGS}" ${SKYDIVE_GITHUB}/topology/probes/vpp

.PHONY: .go-generate
.go-generate:
	$(GOVENDOR) generate -tags="${BUILD_TAGS}" ${SKYDIVE_GITHUB}/...

.PHONY: .go-generate.clean
.go-generate.clean:
	find . \( -name *_gendecoder.go ! -path './vendor/*' \) -exec rm {} \;

.PHONY: compile
compile:
	CGO_CFLAGS_ALLOW='.*' CGO_LDFLAGS_ALLOW='.*' $(BUILD_CMD) install \
		-ldflags="${LDFLAGS} -B $(BUILD_ID) -X $(SKYDIVE_GITHUB_VERSION)" \
		${GOFLAGS} -tags="${BUILD_TAGS}" ${VERBOSE_FLAGS} \
		${SKYDIVE_GITHUB}

.PHONY: compile.static
compile.static:
	$(BUILD_CMD) install \
		-ldflags="${LDFLAGS} -B $(BUILD_ID) -X $(SKYDIVE_GITHUB_VERSION) '-extldflags=-static $(STATIC_LIBS_ABS)'" \
		${GOFLAGS} \
		${VERBOSE_FLAGS} -tags "netgo ${STATIC_BUILD_TAGS}" \
		-installsuffix netgo || true

.PHONY: skydive
skydive: govendor genlocalfiles dpdk.build compile

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
	$(GOVENDOR) test -bench=. ${SKYDIVE_GITHUB}/flow

.PHONY: static
static: skydive.clean govendor genlocalfiles
	$(MAKE) compile.static WITH_LIBVIRT_GO=false

.PHONY: contribs.clean
contribs.clean: contrib.objectstore.clean contrib.snort.clean contrib.collectd.clean

.PHONY: contribs.test
contribs.test: contrib.objectstore.test

.PHONY: contribs
contribs: contrib.objectstore contrib.snort contrib.collectd

.PHONY: contrib.objectstore.clean
contrib.objectstore.clean:
	$(MAKE) -C contrib/objectstore clean

.PHONY: contrib.objectstore
contrib.objectstore: govendor genlocalfiles
	$(MAKE) -C contrib/objectstore

.PHONY: contrib.objectstore.test
contrib.objectstore.test: govendor genlocalfiles
	$(MAKE) -C contrib/objectstore test

.PHONY: contribs.snort.clean
contrib.snort.clean:
	$(MAKE) -C contrib/snort clean

.PHONY: contrib.snort
contrib.snort:govendor genlocalfiles
	$(MAKE) -C contrib/snort

.PHONY: contrib.collectd.clean
contrib.collectd.clean:
	$(MAKE) -C contrib/collectd clean

.PHONY: contrib.collectd
contrib.collectd: govendor genlocalfiles
	$(MAKE) -C contrib/collectd

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
ifeq ($(WITH_EBPF_DOCKER_BUILDER), true)
	$(MAKE) -C probe/ebpf clean docker-ebpf-build
else
	$(MAKE) -C probe/ebpf clean ebpf-build
endif
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
	$(GOVENDOR) test -tags "netgo ${STATIC_BUILD_TAGS} test" \
		-ldflags "${LDFLAGS} -X $(SKYDIVE_GITHUB_VERSION) -extldflags \"-static $(STATIC_LIBS_ABS)\"" \
		-installsuffix netgo \
		${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} \
		-c -o tests/functionals ./tests/

ifeq (${DEBUG}, true)
define functionals_run
cd tests && sudo -E $$(which dlv) $(DLV_FLAGS) exec ./functionals -- $1
endef
else
define functionals_run
cd tests && sudo -E ./functionals $1
endef
endif

.PHONY: test.functionals.run
test.functionals.run:
	cd tests && sudo -E ./functionals ${VERBOSE_TESTS_FLAGS} -test.run "${TEST_PATTERN}" -test.timeout ${TIMEOUT} ${ARGS} ${EXTRA_ARGS}

.PHONY: tests.functionals.all
test.functionals.all: test.functionals.compile
	$(MAKE) TIMEOUT="8m" ARGS="${ARGS}" test.functionals.run EXTRA_ARGS="${EXTRA_ARGS}"

.PHONY: test.functionals.batch
test.functionals.batch: test.functionals.compile
	set -e ; $(MAKE) ARGS="${ARGS} " test.functionals.run EXTRA_ARGS="${EXTRA_ARGS}" TEST_PATTERN="${TEST_PATTERN}"

.PHONY: test.functionals
test.functionals: test.functionals.compile
	for functest in ${FUNC_TESTS} ; do \
		$(MAKE) ARGS="-test.run $$functest$$\$$ ${ARGS}" test.functionals.run EXTRA_ARGS="${EXTRA_ARGS}"; \
	done

.PHONY: functional
functional:
	$(MAKE) test.functionals VERBOSE=true TIMEOUT=10m ARGS='-standalone -analyzer.topology.backend elasticsearch -analyzer.flow.backend elasticsearch' TEST_PATTERN="${TEST_PATTERN}"

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
	$(GOVENDOR) test -tags "${BUILD_TAGS} test" -ldflags="${LDFLAGS}" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} -test.run ${TEST_PATTERN} ${UT_PACKAGES}
else
	set -v ; \
	$(GOVENDOR) test -tags "${BUILD_TAGS} test" -ldflags="${LDFLAGS}" ${GOFLAGS} ${VERBOSE_FLAGS} -timeout ${TIMEOUT} ${UT_PACKAGES}
endif
endif

.PHONY: govendor
govendor:
	$(GO_GET) github.com/kardianos/govendor
	$(GOVENDOR) sync
	patch -p0 < dpdk/dpdk.govendor.patch
	rm -rf vendor/github.com/weaveworks/tcptracer-bpf/vendor/github.com/
	find vendor/github.com/docker/go-connections -name "*.go" | xargs -n 1 perl -i -pe 's|github.com/Sirupsen|github.com/sirupsen|g'

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
	@command -v $@ >/dev/null || gometalinter --install

.PHONY: gometalinter
gometalinter: $(LINTER_COMMANDS)

.PHONY: lint
lint: gometalinter
	@echo "+ $@"
	gometalinter --disable=gotype ${GOMETALINTER_FLAGS} --vendor -e '.*\.pb.go' -e '.*\._easyjson.go' -e '.*\._gendecoder.go' -e 'statics/bindata.go' --skip=statics/... --deadline 10m --sort=path ./... --json | tee lint.json || true

.PHONY: genlocalfiles
genlocalfiles: .proto .go-generate .bindata .easyjson .vppbinapi

.PHONY: clean
clean: skydive.clean test.functionals.clean dpdk.clean contribs.clean ebpf.clean .easyjson.clean .proto.clean .go-generate.clean .typescript.clean .vppbinapi.clean
	grep path vendor/vendor.json | perl -pe 's|.*": "(.*?)".*|\1|g' | xargs -n 1 go clean -i >/dev/null 2>&1 || true

.PHONY: srpm
srpm:
	$(BOOTSTRAP) -s ${BOOTSTRAP_ARGS}

.PHONY: rpm
rpm:
	$(BOOTSTRAP) -b ${BOOTSTRAP_ARGS}

.PHONY: docker-image
docker-image: static
	cp $$GOPATH/bin/skydive contrib/docker/skydive.$$(uname -m)
	docker build -t ${DOCKER_IMAGE}:${DOCKER_TAG} --build-arg ARCH=$$(uname -m) -f contrib/docker/Dockerfile contrib/docker/

SKYDIVE_PROTO_FILES:= \
	flow/flow.proto \
	filters/filters.proto \
	websocket/structmessage.proto \
	flow/layers/generated.proto

SKYDIVE_TAR_INPUT:= \
	vendor \
	statics/bindata.go \
	$(patsubst %.proto,%.pb.go,$(SKYDIVE_PROTO_FILES)) \
	$(GEN_EASYJSON_FILES_ALL) \
	$(GEN_EASYJSON_FILES_TAG) \
	$(GEN_EASYJSON_FILES_TAG_OPENCONTRAIL)

SKYDIVE_TAR:=${DESTDIR}/$(SKYDIVE_PKG).tar

define TAR_CMD
tar $1 -f $(SKYDIVE_TAR) --transform="s||$(SKYDIVE_PATH)|" $2
endef

define TAR_APPEND
$(call TAR_CMD,--append,$(SKYDIVE_TAR_INPUT))
find -type f -name *_gendecoder.go -printf '%P\n' -exec tar --append -f $(SKYDIVE_TAR) --transform="s||$(SKYDIVE_PATH)|" {} \;
endef

.PHONY: localdist
localdist: govendor genlocalfiles
	git ls-files | $(call TAR_CMD,--create,--files-from -)
	$(call TAR_APPEND,)
	gzip -f $(SKYDIVE_TAR)

.PHONY: dist
dist: govendor genlocalfiles
	git archive -o $(SKYDIVE_TAR) --prefix $(SKYDIVE_PATH) HEAD
	$(call TAR_APPEND,)
	gzip -f $(SKYDIVE_TAR)
