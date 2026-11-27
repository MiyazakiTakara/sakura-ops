# Licence exception evidence

Evidence for every entry in `exceptions` of `supply-chain/license-policy.json`. The owner approved
these exceptions on 2026-11-27, on the condition that the original licence files, their hashes and
this verification evidence are recorded. Approval covers exactly the listed purl, licence string
and file hashes. Any change to a version, file or hash needs a new review and a new entry here.
The general licence policy (allowlist, deny list, rules) is unchanged.

## Method

syft reports a file it cannot classify as a licence entry `sha256:<hex>`, where `<hex>` is the
SHA-256 of that file's content. For each exception the coordinator:

1. Took the module from the Go module cache, after `go mod download <module>@<version>`.
   `go mod verify` reported `all modules verified`, so the cache content matches the `h1:` hashes
   in `go.sum`, which come from the Go checksum database.
2. Computed `sha256sum` of the module `LICENSE` and of every file whose hash appears in the
   exception's licence string, and confirmed that each pinned hash is reproduced exactly.
3. Read the files. The licence identified from them is listed below.

Reproduce with Go 1.27.4 (from the repository root):

```sh
go mod download <module>@<version>
cd "$(go env GOMODCACHE)/<module>@<version>" && sha256sum LICENSE <file>...
```

Recorded on 2026-11-27 by the coordinator (`go mod verify`: all modules verified).

## Entries

### github.com/getkin/kin-openapi v0.153.0

- purl: `pkg:golang/github.com/getkin/kin-openapi@v0.153.0`
- go.sum: `h1:Qo4qWW+Pj8o90LUTE6MDyG5TJAtWnfvrcWOBcWg7eBw=`

| File | SHA-256 | Content |
| --- | --- | --- |
| `LICENSE` | `612a11e78c07e12765d9cf3866e3edc5f8212c541602721b14e465311afb3aa6` | MIT License |
| `openapi3/license.go` | `f03c4ca07f75350cd046bb70344cf31e9ebff3e22bda9be4be92fcd6491715f8` | Go source: `type License struct` (OpenAPI License object), not a licence text |

Effective licence: MIT. Used by the OpenAPI contract test only.

### go.yaml.in/yaml/v4 v4.0.0

- purl: `pkg:golang/go.yaml.in/yaml/v4@v4.0.0`
- go.sum: `h1:/LG1pNO84Z4l603j/xMn69CcS5OYhMQ/f7l2LoQUGuw=`

| File | SHA-256 | Content |
| --- | --- | --- |
| `LICENSE` | `4a4e095045f46875715234a884741acd2b61b88959d6f47a243ae03ef95c4e76` | Apache License 2.0 |
| `NOTICE` | `257bb2d062ee0b9226e3b5f9c3c5d6aa3d38a450df27a9e060f061d4d4a4eb6f` | Files under `internal/libyaml/` ported from libyaml stay under their original MIT licence |

Effective licence: Apache-2.0 AND MIT, both allowlisted. Indirect dependency of kin-openapi.
