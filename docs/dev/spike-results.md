# P01 spike results (NEXT-002 / NEXT-003 evidence)

Raw measurements from one machine on 2026-11-27. They are not benchmarks of the product,
not a comparison with any other stack, and no thresholds were derived from them
(docs/17-testing.md). Code measured: commit `b5d38fdeae8431c6e4c1191da4d896333344549b`
(test results below were rerun at `a9e702c`, which adds a goroutine-leak fix and an SPA routing fix; later commits only add documentation).

## Environment

| Item | Value |
| --- | --- |
| CPU | Intel(R) Core(TM) Ultra 7 155H (22 logical CPUs, per `nproc`) |
| RAM | 16 022 940 kB (`/proc/meminfo` MemTotal) |
| OS / kernel | Ubuntu 26.04.1 LTS under WSL2, kernel 6.18.58.1-microsoft-standard-WSL2, x86_64 |
| Go | go1.27.4 linux/amd64 (checksum verified against go.dev JSON) |
| Node / npm | v22.22.1 / 9.2.0 |
| Load | developer laptop, other Windows/WSL workloads running; no isolation |

## Artifact sizes

Both binaries embed the real SPA build (see below). Built with `CGO_ENABLED=0 go build -trimpath`.

| Artifact | Bytes |
| --- | --- |
| `bin/sakuraops-server`, `-ldflags='-s -w'` (what `make build` produces) | 7 958 688 |
| same, not stripped | 11 287 567 |
| `frontend/build` total (18 files) | 871 065 |

Gzip (`gzip -9`) of the frontend files that matter:

| File | Raw bytes | Gzip bytes |
| --- | --- | --- |
| `nodes/3.D20De9GK.js` (spike route: xterm.js + CodeMirror + app code) | 767 362 | 221 979 |
| `chunks/l_hLmMfT.js` + `chunks/DNuZpNz0.js` (shared Svelte/Kit runtime) | 47 908 + 37 390 | 18 289 + 13 762 |
| all `.js` and `.css` files (sum of per-file gzip) | 869 707 | 261 710 |

The spike route's 767 kB chunk triggers Vite's 500 kB chunk warning. It is only loaded by
`/spike/terminal`; the root route's own node chunk is 327 bytes. Splitting xterm.js and
CodeMirror per route is a later concern.

## Server resource use

Method: `bin/sakuraops-server -listen 127.0.0.1:18082`, read `VmRSS` and utime+stime from
`/proc/<pid>` after 1 s idle, then after 10 000 sequential `GET /healthz` requests issued by a
single `curl "http://127.0.0.1:18082/healthz?[1-10000]"` (keep-alive, one connection, 1.18 s wall).

| State | RSS | CPU time (utime+stime) |
| --- | --- | --- |
| idle, 1 s after start | 6 892 kB | 0 ticks (CLK_TCK 100) |
| after 10 000 healthz requests | 13 816 kB | 83 ticks (0.83 s) |
| then 200 fetches of `/spike/terminal` (index.html) | 13 600 kB | 86 ticks |

Single run, no repetition, no concurrency; treat as order of magnitude only. Per-request
latency, p99, concurrent load: NOT RUN.

## mTLS and gRPC latency

`go test -run '^$' -bench . -benchtime 3s -count 3 -benchmem ./internal/spike/mtls`,
client and server in the same process over loopback, ECDSA P-256 certificates, TLS 1.3.

| Benchmark | ns/op (3 runs) | Allocs/op |
| --- | --- | --- |
| `BenchmarkMutualTLSHandshake` (TCP connect + full mutual handshake) | 749 965 / 754 010 / 750 458 | about 1 132 |
| `BenchmarkUnaryEchoOverMutualTLS` (16-byte unary on an open connection) | 108 223 / 107 747 / 110 326 | 170 |

So about 0.75 ms per full handshake and about 0.11 ms per small unary call on this machine.
Cross-host latency, handshake through a reverse proxy, and many concurrent agents: NOT RUN
(no lab network or proxy in this session).

## Test results (commit a9e702c)

| Command | Result |
| --- | --- |
| `go test -race -count=1 -v ./...` | 50 PASS lines, 0 FAIL, 0 SKIP: 24 top-level tests and 26 subtests (httpserver 13+16, spike/mtls 11+10) |
| `make fuzz-short` | `FuzzCleanAssetPath` 574 411 execs and `FuzzSPAHandlerNeverLeaks` 581 076 execs in 10 s each, no failures |
| `make web-check` | svelte-check: 0 errors, 0 warnings (224 files); `prettier --check` clean; vitest: 1 file, 18 tests passed |
| `make staticcheck`, `make vet`, `make fmt-check` | clean |
| `make proto-check` | clean; verified to fail when the schema changes without regeneration |
| `make vuln` | No vulnerabilities found (after bumping `golang.org/x/net` to v0.63.0; before the bump two unreachable advisories, GO-2026-6866 and GO-2026-6860, were reported in x/net v0.60.0) |
| `make secrets` | gitleaks 8.30.1, no leaks found over the branch history |
| `npm audit` | 0 vulnerabilities |

`TestHelperClientProcess` shows as PASS in a normal run because it returns immediately unless
re-executed by `TestClientInSeparateProcess`; the real client work is covered by the latter
(it asserts the child PID differs from the server PID and that the server saw the child's
certificate identity).

