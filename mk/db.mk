# Database targets. See docs/dev/database.md.
.PHONY: sqlc sqlc-check test-integration

## sqlc: regenerate internal/store/dbgen from db/migrations and db/queries.
sqlc:
	$(TOOL) sqlc generate

## sqlc-check: regenerate and fail when committed generated code drifts (tracked or untracked).
sqlc-check: sqlc
	@if [ -n "$$(git status --porcelain -- internal/store/dbgen)" ]; then \
		echo "generated sqlc code is out of date; run 'make sqlc' and commit"; \
		git --no-pager status --short -- internal/store/dbgen; exit 1; fi

## test-integration: migrations, roles and /readyz against a throwaway PostgreSQL container (needs docker).
test-integration:
	$(GO) test -tags integration -race -count=1 -timeout 15m -v ./test/integration/...
