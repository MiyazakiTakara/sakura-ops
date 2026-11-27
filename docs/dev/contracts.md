# Contracts (NEXT-006)

Two contract families, both generated code is committed and checked for drift.

## What is normative

| Contract | Source of truth | Generated (committed, never edited by hand) |
| --- | --- | --- |
| REST, v1, evolving | `api/openapi/sakuraops.v1.yaml` (OpenAPI 3.1) | `internal/api/apitypes/types.gen.go`, `frontend/src/lib/api/schema.d.ts` |
| Agent protocol v1 draft | `proto/sakuraops/agent/v1/agent.proto` | `internal/agentproto/agentv1/*.pb.go` |

The OpenAPI document lists ONLY behaviour the server implements and tests today (`GET /healthz`,
the reserved `/api` namespace, the error envelope). `docs/23-api-contract.md` stays the planning
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
- Add new fields in the gaps already left (each message reserves a block for future fields).
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
make contracts-check    # contracts + proto-breaking, then fail if git sees any diff in generated paths
make contracts-test     # Go and Vitest contract tests (fixtures validated on both sides)
make proto-breaking     # buf breaking (FILE) against origin/v2/native-agent-blueprint
```

`contracts-check` also runs `buf lint` (STANDARD, via `make agent-proto`). Two rules are
disabled in `proto/buf.yaml` on purpose: `SERVICE_SUFFIX` (the service is called `AgentHub`
in docs/13) and `RPC_REQUEST/RESPONSE_STANDARD_NAME` (the stream carries the `AgentMessage` /
`HubMessage` envelopes).

Drift: the generators are pinned (oapi-codegen and buf plugins in `tools/go.mod`,
`openapi-typescript` exact-pinned in `frontend/package-lock.json`). CI regenerates and fails
when the committed output differs, so a YAML or `.proto` edit without regeneration cannot merge.

Breaking: `proto-breaking` runs `buf breaking` with rule set FILE against the `proto/` module on
`PROTO_BASE` (default `origin/v2/native-agent-blueprint`; override with `make PROTO_BASE=<ref>`).
While the base ref has no `proto/buf.yaml` (the first PR that adds it) the target prints a note
and passes, because a new contract has nothing to break. After the first merge it compares for real.
There is no breaking check for OpenAPI yet.

## Cross-language fixtures

`api/fixtures/*.json` are validated by `internal/api/contract_test.go` (Go, `kin-openapi`
against the OpenAPI schema) and by `frontend/src/lib/api/errors.test.ts` (Vitest, hand-written
runtime guards in `errors.ts`). The file name states the expected verdict, so both sides must
agree. `kin-openapi` (MIT) is a test-only dependency; `oapi-codegen` (Apache-2.0) and
`openapi-typescript` (MIT) are build tools only.
