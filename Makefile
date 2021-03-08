define VERSION_CMD =
eval ' \
	define=""; \
	version=`git describe --abbrev=0 --tags | tr -d "[a-z]"` ; \
	[ -z "$$version" ] && version="unknown"; \
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

VERSION?=$(shell $(VERSION_CMD))
GO?=go
BUILD_ID:=$(shell echo 0x$$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \n'))
SKYDIVE_GITHUB:=github.com/skydive-project/skydive
SKYDIVE_PKG:=skydive-${VERSION}
SKYDIVE_PATH:=$(SKYDIVE_PKG)/src/$(SKYDIVE_GITHUB)/
SKYDIVE_GITHUB_VERSION:=$(SKYDIVE_GITHUB)/version.Version=${VERSION}
VERBOSE_FLAGS?=
VERBOSE?=true
ifeq ($(VERBOSE), true)
  VERBOSE_FLAGS+=-v
endif
BUILD_TAGS?=$(TAGS)

WITH_LXD?=true
WITH_OPENCONTRAIL?=true
WITH_LIBVIRT_GO?=true
WITH_EBPF_DOCKER_BUILDER?=true
WITH_VPP?=false
WITH_PACKETINJECT?=true

EXTRA_BUILD_TARGET=

ifeq ($(WITH_DPDK), true)
  BUILD_TAGS+=dpdk
endif

ifeq ($(WITH_EBPF), true)
  BUILD_TAGS+=ebpf
  EXTRA_BUILD_TARGET+=.ebpf
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
  BUILD_TAGS+=insanelock
endif

ifeq ($(WITH_K8S), true)
  BUILD_TAGS+=k8s
endif

ifeq ($(WITH_ISTIO), true)
  BUILD_TAGS+=k8s istio
endif

ifeq ($(WITH_LXD), true)
  BUILD_TAGS+=lxd
endif

ifeq ($(WITH_LIBVIRT_GO), true)
  BUILD_TAGS+=libvirt
endif

ifeq ($(WITH_VPP), true)
  BUILD_TAGS+=vpp
endif

ifeq ($(WITH_OPENCONTRAIL), true)
  BUILD_TAGS+=opencontrail
endif

ifeq ($(WITH_PACKETINJECT), true)
  BUILD_TAGS+=packetinject
endif


include .mk/ebpf.mk
include .mk/api.mk
include .mk/bench.mk
include .mk/ui.mk
include .mk/bindata.mk
include .mk/check.mk
include .mk/contrib.mk
include .mk/debug.mk
include .mk/dist.mk
include .mk/easyjson.mk
include .mk/gendecoder.mk
include .mk/proto.mk
include .mk/static.mk
include .mk/tests.mk
include .mk/vppapi.mk
include .mk/swagger.mk
include .mk/stringer.mk

.DEFAULT_GOAL := all

.PHONY: all install
all install: skydive

.PHONY: version
version:
	@echo -n ${VERSION}

define GOCOMPILE
CGO_CFLAGS_ALLOW='.*' CGO_LDFLAGS_ALLOW='.*' $(GO) $1 \
                -ldflags="${LDFLAGS} -B $(BUILD_ID) -X $(SKYDIVE_GITHUB_VERSION)" \
                ${GOFLAGS} -tags="${BUILD_TAGS}" ${VERBOSE_FLAGS} \
                ${SKYDIVE_GITHUB}
endef

.PHONY: .build
.build:
	$(call GOCOMPILE,build)

.PHONY: build
build: gopath moddownload genlocalfiles .build

.PHONY: .install
.install:
	$(call GOCOMPILE,install)

.PHONY: skydive
skydive: gopath moddownload genlocalfiles .install

.PHONY: skydive.clean
skydive.clean:
	go clean -i $(SKYDIVE_GITHUB)

.PHONY: moddownload
moddownload:
ifneq ($(OFFLINE), true)
	go mod download
endif

.PHONY: genlocalfiles
genlocalfiles: $(EXTRA_BUILD_TARGET) .proto .typescript .bindata .gendecoder .easyjson .vppbinapi

.PHONY: touchlocalfiles
touchlocalfiles: .proto.touch .typescript.touch .bindata.touch .gendecoder.touch .easyjson.touch

.PHONY: clean
clean: skydive.clean test.functionals.clean contribs.clean .ebpf.clean .easyjson.clean .proto.clean .gendecoder.clean .typescript.clean .vppbinapi.clean swagger.clean
	go clean -i >/dev/null 2>&1 || true

.PHONY: docker
docker:
	docker build . -t $(DOCKER_IMAGE):$(DOCKER_TAG)
