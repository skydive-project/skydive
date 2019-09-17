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

ifeq ($(WITH_OPENCONTRAIL), true)
  BUILD_TAGS+=opencontrail
ifeq ($(OS_RHEL),Y)
  STATIC_LIBS+=libxml2.a
endif
ifeq ($(OS_DEB),Y)
  STATIC_LIBS+=libicuuc.a \
               libicudata.a \
               libxml2.a
endif
endif

STATIC_LIBS_ABS := $(addprefix $(STATIC_DIR)/,$(STATIC_LIBS))
STATIC_BUILD_TAGS := $(filter-out libvirt,$(BUILD_TAGS))

.PHONY: static
static: skydive.clean genlocalfiles
	$(MAKE) compile.static WITH_LIBVIRT_GO=false
	$(MAKE) contribs.static

.PHONY: compile.static
compile.static:
	CGO_CFLAGS_ALLOW='.*' CGO_LDFLAGS_ALLOW='.*' $(GO) install \
		-ldflags="${LDFLAGS} -B $(BUILD_ID) -X $(SKYDIVE_GITHUB_VERSION) '-extldflags=-static $(STATIC_LIBS_ABS)'" \
		${GOFLAGS} \
		${VERBOSE_FLAGS} -tags "netgo ${STATIC_BUILD_TAGS}" \
		-installsuffix netgo || true
