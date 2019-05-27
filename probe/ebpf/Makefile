LLC ?= llc
CLANG ?= clang
DOCKER_FILE ?= Dockerfile
DOCKER_IMAGE ?= skydive/ebpf-builder
TOPLEVEL_GOPATH ?= $(GOPATH)
UID ?= $(shell id -u)
GID ?= $(shell id -g)
GOVENDOR:=$(TOPLEVEL_GOPATH)/bin/govendor

.PHONY: all ebpf-build clean build-ebpf-docker-image docker-ebpf-build

all: clean ebpf-build

ebpf-build: flow-gre.o flow.o

%.o: %.c
	$(CLANG) \
		-I ../../vendor/github.com/iovisor/gobpf/elf \
		-D__KERNEL__ -D__ASM_SYSREG_H -Wno-unused-value -Wno-pointer-sign \
		-Wno-compare-distinct-pointer-types \
		-Wno-gnu-variable-sized-type-not-at-end \
		-Wno-address-of-packed-member -Wno-tautological-compare \
		-Wno-compare-distinct-pointer-types \
		-fno-stack-protector \
		-fno-jump-tables \
		-fno-common \
		-O2 -g -emit-llvm -c $< -o ebpf.bc
	$(LLC) -march=bpf -filetype=obj -mattr=dwarfris -o $@ ebpf.bc
	rm -f ebpf.bc

clean:
	rm -f *.o

build-ebpf-docker-image:
	docker build -t $(DOCKER_IMAGE) -f $(DOCKER_FILE) .

docker-ebpf-build: build-ebpf-docker-image
	docker run --rm \
		--env=UID=$(UID) \
		--env=GID=$(GID) \
		-v $(TOPLEVEL_GOPATH):/go \
		--workdir=/go/src/github.com/skydive-project/skydive/probe/ebpf \
		$(DOCKER_IMAGE) \
		sh -c "make ebpf-build && chown -R $(UID):$(GID) ."
