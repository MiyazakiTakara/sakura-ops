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

### github.com/jackc/pgx/v5 v5.13.0

- purl: `pkg:golang/github.com/jackc/pgx/v5@v5.13.0`
- go.sum: `h1:Hl08k64maU0nrRyVUEbwCnK6/gT0EhMrtAqBHz9/gA4=`

| File | SHA-256 | Content |
| --- | --- | --- |
| `LICENSE` | `467f95e074fe23079a5623ed652619682692041b8551da27e3c2ddb9659a1507` | MIT License |
| `pgproto3/notice_response.go` | `4b9071daf5f9975eb936c370200709bb7433862c5d4a32d21db65620133f4a8e` | Go source: `type NoticeResponse ErrorResponse` (PostgreSQL protocol message), not a licence text |

Effective licence: MIT. Linked into the server binary (database store).

### modernc.org/libc v1.80.2

- purl: `pkg:golang/modernc.org/libc@v1.80.2`
- go.sum: `h1:3+a4fJZvvzg920ZC0mOepnKPGn7cqCaVeDxm/UnrM1A=`

| File | SHA-256 | Content |
| --- | --- | --- |
| `LICENSE` | `95ff867eb55a56935fa7492406cfa953fb7c13ca73f4c0a86ae05756b4605600` | BSD-3-Clause |
| `LICENSE-3RD-PARTY.md` | `f597097efe3d97021f89170746bd3a0fb9a8b6fb26b82043ed68a4e0283bee6c` | Third-party notices: Go (BSD-3-Clause), musl libc (MIT, with BSD-2-Clause and public-domain portions), go-netdb and nixpkgs (MIT) |

Effective licence: BSD-3-Clause with MIT and BSD-2-Clause notices, all allowlisted. Module graph
only (goose's own tests); `go list -deps -test ./...` lists no modernc.org package.

### modernc.org/memory v1.12.1

- purl: `pkg:golang/modernc.org/memory@v1.12.1`
- go.sum: `h1:nFMiWrpStgZczNl6XI9GnIk/rWhYIyHGUaR04pGbp9g=`

| File | SHA-256 | Content |
| --- | --- | --- |
| `LICENSE` | `59895e669f48f168b6b858358f6005779cdf40a265f7828813061b56af67b496` | BSD-3-Clause |
| `LICENSE-LOGO` | `5ae5bee3072a841376451b48d8cfcec7188e10543926d5870828d36c8a750dc5` | One line: a Wikimedia Commons URL crediting the project logo (not part of the Go code) |

Effective licence: BSD-3-Clause. Module graph only (goose's own tests).

### modernc.org/sqlite v1.62.0

- purl: `pkg:golang/modernc.org/sqlite@v1.62.0`
- go.sum: `h1:gMaCjSM1qpl+iOjiWhR1LYP3zqrYDI+5GYLGt6veP2g=`

| File | SHA-256 | Content |
| --- | --- | --- |
| `LICENSE` | `c6fe05491a60ae13bcd223088d2705e36dede24e5587226231d2459ada5c4822` | BSD-3-Clause |
| `LICENSE-SQLITE` | `8438c9c89b849131ead81d5435cb97fcf052df5b0b286dda8a2d4c29e6cb3fd0` | "SQLite Is Public Domain" dedication |
| `LICENSE-3RD-PARTY.md` | `f4a00ade8f4057ab0ea49097d9c459df8559fcf10ca78da0c950343fb7428d37` | Third-party table: MIT, BSD-3-Clause, Apache-2.0, public domain |

Effective licence: BSD-3-Clause AND MIT plus public-domain SQLite, all acceptable under the
allowlist. Module graph only (goose's own tests).
