.PHONY: contribs.clean
contribs.clean: contrib.snort.clean

.PHONY: contribs
contribs: contrib.snort

.PHONY: contribs.snort.clean
contrib.snort.clean:
	$(MAKE) -C contrib/snort clean

.PHONY: contrib.snort
contrib.snort:genlocalfiles
	$(MAKE) -C contrib/snort
