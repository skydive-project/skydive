STRINGER_FILES = $(patsubst %.go,%_string.go,$(shell git grep //go:generate | grep "stringer" | cut -d ":" -f 1))

%_string.go: %.go
	go generate -run stringer $<

.PHONY: .stringer
.stringer: $(STRINGER_FILES)

.PHONY: .stringer.clean
.stringer.clean:
	find . \( -name *_string.go ! -path './vendor/*' \) -exec rm {} \;