Required mTLS cases, each a real passing test: two valid clients with distinct verified
identities; foreign-CA client cert; no client cert; client cert with serverAuth-only EKU;
client cert with no EKU; expired and not-yet-valid client certs; server cert for the wrong DNS
name; client not trusting the server CA; client capped at TLS 1.2; revoked serial rejected at
handshake; revocation while a stream is open closes only that client's stream (and the revoked
client's new calls on the old connection and new connections fail); deadline exceeded;
message of 256 KiB + 1 byte rejected with ResourceExhausted; TLS 1.3 and ALPN h2 observed
server-side; reconnect after server restart within 4 s with a 50-400 ms backoff; client in a
separate OS process.

## Browser check

Headless Chrome (Windows build, started from WSL) loaded `/spike/terminal` from the embedded
server and the dumped DOM contains the page heading, the xterm.js screen with the `skr > `
prompt line and the CodeMirror editor, so the SPA bootstrap runs under the CSP (inline script
allowed by hash only). That is all that was verified in a browser.

NOT RUN: interactive behaviour in a real browser (dragging, resizing, keyboard move/resize,
typing, history and Tab in xterm.js, Ctrl+L), console-error inspection, 360/768/1280/1920 px
and 200% zoom checks, keyboard-only and contrast review. The line-editing logic is unit tested
(Vitest) but the window and xterm wiring has no automated browser test until Playwright (P02).
Known spike limits: a line longer than the terminal width wraps and the single-line redraw
garbles it; wide (CJK) characters shift the cursor.

## Direct dependencies

### Go (main module)

| Module | Version | License |
| --- | --- | --- |
| google.golang.org/grpc | v1.85.0 | Apache-2.0 |
| google.golang.org/protobuf | v1.36.12 | BSD-3-Clause |

Indirect, kept at fixed versions by `go.mod`: golang.org/x/net v0.63.0, x/sys v0.50.0,
x/text v0.44.0, google.golang.org/genproto/googleapis/rpc.

### Go dev tools (`tools/go.mod`, not shipped)

| Tool module | Version | License |
| --- | --- | --- |
| github.com/bufbuild/buf | v1.74.0 | Apache-2.0 |
| google.golang.org/protobuf (protoc-gen-go) | v1.36.12 | BSD-3-Clause |
| google.golang.org/grpc/cmd/protoc-gen-go-grpc | v1.6.2 | Apache-2.0 |
| github.com/pressly/goose/v3 | v3.30.2 | MIT |
| github.com/sqlc-dev/sqlc | v1.31.1 | MIT |
| honnef.co/go/tools (staticcheck) | v0.8.2 (2026.2.2) | MIT |
| golang.org/x/vuln (govulncheck) | v1.10.0 | BSD-3-Clause |

sqlc builds without cgo (it uses a WASM-based parser), so no workaround was needed.
Licenses were read from the `LICENSE` files in the module cache.

### npm (`frontend/`, lockfile committed)

Runtime (bundled into the SPA):

| Package | Version | License |
| --- | --- | --- |
| @xterm/xterm | 6.0.0 | MIT |
| @xterm/addon-fit | 0.11.0 | MIT |
| codemirror | 6.0.2 | MIT |
| @codemirror/lang-yaml | 6.1.3 | MIT |
| @codemirror/state | 6.7.6 | MIT |
| @codemirror/view | 6.44.2 | MIT |

Build and test only:

| Package | Version | License |
| --- | --- | --- |
| svelte | 5.59.2 | MIT |
| @sveltejs/kit | 3.4.1 | MIT |
| @sveltejs/adapter-static | 4.0.1 | MIT |
| @sveltejs/vite-plugin-svelte | 7.3.2 (required peer of kit) | MIT |
| vite | 8.4.0 | MIT |
| vitest | 5.1.2 | MIT |
| svelte-check | 4.7.8 | MIT |
| typescript | 6.0.3 (kit 3.4.1 peers `^6`, so 7.x was not used) | Apache-2.0 |
| prettier | 3.10.1 | MIT |
| prettier-plugin-svelte | 4.2.0 | MIT |

License values come from each package's `package.json` `license` field. Transitive licenses
were not scanned: NOT RUN (the license scan is a pre-distribution item for P02).

## Findings relevant to the NEXT-002 decision

- SvelteKit 3 no longer reads `svelte.config.js` and removed `$lib`; configuration lives in
  `vite.config.ts` and tsconfig must extend `$app/tsconfig`. The ecosystem moves quickly, pin
  versions and keep `package-lock.json`.
- xterm.js and CodeMirror integrate with Svelte 5 through plain `onMount` wrappers without
  blockers; the SPA fallback needs one CSP script hash that must be derived per build.
- Revocation state is in memory; a persistent revocation list and a CRL/OCSP-like distribution
  decision belong to NEXT-006 and the CA design.
- Revocation in the spike ends the affected streams and rejects new RPCs, but does not close
  the underlying TCP/TLS connection. NEXT-017 must close the transport on revoke
  (docs/03-architecture.md requires that revocation closes the existing connection).
- The server's `WriteTimeout` of 30 s will have to be revisited (per-handler
  `http.ResponseController` deadlines or none) for SSE endpoints in P02/P05.
- SPA fallback treats a missing path as a 404 only under `/_app/` or with a static-asset
  extension (js, mjs, css, map, json, txt, ico, png, svg, webp, jpg, woff, woff2,
  webmanifest, wasm, html); dotted application routes such as `/tasks/linux.facts` fall back
  to `index.html`.
- HTTP/2 and ALPN were verified on a direct connection only; the target reverse-proxy
  topology (docs/10-stack.md gate 3) is NOT RUN.
