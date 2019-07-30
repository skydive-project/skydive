PROG := secadvisor

SRCS := \
	main.go \
	transform.go \

all: LDFLAGS:=
static: LDFLAGS:=-ldflags '-extldflags "-static"'

.PHONY: all static
all static:
	go build -o $(PROG) $(LDFLAGS) $(SRCS)

.PHONY: test
test:
	go test

.PHONY: clean
clean:
	rm -rf $(PROG)
