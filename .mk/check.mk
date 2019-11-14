
.PHONY: fmt
fmt: genlocalfiles
	@echo "+ $@"
	@test -z "$$($(GO) fmt)" || \
		(echo "+ please format Go code with 'gofmt -s'" && /bin/false)

.PHONY: vet
vet:
	@echo "+ $@"
	test -z "$$($(GO) tool vet $$( \
			$(GO) list ./... \
			| perl -pe 's|$(SKYDIVE_GITHUB)/?||g' \
			| grep -v '^tests') 2>&1 \
		| tee /dev/stderr \
		| grep -v '^flow/probes/afpacket/' \
		| grep -v 'exit status 1' \
		)"

.PHONY: check
check: lint
	# check if Go modules are in sync
	$(GO) mod tidy
	@test -z "$$(git diff)" || \
		(echo -e "Repository is altered after build:\n$$(git diff)" && /bin/false)
	nbnotcomment=$$(grep '"linter":"golint"' lint.json | wc -l); \
	if [ $$nbnotcomment -gt 0 ]; then \
		cat lint.json; \
		echo "===> You should comment you code <==="; \
		exit 1; \
	fi

.PHONY: golangci-lint
golangci-lint:
	@echo "+ $@"
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run ${GOMETALINTER_FLAGS} -e '.*\.pb.go' -e '.*\._easyjson.go' -e '.*\._gendecoder.go' -e 'statics/bindata.go' --skip-dirs=statics --deadline 10m --out-format=json ./... | tee lint.json || true

.PHONY: lint
lint:
	make golangci-lint GOMETALINTER_FLAGS="--disable-all --enable=golint"
