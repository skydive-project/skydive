GOVENDOR:=${GOPATH}/bin/govendor

.PHONY: all
all: snortSkydive

.PHONY: clean
clean:
	rm -rf snortSkydive

snortSkydive: snortSkydive.go
	$(GOVENDOR) build -o $$GOPATH/bin/snort-skydive $<
