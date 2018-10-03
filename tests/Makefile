SRC := tests.go k8s_test.go istio_test.go
OUT := ${PWD}/a.out
GOFLAGS := -race -gcflags='-N -l'

TEST_PATTERN ?= TestBookInfoScenario
BACKEND := memory
ARGS := \
	-test.run $(TEST_PATTERN) \
	-analyzer.topology.backend $(BACKEND) \
	-analyzer.flow.backend $(BACKEND) \
	-analyzer.topology.probes istio \
	-standalone

all: $(OUT)
	sudo $(OUT) $(ARGS)

debug: $(OUT)
	sudo $$(which dlv) exec $(OUT) -- $(ARGS)

build: $(OUT)

$(OUT): $(SRC)
	go test -o $(OUT) -c $(GOFLAGS) $(SRC)

clean:
	rm -rf $(OUT)



