# SakuraOps Next developer entry points. Every target here is real and used by CI.
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

GO        ?= go
TOOL      := $(GO) tool -modfile=$(CURDIR)/tools/go.mod
GOLANG_PKGS := ./...
GITLEAKS  := bin/tools/gitleaks
PROTO_DIR := internal/spike/mtls/proto
PROTO_OUT := internal/spike/mtls/spikepb
DIST      := internal/webui/dist

.PHONY: tools fmt-check vet staticcheck test fuzz-short vuln proto proto-check \
	web-install web-check web-build build secrets check

## tools: print the pinned versions of the Go toolchain and dev tools.
tools:
	@$(GO) version
	@echo "buf $$($(TOOL) buf --version)"
	@$(TOOL) protoc-gen-go --version
	@$(TOOL) protoc-gen-go-grpc --version
	@$(TOOL) goose --version
	@echo "sqlc $$($(TOOL) sqlc version)"
	@$(TOOL) staticcheck -version
	@$(TOOL) govulncheck -version | head -3
	@$(TOOL) oasdiff --version
	@echo "actionlint $$(cd tools/actionlint && $(GO) tool actionlint -version | head -1)"

fmt-check:
	@out="$$(gofmt -l cmd internal)"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet $(GOLANG_PKGS)

staticcheck:
	$(TOOL) staticcheck $(GOLANG_PKGS)

test:
	$(GO) test -race -count=1 $(GOLANG_PKGS)

## fuzz-short: every Fuzz target found under cmd/ and internal/, 10 s each (scripts/fuzz-short.sh).
fuzz-short:
	GO='$(GO)' ./scripts/fuzz-short.sh 10s

## vuln: needs network access to the Go vulnerability database.
vuln:
	$(TOOL) govulncheck $(GOLANG_PKGS)

proto:
	cd $(PROTO_DIR) && $(TOOL) buf lint && $(TOOL) buf generate

## proto-check: regenerate and fail if committed generated code drifts.
proto-check: proto
	@if [ -n "$$(git status --porcelain -- $(PROTO_OUT))" ]; then \
		echo "generated protobuf code is out of date; run 'make proto' and commit"; \
		git --no-pager diff --stat -- $(PROTO_OUT); exit 1; fi

web-install:
	cd frontend && npm ci

web-check:
	cd frontend && npm run check && npm run format:check && npm run test

## web-build: build the SPA and copy it into the Go embed directory.
web-build:
	cd frontend && npm run build
	find $(DIST) -mindepth 1 ! -name .gitkeep -delete
	cp -a frontend/build/. $(DIST)/

## build: real UI embedded, stripped static-ish server binary in bin/.
build: web-build
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o bin/sakuraops-server ./cmd/sakuraops-server

## secrets: scan the full git history with the pinned gitleaks (values redacted).
secrets:
	./scripts/install-gitleaks.sh
	$(GITLEAKS) git --config .gitleaks.toml --redact --no-banner --verbose .

## check: the main CI checks in one go (see docs/dev/ci.md for the rest). Includes `vuln`, which needs network.
check: fmt-check vet staticcheck test proto-check workflow-lint web-install web-check build vuln secrets

-include mk/*.mk
