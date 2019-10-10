.PHONY: contribs.clean
contribs.clean: contrib.exporters.clean contrib.snort.clean

.PHONY: contribs.test
contribs.test: contrib.exporters.test

.PHONY: contribs
contribs: contrib.exporters contrib.snort

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
