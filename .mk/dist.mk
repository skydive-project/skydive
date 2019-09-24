BOOTSTRAP:=contrib/packaging/rpm/generate-skydive-bootstrap.sh
BOOTSTRAP_ARGS?=
DOCKER_IMAGE?=skydive/skydive
DOCKER_TAG?=devel
DESTDIR?=$(shell pwd)
SKYDIVE_TAR_INPUT:= \
	vendor \
	statics/bindata.go \
	$(GEN_PROTO_FILES) \
	$(GEN_DECODER_FILES) \
	$(GEN_EASYJSON_FILES)

SKYDIVE_TAR:=${DESTDIR}/$(SKYDIVE_PKG).tar

define TAR_CMD
tar $1 -f $(SKYDIVE_TAR) --transform="s||$(SKYDIVE_PATH)|" $2
endef

define TAR_APPEND
$(call TAR_CMD,--append,$(SKYDIVE_TAR_INPUT))
endef

.PHONY: vendor
vendor: genlocalfiles
ifeq (${GO111MODULE}, on)
	go mod vendor
endif

.PHONY: localdist
localdist: vendor
	git ls-files | $(call TAR_CMD,--create,--files-from -)
	$(call TAR_APPEND,)
	gzip -f $(SKYDIVE_TAR)

.PHONY: dist
dist: vendor
	git archive -o $(SKYDIVE_TAR) --prefix $(SKYDIVE_PATH) HEAD
	$(call TAR_APPEND,)
	gzip -f $(SKYDIVE_TAR)

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
