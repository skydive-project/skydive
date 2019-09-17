.PHONY: contribs.clean
contribs.clean: contrib.exporters.clean contrib.snort.clean contrib.collectd.clean

.PHONY: contribs.test
contribs.test: contrib.exporters.test

.PHONY: contribs
contribs: contrib.exporters contrib.snort contrib.collectd

.PHONY: contribs.static
contribs.static:
	$(MAKE) -C contrib/exporters static

.PHONY: contrib.exporters.clean
contrib.exporters.clean:
	$(MAKE) -C contrib/exporters clean

.PHONY: contrib.exporters
contrib.exporters: genlocalfiles
	$(MAKE) -C contrib/exporters

.PHONY: contrib.exporters.test
contrib.exporters.test: genlocalfiles
	$(MAKE) -C contrib/exporters test

.PHONY: contribs.snort.clean
contrib.snort.clean:
	$(MAKE) -C contrib/snort clean

.PHONY: contrib.snort
contrib.snort:genlocalfiles
	$(MAKE) -C contrib/snort

.PHONY: contrib.collectd.clean
contrib.collectd.clean:
	$(MAKE) -C contrib/collectd clean

.PHONY: contrib.collectd
contrib.collectd: genlocalfiles
	$(MAKE) -C contrib/collectd
