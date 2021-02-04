SUBDIRS := \
	elasticsearch \
	kibana \
	skydive-analyzer \
	skydive-agent

.PHONY: install uninstall status
install uninstall status:
	for i in $(SUBDIRS); do \
		make -C $$i $@; \
	done

include ../../.mk/k8s.mk
