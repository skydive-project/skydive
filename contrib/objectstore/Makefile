GOVENDOR:=${GOPATH}/bin/govendor

.PHONY: all
all: objectstore

.PHONY: clean
clean:
	rm -rf objectstore

.PHONY: test
test:
	$(GOVENDOR) test -test.v ./...

objectstore: main.go
	$(GOVENDOR) build -o $$GOPATH/bin/objectstore-skydive $<
