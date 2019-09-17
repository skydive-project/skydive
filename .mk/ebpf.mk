.PHONY: .ebpf
.ebpf: vendor
ifeq ($(WITH_EBPF_DOCKER_BUILDER), true)
	$(MAKE) -C probe/ebpf docker-ebpf-build
else
	$(MAKE) -C probe/ebpf
endif

.PHONY: .ebpf.clean
.ebpf.clean:
	$(MAKE) -C probe/ebpf clean