# Contracts (NEXT-006)

Two contract families, both generated code is committed and checked for drift.

## What is normative

| Contract | Source of truth | Generated (committed, never edited by hand) |
| --- | --- | --- |
| REST, v1, evolving | `api/openapi/sakuraops.v1.yaml` (OpenAPI 3.1) | `internal/api/apitypes/types.gen.go`, `frontend/src/lib/api/schema.d.ts` |
| Agent protocol v1 draft | `proto/sakuraops/agent/v1/agent.proto` | `internal/agentproto/agentv1/*.pb.go` |

The OpenAPI document lists ONLY behaviour the server implements and tests today (`GET /healthz`,
`GET /readyz`, the reserved `/api` namespace, the error envelope). `docs/23-api-contract.md` stays the planning
scope; an endpoint enters the OpenAPI file in the same change that implements it, never earlier.

The `.proto` file is normative for the wire format from now on; `docs/13-agent-protocol.md`
stays normative for semantics. Limits (256 KiB frame, bounded `active_command_ids`, bounded
lists and strings) are enforced by the implementation, not by the schema. Signed bytes are
canonical JSON carried in `bytes` fields; protobuf serialization is never signed. Typed
command results are an opaque canonical-JSON `bytes` plus a `result_schema` reference until
NEXT-016/NEXT-022 define the schemas; there is no `map<string,string>`.

The spike schema in `internal/spike/mtls/proto` is a separate buf module and is not part of this contract.

## Changing a contract

Protobuf:

- Field numbers and their types are immutable. Never reuse a number.
- Removing a field: delete it and add `reserved <number>;` and `reserved "<name>";`.
  `reserved` is used ONLY for removed fields; nothing is reserved today. Numbers 2-9 in the
  envelopes are left free for future envelope-level fields (body cases start at 10), and other
  messages leave gaps after their last field. This is a convention, not enforced by the schema.
- Add new fields in those gaps.
- New `oneof` cases and enum values are additive within a major version. Receivers treat an
  unknown or unset case as a protocol error (fail closed).
- Any change of meaning, or anything that old receivers cannot ignore, is a new version
  (`sakuraops.agent.v2`, or a major bump negotiated in Hello/ServerHello) and needs an ADR.

OpenAPI:

- Do not change the type of an existing field or loosen validation silently; additive optional
  fields only. A semantic change is a new API version (`/api/v2`).
- Edit the YAML, run `make contracts`, update fixtures in `api/fixtures/` (name pattern
  `<schema>.<valid|invalid>.<case>.json`), commit YAML and generated files together.

## Commands

```
make contracts          # regenerate OpenAPI Go + TS types and the agent proto Go code
make contracts-check    # contracts + proto-breaking + openapi-breaking, then fail if git sees any diff in generated paths
make contracts-test     # Go and Vitest contract tests (fixtures validated on both sides)
make proto-breaking     # buf breaking (FILE) against origin/v2/native-agent-blueprint
make proto-breaking-test # scripts/proto-breaking_test.sh: fail-closed and post-bootstrap cases
make openapi-breaking   # oasdiff breaking changes (ERR and WARN fail) against the same base
make openapi-breaking-test # scripts/openapi-breaking_test.sh: same cases for the OpenAPI document
```

`contracts-check` also runs `buf lint` (STANDARD, via `make agent-proto`). Two rules are
disabled in `proto/buf.yaml` on purpose: `SERVICE_SUFFIX` (the service is called `AgentHub`
in docs/13) and `RPC_REQUEST/RESPONSE_STANDARD_NAME` (the stream carries the `AgentMessage` /
`HubMessage` envelopes).

Drift: the generators are pinned (oapi-codegen and buf plugins in `tools/go.mod`,
`openapi-typescript` exact-pinned in `frontend/package-lock.json`). CI regenerates and fails
when the committed output differs, so a YAML or `.proto` edit without regeneration cannot merge.

Breaking: `proto-breaking` (`scripts/proto-breaking.sh`) runs `buf breaking` with rule set FILE
against the `proto/` module on `PROTO_BASE` (default `origin/v2/native-agent-blueprint`; override
with `make PROTO_BASE=<ref>`). It fails closed:

1. `PROTO_BASE` is resolved with `git rev-parse --verify --quiet <ref>^{commit}`. If it does not
   resolve (missing ref, shallow clone gap, git error) the target prints an error naming the ref
   and exits non-zero. It never skips here.
2. On the resolved commit SHA, `git ls-tree --name-only <sha> -- proto/buf.yaml` decides whether
   the module exists. A git error fails the target.
3. The single skip condition: a verified commit that genuinely lacks `proto/buf.yaml` (the first
   PR that adds the module). The message prints the resolved SHA. Nothing to break in a new contract.
4. Otherwise `buf breaking` runs against that SHA (not the symbolic ref).

CI checks out with `fetch-depth: 0` so the base ref resolves; no step relies on a skip.
`make proto-breaking-test` builds temporary git repos (no network) and covers: unresolvable base,
base without the module (skip + SHA), post-bootstrap compatible change (passes), post-bootstrap
breaking change (buf fails), and non-commit object / non-git directory / missing tree object
(fail). It uses the pinned buf from `tools/go.mod` and runs in the contracts workflow.

`make openapi-breaking` (`scripts/openapi-breaking.sh`, oasdiff pinned in `tools/go.mod`) has the same
fail-closed structure: an unresolvable base fails, a verified base without
`api/openapi/sakuraops.v1.yaml` skips and prints the SHA, a spec deleted from the working tree fails, and
otherwise `oasdiff breaking --fail-on WARN` compares the base copy with the working tree.
`make openapi-breaking-test` covers these cases in temporary repos. `internal/api/openapi_valid_test.go`
loads and validates the document with kin-openapi (no external refs) and has a negative case.

## What the schema does not enforce

The round-trip and unknown-`oneof` tests in `internal/agentproto` are wire-format evidence only.
Validation of messages that mix known and unknown fields, handling of unknown enum values,
version negotiation, signature verification and fencing are implementation gates, not enforced
by the schema. They are tracked in the hub and agent protocol issues.

## npm `overrides` for openapi-typescript

`openapi-typescript` 7.13.0 (latest) declares the peer `typescript ^5.x`, while the frontend uses
TypeScript 6, so a plain `npm install` fails with ERESOLVE. `frontend/package.json` therefore has
`"overrides": {"openapi-typescript": {"typescript": "$typescript"}}`, which makes it use the
project's TypeScript. This is acceptable because the generator is a build-time tool only (it
ships nothing to users) and its output, `schema.d.ts`, is type-checked by svelte-check in CI
(`make web-check`) and drift-checked by `make contracts-check`. Remove the override as soon as an
`openapi-typescript` release declares TypeScript 6 support, then re-run `npm install` and `make contracts`.

## Cross-language fixtures

`api/fixtures/*.json` are validated by `internal/api/contract_test.go` (Go, `kin-openapi`
against the OpenAPI schema) and by `frontend/src/lib/api/errors.test.ts` (Vitest, hand-written
runtime guards in `errors.ts`). The file name states the expected verdict, so both sides must
agree. `kin-openapi` (MIT) is a test-only dependency; `oapi-codegen` (Apache-2.0) and
`openapi-typescript` (MIT) are build tools only.
