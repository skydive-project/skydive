LLC ?= llc
CLANG ?= clang
DOCKER_FILE ?= Dockerfile
DOCKER_IMAGE ?= skydive/ebpf-builder
TOPLEVEL_GOPATH ?= $(GOPATH)
UID ?= $(shell id -u)

all: ebpf-build

ebpf-build: flow.o

%.o: %.c
	$(CLANG) \
		-I ../../vendor/github.com/iovisor/gobpf/elf \
		-I /usr/include/bcc/compat \
		-D__KERNEL__ -D__ASM_SYSREG_H -Wno-unused-value -Wno-pointer-sign \
		-Wno-compare-distinct-pointer-types \
		-fno-stack-protector \
		-O2 -emit-llvm -c $< -o -| $(LLC) -march=bpf -filetype=obj -o $@

clean:
	rm -f *.o

build-ebpf-docker-image:
	docker build -t $(DOCKER_IMAGE) -f $(DOCKER_FILE) .


docker-ebpf-build: build-ebpf-docker-image
	docker run --rm \
		--env UID=$(UID) \
		-v $(TOPLEVEL_GOPATH):/go \
		--workdir=/go/src/github.com/skydive-project/skydive/probe/ebpf \
		$(DOCKER_IMAGE) \
		sh -c "make ebpf-build && chown -R $(UID):$(UID) ."
