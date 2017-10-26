PROJECT_ROOT := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST))))/..)

DPDK_VERSION = 17.08
DPDK_DIR = dpdk-$(DPDK_VERSION)
export RTE_SDK = $(PROJECT_ROOT)/dpdk/$(DPDK_DIR)
export RTE_TARGET = x86_64-native-linuxapp-gcc

DPDK_INSTALL_DIR=$(RTE_TARGET)-install


.PHONY: all
all: $(DPDK_DIR)/$(DPDK_INSTALL_DIR)


$(DPDK_DIR):
	curl http://fast.dpdk.org/rel/dpdk-$(DPDK_VERSION).tar.xz | tar xJ

$(DPDK_DIR)/$(DPDK_INSTALL_DIR): $(DPDK_DIR)
	$(MAKE) -C $(DPDK_DIR) config T=$(RTE_TARGET)
	$(MAKE) -C $(DPDK_DIR) install T=$(RTE_TARGET) DESTDIR=$(DPDK_INSTALL_DIR)

clean:
	rm -rf $(DPDK_DIR)
