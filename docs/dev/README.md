# Developer setup (P01 skeleton)

Status: monorepo skeleton and technical spikes for NEXT-002/NEXT-003. Nothing here
is a product feature. Linux only (developed on Ubuntu under WSL2).

## Prerequisites

| Tool | Version used | Notes |
| --- | --- | --- |
| Go | 1.27.4 | `go.mod` carries the version; install from <https://go.dev/dl/> and verify the SHA-256 published in <https://go.dev/dl/?mode=json> |
| Node.js | 22.x (>= 22.17.0) | `frontend/package.json` `engines`; npm 9+ |
| make, bash, curl, gcc | any recent | gcc is only needed by the race detector |

No Docker, PostgreSQL or Redis is required at this stage.

## Dev tools

Go dev tools are pinned in the separate module `tools/go.mod` (`tool` directives) and
run as `go tool -modfile=tools/go.mod <tool>`: buf, protoc-gen-go, protoc-gen-go-grpc,
goose, sqlc, staticcheck, govulncheck, oasdiff. actionlint lives in its own module
`tools/actionlint/go.mod` (see `docs/dev/ci.md`). `make tools` prints their versions. goose and sqlc
are pinned and version-checked only; there are no migrations or queries yet.

gitleaks is installed by `scripts/install-gitleaks.sh` into `bin/tools/` from a pinned
release after verifying a pinned SHA-256 (`make secrets` calls it).

## Everyday commands

```
make check        # fmt-check vet staticcheck test proto-check workflow-lint web-install web-check build vuln secrets
make build        # builds the SPA, embeds it, writes bin/sakuraops-server (stripped)
make test         # go test -race ./...
make fuzz-short   # every Fuzz target under cmd/ and internal/, 10 s each
make web-check    # svelte-check (strict) + prettier --check + vitest
```

`make check` includes `vuln`, which downloads the Go vulnerability database, so it needs
network access. Each target can also be run on its own.

## Run the server

```
make build
bin/sakuraops-server -listen 127.0.0.1:8080
curl -s http://127.0.0.1:8080/healthz        # {"status":"ok"}
```

Open `http://127.0.0.1:8080/` for the placeholder page and
`http://127.0.0.1:8080/spike/terminal` for the technical spike (floating local-echo
terminal plus a CodeMirror YAML editor). The spike is not linked from any navigation, never
calls a backend and executes nothing.

Without `make web-build`, only `internal/webui/dist/.gitkeep` exists: the Go packages still
compile and `go test ./...` passes, and the server answers 503 `frontend_not_built` for
application routes while `/healthz` keeps working. Built assets are git-ignored.

## HTTP behaviour worth knowing

- `/api`, `/api/...`, `/healthz` and `/healthz/...` never fall back to `index.html`; unknown
  API paths return the JSON error envelope `{code,message,request_id}`.
- Missing files with an extension and anything under `/_app/` return 404, never HTML.
- `/_app/immutable/*` is cached for a year; everything else is `no-cache`.
- The CSP has no `unsafe-inline` for scripts. SvelteKit's static fallback contains one inline
  bootstrap script whose body changes every build (it embeds a version timestamp), so the
  server computes the `sha256-` hash from the embedded `index.html` at startup. Styles keep
  `'unsafe-inline'` (Svelte and xterm.js emit style attributes); that exception is for styles only.

## Protobuf generation

The spike schema lives at `internal/spike/mtls/proto/sakuraops/spike/v1/spike.proto` and is
NOT the agent protocol (NEXT-006 decides that). `make proto` runs `buf lint` and `buf generate`
(plugins resolved through `tools/go.mod`) and writes `internal/spike/mtls/spikepb/`, which is
committed. `make proto-check` regenerates and fails when `git status` shows any change there.

## mTLS spike

`internal/spike/mtls` is non-normative: TLS 1.3 minimum, mandatory verified client
certificates with an explicit clientAuth EKU, ALPN `h2`, 256 KiB message limit, and an
in-memory revocation set checked at handshake; revoking also ends open streams and closes the
transport connection. All certificates are
generated at test time (ECDSA P-256) in memory or `t.TempDir()`; no key material is
committed. `TestClientInSeparateProcess` re-executes the test binary (no shell) as the client.
Known limits are listed in `docs/dev/spike-results.md` ("Limitations assigned to NEXT-017").

## More developer documents

[`ci.md`](ci.md) (workflows, required checks), [`contracts.md`](contracts.md),
[`database.md`](database.md), [`compose.md`](compose.md), [`e2e.md`](e2e.md),
[`supply-chain.md`](supply-chain.md), [`spike-results.md`](spike-results.md).

## Not here yet (P02 and later)

Docker Compose, PostgreSQL, Goose migrations and sqlc queries, OpenAPI and generated
clients, Playwright, SBOM and license scanning in CI, chi router, auth,
real agent protocol, any task execution.
