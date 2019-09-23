LLC ?= llc
CLANG ?= clang
DOCKER_FILE ?= Dockerfile
DOCKER_EBPF_BUILDER_IMAGE ?= skydive/ebpf-builder
UID ?= $(shell id -u)
GID ?= $(shell id -g)

.PHONY: clean build-ebpf-docker-image docker-ebpf-build

ebpf-build: flow-gre.o flow.o

all: clean docker-ebpf-build

%.o: %.c
	$(CLANG) \
		-I$${GOPATH}/pkg/mod/github.com/lebauce/gobpf@v0.0.0-20190909090614-f9e9df81702a/elf \
		-D__KERNEL__ -D__ASM_SYSREG_H -D__TARGET_ARCH_BPF -Wno-unused-value -Wno-pointer-sign \
		-Wno-compare-distinct-pointer-types \
		-Wno-gnu-variable-sized-type-not-at-end \
		-Wno-address-of-packed-member -Wno-tautological-compare \
		-Wno-compare-distinct-pointer-types -Wno-unknown-warning-option \
		-fno-stack-protector \
		-fno-jump-tables \
		-fno-common \
		-O2 -g -emit-llvm -c $< -o ebpf.bc
	$(LLC) -march=bpf -filetype=obj -mattr=dwarfris -o $@ ebpf.bc
	rm -f ebpf.bc

clean:
	rm -f probe/ebpf/*.o

build-ebpf-docker-image:
	docker build -t $(DOCKER_EBPF_BUILDER_IMAGE) -f $(DOCKER_FILE) .

pull-ebpf-docker-image:
	docker pull $(DOCKER_EBPF_BUILDER_IMAGE):latest

docker-ebpf-build: pull-ebpf-docker-image
	docker run --rm \
		--env=UID=$(UID) \
		--env=GID=$(GID) \
		--env=GOPATH=/go \
		-v $(GOPATH):/go \
		-v $(GOPATH)/pkg:/go/pkg \
		--workdir=/go/src/github.com/skydive-project/skydive/probe/ebpf \
		$(DOCKER_EBPF_BUILDER_IMAGE) \
		sh -c "make ebpf-build && chown -R $(UID):$(GID) ."
