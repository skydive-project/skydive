
.PHONY: all
all: snortSkydive

.PHONY: clean
clean:
	rm -rf snortSkydive

snortSkydive: snortSkydive.go
	govendor build -o $$GOPATH/bin/snort-skydive $<
