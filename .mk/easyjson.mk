GEN_EASYJSON_FILES = $(patsubst %.go,%_easyjson.go,$(shell git grep //go:generate | grep "easyjson" | grep -v Makefile | cut -d ":" -f 1))
GEN_EASYJSON_FILES += flow/flow.pb_easyjson.go

%_easyjson.go: %.go
	go generate -run easyjson $<

.PHONY: .easyjson
.easyjson: flow/flow.pb_easyjson.go $(GEN_EASYJSON_FILES)

.PHONY: .easyjson.touch
.easyjson.touch:
	@echo $(GEN_EASYJSON_FILES) | xargs touch

.PHONY: .easyjson.clean
.easyjson.clean:
	find . \( -name *_easyjson.go ! -path './vendor/*' \) -exec rm {} \;

