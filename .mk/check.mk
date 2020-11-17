
.PHONY: gopath
gopath:

ifndef GOPATH
	$(error GOPATH is undefined, Skydive requires to have GOPATH defined to build)
endif

.PHONY: fmt
fmt: genlocalfiles
	@echo "+ $@"
	@test -z "$$($(GO) fmt)" || \
		(echo "+ please format Go code with 'gofmt -s'" && /bin/false)

.PHONY: vet
vet:
	@echo "+ $@"
	go vet ./...

.PHONY: check
check: lint
	# check if Go modules are in sync
	$(GO) mod tidy
	@test -z "$$(git diff)" || \
		(echo -e "Repository is altered after build:\n$$(git diff --text)" && /bin/false)
	nbnotcomment=$$(grep '"linter":"golint"' lint.json | wc -l); \
	if [ $$nbnotcomment -gt 0 ]; then \
		cat lint.json; \
		echo "===> You should comment you code <==="; \
		exit 1; \
	fi
	unwanteddeps=$$(git grep --text github.com/skydive-project/skydive graffiti | grep -v github.com/skydive-project/skydive/graffiti | grep -v graffiti/gendecoder/main.go); \
	test -z "$$unwanteddeps" || \
		(echo -e "Graffiti should not import Skydive only packages:\n$$unwanteddeps" && /bin/false)

.PHONY: golangci-lint
golangci-lint:
	@echo "+ $@"
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run ${GOMETALINTER_FLAGS} -e '.*\.pb.go' -e '.*\._easyjson.go' -e '.*\._gendecoder.go' -e 'statics/bindata.go' --skip-dirs=statics --deadline 10m --out-format=json ./... | tee lint.json || true

.PHONY: lint
lint:
	make golangci-lint GOMETALINTER_FLAGS="--disable-all --enable=golint"
