
DLV_FLAGS=--check-go-version=false

ifeq (${DEBUG}, true)
define skydive_run
sudo -E $$(which dlv) $(DLV_FLAGS) exec $$(which skydive) -- $1 -c skydive.yml
endef
else
define skydive_run
sudo -E $$(which skydive) $1 -c skydive.yml
endef
endif

ifeq (${DEBUG}, true)
  GOFLAGS=-gcflags='-N -l'
  GO_BINDATA_FLAGS+=-debug
  export DEBUG
endif

.PHONY: debug.agent
run.agent:
	$(call skydive_run,agent)

.PHONY: debug.analyzer
run.analyzer:
	$(call skydive_run,analyzer)
