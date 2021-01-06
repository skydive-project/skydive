GEN_DECODER_FILES = $(patsubst %.go,%_gendecoder.go,$(shell git grep //go:generate | grep "gendecoder" | grep -v "\.mk" | cut -d ":" -f 1))
GEN_DECODER_FILES += flow/flow.pb_gendecoder.go

%_gendecoder.go: %.go
	go generate -run gendecoder $<

.PHONY: .gendecoder
.gendecoder: $(GEN_DECODER_FILES)

.PHONY: .gendecoder.touch
.gendecoder.touch:
	@echo $(GEN_DECODER_FILES) | xargs touch

.PHONY: .gendecoder.clean
.gendecoder.clean:
	find . \( -name *_gendecoder.go ! -path './vendor/*' \) -exec rm {} \;
