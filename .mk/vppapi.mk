.PHONY: .vppbinapi
.vppbinapi:
ifeq ($(WITH_VPP), true)
	go generate -tags "${BUILD_TAGS}" ${SKYDIVE_GITHUB}/topology/probes/vpp
endif

.PHONY: .vppbinapi.clean
.vppbinapi.clean:
	rm -rf topology/probes/vpp/bin_api
