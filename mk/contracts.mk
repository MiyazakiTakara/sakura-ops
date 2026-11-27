# Contract generation and drift checks (NEXT-006). Included from the root Makefile.
OPENAPI_SPEC  := api/openapi/sakuraops.v1.yaml
OPENAPI_CFG   := api/openapi/oapi-codegen.yaml
OPENAPI_GO    := internal/api/apitypes
OPENAPI_TS    := frontend/src/lib/api/schema.d.ts
AGENT_PROTO   := proto
AGENT_GO      := internal/agentproto/agentv1
PROTO_BASE    ?= origin/v2/native-agent-blueprint
CONTRACT_PATHS := $(OPENAPI_GO) $(OPENAPI_TS) $(AGENT_GO)

.PHONY: contracts openapi-go openapi-ts agent-proto proto-lint proto-breaking \
	proto-breaking-test contracts-check contracts-test

## openapi-go: regenerate the Go types from the OpenAPI document.
openapi-go:
	$(TOOL) oapi-codegen -config $(OPENAPI_CFG) $(OPENAPI_SPEC)

## openapi-ts: regenerate the TypeScript schema (needs `make web-install`).
openapi-ts:
	cd frontend && npx --no-install openapi-typescript ../$(OPENAPI_SPEC) -o ../$(OPENAPI_TS)

## agent-proto: lint and regenerate the agent protocol Go code.
agent-proto:
	cd $(AGENT_PROTO) && $(TOOL) buf lint && $(TOOL) buf generate

## contracts: regenerate every generated contract artifact.
contracts: openapi-go openapi-ts agent-proto

## proto-breaking: buf breaking (FILE) against PROTO_BASE. Fails closed when the ref does not
## resolve or git errors; skips only when the verified base commit has no proto/ module.
proto-breaking:
	@BUF='$(TOOL) buf' sh scripts/proto-breaking.sh "$(PROTO_BASE)" "$(CURDIR)"

## proto-breaking-test: tests for scripts/proto-breaking.sh in temporary git repos.
proto-breaking-test:
	@BUF='$(TOOL) buf' sh scripts/proto-breaking_test.sh

## contracts-test: contract tests on both sides (Go and Vitest).
contracts-test:
	$(GO) test -race -count=1 ./internal/api/... ./internal/agentproto/...
	cd frontend && npx --no-install vitest run src/lib/api

## contracts-check: regenerate, fail on drift, lint and breaking checks.
contracts-check: contracts proto-breaking
	@if [ -n "$$(git status --porcelain -- $(CONTRACT_PATHS))" ]; then \
		echo "generated contracts are out of date; run 'make contracts' and commit"; \
		git --no-pager status --short -- $(CONTRACT_PATHS); exit 1; fi
