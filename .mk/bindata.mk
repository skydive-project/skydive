STATIC_FILES=$(shell find statics -type f \( ! -iname "bindata.go" ! -iname "bundle.js" \))
GO_BINDATA_GITHUB:=github.com/jteeuwen/go-bindata/go-bindata
EXTRABINDATA:=

ifeq ($(WITH_EBPF), true)
  EXTRABINDATA+=probe/ebpf/*.o
endif

BINDATA_DIRS := \
	js/*.js \
	rbac/policy.csv \
	statics/index.html \
	statics/css/* \
	statics/css/themes/*/* \
	statics/fonts/* \
	statics/img/* \
	statics/js/* \
	statics/schemas/* \
	statics/workflows/*.yaml \
	${EXTRABINDATA}

.PHONY: .bindata
.bindata: statics/bindata.go

statics/bindata.go: statics/js/bundle.js $(EBPF_PROBES) $(STATIC_FILES)
	echo $(STATIC_FILES)
	go run ${GO_BINDATA_GITHUB} ${GO_BINDATA_FLAGS} -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go $(BINDATA_DIRS)
	gofmt -w -s statics/bindata.go
