BUILD := \
	core \
	secadvisor \
	awsflowlogs \
	vpclogs \
	dummy \
	uber \

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
