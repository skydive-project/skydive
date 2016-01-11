all: build test

build:
	go build -v

test:
	go test -covermode=count -coverprofile=coverage.out -test.short -v

test-all:
	go test -covermode=count -coverprofile=coverage.out -v

