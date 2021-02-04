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

.PHONY: static install.static
static install.static: skydive.static

define GOCOMPILE_STATIC
	CGO_CFLAGS_ALLOW='.*' CGO_LDFLAGS_ALLOW='.*' $(GO) $1 \
		-ldflags="${LDFLAGS} -B $(BUILD_ID) -X $(SKYDIVE_GITHUB_VERSION) '-extldflags=-static $(STATIC_LIBS_ABS)'" \
		${GOFLAGS} \
		${VERBOSE_FLAGS} -tags "osusergo netgo ${STATIC_BUILD_TAGS}" \
		-installsuffix netgo || true
endef

.PHONY: .build.static
.build.static:
	$(call GOCOMPILE_STATIC,build)

.PHONY: build.static
build.static: genlocalfiles .build.static

.PHONY: .install.static
.install.static:
	$(call GOCOMPILE_STATIC,install)

.PHONY: skydive.static
skydive.static: genlocalfiles
	make .install.static WITH_LIBVIRT_GO=false
