BUILD := \
	allinone \
	awsflowlogs \
	core \
	dummy \
	secadvisor \
	vpclogs \

STATIC := \
	allinone \

TEST := \
	secadvisor \

.PHONY: all
all:
	for i in $(BUILD); do $(MAKE) -C $$i; done

.PHONY: clean
clean:
	for i in $(BUILD); do $(MAKE) -C $$i clean; done

.PHONY: test
test:
	for i in $(TEST); do $(MAKE) -C $$i test; done

.PHONY: static
static:
	for i in $(STATIC); do $(MAKE) -C $$i static; done
