# CI (NEXT-005)

Every workflow in `.github/workflows/` runs on `pull_request` targeting
`v2/native-agent-blueprint` only: no push, schedule, manual trigger or `pull_request_target`, no
secrets, no deployment, top-level `permissions: contents: read`, every action pinned to a 40-hex
commit SHA, `persist-credentials: false` on every checkout. `make workflow-policy`
(`internal/workflowpolicy`, with negative tests) enforces the trigger type (`pull_request` only),
read-only permissions, SHA pins and `persist-credentials: false`; it does not check the target
branch filter or the absence of `secrets.*` references, which remain review items.
`make workflow-lint` runs it together with actionlint, so a violating workflow fails the
`Workflow lint and policy` job.

## Workflows and jobs

| Workflow | Job name (check name) | Covers | Run locally |
| --- | --- | --- | --- |
| CI (`ci.yml`) | `Go` | tool versions, gofmt, vet, staticcheck, `go test -race`, protobuf drift, govulncheck | `make tools fmt-check vet staticcheck test proto-check vuln` |
| CI | `Web and embedded build` | svelte-check, prettier, Vitest, SPA build embedded in the server, smoke test of the binary | `make web-install web-check build` |
| CI | `Secret scan` | pinned gitleaks over the full history, redacted | `make secrets` |
| CI | `Fuzz (short)` | every `Fuzz*` target under `cmd/` and `internal/`, 10 s each; uploads `testdata/fuzz` crashers as artifact `fuzz-crashers` on failure | `make fuzz-short` |
| CI | `Workflow lint and policy` | actionlint plus the workflow policy test | `make workflow-lint` |
| Contracts (`contracts.yml`) | `Generated contracts, lint, breaking, contract tests` | OpenAPI/proto generation drift, buf lint, `proto-breaking`, `openapi-breaking`, Go and Vitest contract tests, the OpenAPI document validation, breaking-script tests | `make contracts-check contracts-test proto-breaking-test openapi-breaking-test` |
| E2E (`e2e.yml`) | `Playwright (Chromium)` | browser tests against the embedded server | `make e2e-install e2e` |
| Supply chain (`supply-chain.yml`) | `SBOM, licence policy, vulnerability audit` | CycloneDX SBOMs, licence policy, govulncheck, npm audit | `make supply-chain` |
| Compose (`compose.yml`) | `Compose clean start` | clean start of the development stack (bootstrap, migrator, hub) and the smoke test: hardening, secret isolation, DB permissions (docker) | `make dev-up dev-smoke dev-down` |
| Compose | `Compose DB scenarios` | restart on an existing volume, database down, schema behind, migration failure (docker) | `make dev-scenarios` |
| Database (`db.yml`) | `Migrations and store` | sqlc drift, unit tests, integration tests against a throwaway PostgreSQL (docker) | `make sqlc-check test-integration` |

`make check` bundles the Go, web, secret and workflow checks (needs network for `vuln`); the other
jobs are run with the targets above.

## Required checks for branch protection

The owner should mark these exact job names as required on `v2/native-agent-blueprint`
(repository settings are not changed by this repository):

1. `Go`
2. `Web and embedded build`
3. `Secret scan`
4. `Fuzz (short)`
5. `Workflow lint and policy`
6. `Generated contracts, lint, breaking, contract tests`
7. `Playwright (Chromium)`
8. `SBOM, licence policy, vulnerability audit`
9. `Compose clean start`
10. `Migrations and store`

## Notes

- Fuzzing is bounded (10 s per target) and discovered automatically by `scripts/fuzz-short.sh`, so
  a new `func FuzzXxx(f *testing.F)` under `cmd/` or `internal/` is picked up without editing a list.
  This is a smoke level, not a replacement for longer fuzz campaigns.
- actionlint is pinned in its own module, `tools/actionlint/go.mod`, because it needs
  `go.yaml.in/yaml/v4` rc.3 while oasdiff, kin-openapi and oapi-codegen (in `tools/go.mod`) need the
  final v4.0.0 whose API differs. Merging them into one module does not build. It runs without
  shellcheck locally; GitHub runners ship shellcheck, so `run:` scripts are linted there.
- `openapi-breaking` and `proto-breaking` skip only when the verified base commit does not yet contain
  the contract; they print the base SHA when they do. The skip disappears once the contract is on the base.

## Not covered yet

- Container image vulnerability scanning: separate follow-up.
- dependency-review: the repository dependency graph is disabled; tracked in #24.
- Release signing and provenance: #30 and #36. There is no release workflow.
