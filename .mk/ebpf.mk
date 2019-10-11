.PHONY: .ebpf
.ebpf: moddownload
ifeq ($(WITH_EBPF_DOCKER_BUILDER), true)
	$(MAKE) -C ebpf docker-ebpf-build
else
	$(MAKE) -C ebpf
endif

.PHONY: ebpf.test
ebpf.test: .ebpf ebpf/statics/bindata.go
	$(MAKE) -C ebpf ebpf-test

.PHONY: .ebpf.clean
.ebpf.clean:
	$(MAKE) -C ebpf clean
