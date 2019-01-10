GOVENDOR:=${GOPATH}/bin/govendor

.PHONY: all
all: objectstore

.PHONY: clean
clean:
	rm -rf objectstore

objectstore: main.go
	$(GOVENDOR) build -o $$GOPATH/bin/objectstore-skydive $<
