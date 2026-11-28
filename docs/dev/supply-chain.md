# Supply chain checks (NEXT-005, issue #24)

What the repository verifies about its dependencies, how, and what it does not cover. Everything
here runs on pull requests only (`.github/workflows/supply-chain.yml`) and locally with
`make supply-chain`. No secrets, no publishing, no deployment.

## Commands

| Target | What it does |
| --- | --- |
| `make sbom` | Builds the server (`make build`), installs the pinned syft and writes SBOMs to `bin/sbom/` (gitignored). |
| `make license-check` | Runs `sbom`, then `cmd/licensecheck` against `supply-chain/license-policy.json`. |
| `make deps-audit` | `govulncheck` (`make vuln`) plus `npm audit` on the lockfile. |
| `make supply-chain` | `license-check` and `deps-audit`. |
| `make image-scan` | Builds the hub image and scans it plus the pinned postgres image (see Image scanning). Not part of `supply-chain`: needs docker. |

`make supply-chain` needs network (module cache, npm registry, Go and npm advisory databases) and
`make web-install` done beforehand (the `build` step needs the frontend dependencies).

## SBOM

Format: CycloneDX 1.x JSON. CycloneDX was chosen because syft writes it natively with component
licences as structured `license.id` or `expression` fields, which the checker reads with the
standard library. SPDX would work equally well; there is no consumer requirement for it yet.

Tool: syft 1.57.2, installed by `scripts/install-syft.sh` with a pinned SHA-256 of the release
archive (the same pattern as `scripts/install-gitleaks.sh`).

| File | Content | How |
| --- | --- | --- |
| `go.cdx.json` | Go main module (`go.mod`), licences from `LICENSE` files in the module cache | `go-module-file-cataloger`, `go mod download` first |
| `npm-prod.cdx.json` | npm production tree | `npm ci --omit=dev --ignore-scripts` in a scratch copy of `package.json` and the lockfile, then `javascript-package-cataloger` |
| `npm-dev.cdx.json` | npm production plus dev tree | same, without `--omit=dev` |
| `server-binary.cdx.json` | the built `bin/sakuraops-server` | `go-module-binary-cataloger` (module and toolchain info embedded in the binary) |

Why not scan `package-lock.json` directly: the lockfile carries no licence field, so syft would
report every npm package as licence-less. Installing from the lockfile (scripts disabled, scratch
directory, `frontend/node_modules` untouched) gives the `license` field of each installed
`package.json`. A consequence: optional platform-specific packages for other OS or CPU
combinations are in the lockfile but not installed on the runner, so their licences are not
scanned (they are the same projects as the installed variants, e.g. lightningcss, rolldown).

The CI job uploads `bin/sbom/*.cdx.json` as the artifact `sbom-cyclonedx`, retention 7 days. The
SBOMs list package names, versions and licences only.

## Licence policy

File: `supply-chain/license-policy.json` (JSON so the checker needs only the standard library).

- `allowed`: explicit SPDX identifiers (MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, 0BSD,
  BlueOak-1.0.0, CC0-1.0, Unlicense, Python-2.0, MPL-2.0). MPL-2.0 is allowed for unmodified
  third-party use. **This restriction is not enforced automatically**: the checker only sees an
  SPDX identifier and cannot tell whether a package was patched. Any patch, fork or vendored
  modification of an MPL-licensed package needs a manual review and recorded patch provenance
  (where the modified copy lives and how it differs from upstream) in the pull request that
  introduces it. Today MPL-2.0 appears only in the dev tree (lightningcss, a build tool).
- `denied_ids` and `denied_prefixes`: `GPL-*`, `AGPL-*`, `LGPL-*`, `SSPL-*`, `BUSL-*`,
  Commons Clause, `LicenseRef-*`, `NOASSERTION`, `NONE`, `UNKNOWN`. The deny list wins over the
  allowlist; a file that lists the same identifier in both is rejected at load time. LGPL is
  denied by default; it is allowed only through a reviewed exception.
- `first_party`: versionless package URLs (`pkg:golang/github.com/MiyazakiTakara/sakura-ops`,
  `pkg:npm/sakuraops-frontend`) of this repository's own modules. A component is skipped only when
  its `purl`, without version, qualifiers and subpath, equals an entry exactly. The ecosystem is
  part of the identity, a name prefix does not match, and a component without a `purl` is never
  first-party. Components nested under a skipped component are still evaluated. Residual limit:
  a registry package with the identical purl is indistinguishable from the first-party one.
- `exceptions`: waivers of exactly one package URL for exactly one licence, each with `reason` and
  `reviewer`. Fields: `purl` (full, with type, name and version, e.g. `pkg:npm/foo@1.2.3`),
  `license` (the exact licence string being waived, as printed in the per-licence count of the
  run, compared after trimming and collapsing whitespace; case-sensitive), `reason`, `reviewer`.
  An exception applies only if the component's `purl` equals the exception `purl` exactly and its
  evaluated licence equals `license`; a different version, ecosystem or licence is still a finding
  and a component without a `purl` can never be waived. The policy is rejected at load for a
  missing field, a versionless or non-purl `purl`, a duplicate `purl` and unknown fields. Current
  entries: two Go detection artefacts in the contract-test dependencies, where syft reports a file
  it cannot classify as `sha256:<hash>` (kin-openapi `openapi3/license.go`, a source file;
  go-yaml v4 `NOTICE`, Apache-2.0 plus MIT for the libyaml port). Both real licences are
  allowlisted, and the hash pins the reviewed file. Since #50 there are also pgx v5 (Go source
  file `pgproto3/notice_response.go` misread, MIT, shipped) and three modernc.org modules (libc,
  memory, sqlite). The modernc modules are in the module graph only through goose's own tests and
  are never compiled (`go list -deps -test ./...`); the Go SBOM is built from the module graph, so
  it is broader than the build. The server-binary SBOM is the exact shipped set. Original licence
  files, reproduced hashes and the owner approval are recorded in
  [licence-exceptions.md](licence-exceptions.md). An exception that waives nothing is a **failure** (exit 1), so stale waivers get
  removed.

Evaluation (`internal/licensecheck`, unit tested):

- `A OR B` passes if any operand passes (the consumer may choose). `A AND B` requires all operands.
  `AND` binds tighter than `OR`; parentheses are supported.
- `X WITH Y` is one identifier and must be allowlisted verbatim; its base licence must not be
  denied.
- Several licence entries on one component are all required (joined with `AND`).
- A component with no licence, a free-text licence name, an unparseable expression or an identifier
  that is neither allowed nor denied fails as unknown or unlisted. Nothing unknown passes.
- Nested `components[].components[]` and the children of `metadata.component` are evaluated
  recursively (depth limit 32). The root `metadata.component` itself is not a dependency and is
  not counted. `file` entries and first-party entries are skipped but never hide their children.
- An SBOM that yields zero evaluated components (empty, or only `file` and first-party entries)
  is an error (exit 2), for every SBOM passed, so a broken scan cannot pass silently. All four
  SBOMs from `scripts/sbom.sh` are dependency-bearing; there is no opt-out.

Exit codes of `cmd/licensecheck`: 0 pass; 1 findings or unused exceptions; 2 usage, policy or SBOM
error.

Offenders are printed as `package`, `version`, `licence`, reason and SBOM file. The run also
prints a count per licence.

### Requesting an exception or a new allowed licence

Open a pull request that edits `supply-chain/license-policy.json`:

1. For a new allowed identifier: explain why it is compatible with distribution of a self-hosted,
   browser-delivered product, and whether it is copyleft; the owner must approve.
2. For an exception: add the full `purl` with version, the exact `license` string, `reason` (what
   the licence really is, where the dependency is used, why it cannot be avoided) and `reviewer`.
   Exceptions are per version and per licence, so an upgrade or a licence change needs a new
   review. Remove the entry when the dependency goes away.
3. A deny-list change needs the owner's explicit approval.

Nothing in the policy is relaxed to make a test pass; a failing scan means fix the dependency or
get a reviewed exception.

## Vulnerabilities

- Go: `govulncheck ./...` (pinned in `tools/go.mod`), reachability-aware. Fails on any finding.
- npm: `npm audit --omit=dev --audit-level=high` (what ships in the SPA bundle) and then
  `npm audit --audit-level=high` (including dev and build tooling). Both fail on high or critical;
  lower severities are printed and do not fail. The second run is a superset by design: the
  separate first run names the production failures clearly in the log, and the full run covers
  build tooling that executes in CI and on developer machines.
- osv-scanner was not added: Go is covered with better precision by govulncheck, and npm advisories
  come from the GitHub advisory database that `npm audit` uses. A second scanner would add a
  binary to pin without new coverage. Revisit if a language without its own scanner is added.
- `actions/dependency-review-action` is **not enabled**: it needs the repository dependency
  graph, which is disabled in this repository (the API answers `Forbidden`). Enabling it is a
  repository setting for the owner; tracked in #24. Until then, the licence policy over the SBOM,
  `npm audit` and govulncheck above are the enforced checks.

## Image scanning

`make image-scan` (CI job `Container image vulnerability scan`, `scripts/image-scan.sh`) builds the
hub image from `deployment/docker/hub.Dockerfile` (the Dockerfile Compose uses, local tag
`sakuraops-hub:image-scan-local`, override with `IMAGE_TAG`), pulls the PostgreSQL image pinned by
digest in `deployment/compose/compose.dev.yaml` (the digest is read from that file, so the scan
cannot drift from what Compose runs) and scans both with grype. Needs docker and network.

Scanner: grype 0.122.1, installed by `scripts/install-grype.sh` with a pinned SHA-256 of the release
archive (same pattern as syft and gitleaks; no `latest`, no `curl | sh`). Grype was chosen over
Trivy because it is from the same project as the already pinned syft, ships as one static binary,
and emits JSON with explicit fix versions and state, which `cmd/imagescan` evaluates. The
vulnerability database is downloaded at run time (not cached, so results are current) and is not
pinned: a new advisory can fail a PR that touched nothing, which is the intent. The database is large (about 3.6 GB unpacked at v0.122.1) and its first import can take
several minutes on a slow or loaded machine; the CI job has a 30 minute timeout.

Policy (`internal/imagescan`, `cmd/imagescan`, unit tested; grype itself is run without a failure
threshold and the verdict comes from the checker):

- HIGH or CRITICAL with a fix available: **fail**.
- HIGH or CRITICAL without a fix: printed as `WARNING (unfixed)`, does not fail.
- Lower severities are not evaluated.
- Reports in `bin/image-scan/` (artifact `image-scan-reports`, 7 days): grype JSON and CycloneDX
  SBOM per image. They hold package names, versions and advisory ids only.

Exceptions: only `supply-chain/image-scan-ignore.json`, reviewed in a pull request. Each entry
has `id` (advisory id, CVE or GHSA, matched against the finding and its related ids), `image`
(`hub` or `postgres`), `package` (exact name), `reason`, `reviewer` and `expires` (`YYYY-MM-DD`,
valid through that day). No wildcards, unknown fields or duplicates. An expired entry fails and
no longer suppresses; an entry that matches no HIGH/CRITICAL finding fails (remove it). Entries
are per image and package, so a fix elsewhere is not hidden. There is no severity downgrade and no
exit-code override.

Coverage: OS packages and language packages found by syft in the image layers, including base
image layers (node and golang stages are build-only and not in the final hub image; the final hub
image is distroless static plus the Go binary). Limits: only these two images; the final image
has no package manager database for the Go binary beyond embedded module info; grype matches
against advisory data and can produce false positives or miss unpublished issues; vulnerabilities
without distro fix data show as unfixed and only warn; the build-stage images are not scanned.

## Coverage and limitations

Covered:

- Go main module dependencies (`go.mod`) and the built `bin/sakuraops-server` (linux/amd64, the
  runner and developer platform).
- The npm tree from `frontend/package-lock.json`, production and production plus dev, as installed
  on the runner.

Not covered:

- The Go tools module `tools/go.mod` (buf, sqlc, goose, staticcheck, govulncheck, plugins): not
  shipped, not in the SBOMs, no licence scan. Scanning it is a follow-up if it ever ships.
- Other OS and architecture builds. The binary SBOM describes the linux/amd64 build only, and npm
  optional packages for other platforms are in the lockfile but not installed, so not scanned.
- Container images other than the hub image and the pinned postgres image (see Image scanning).
- Accuracy of licence data: it depends on package metadata and on syft's `LICENSE` detection for
  Go modules; the policy does not prove the identifier is correct.
- MPL-2.0 modification checks (manual review, see above).
- GitHub `dependency-review-action` is **not active**: the repository dependency graph is
  disabled. It stays tracked in issue #24 until the graph is enabled and the check passes.

## Out of scope here

- Release signing, provenance and attestations of release artefacts: NEXT-024 and NEXT-046,
  docs/19-release-plan.md.
- Container image licence checks (only vulnerabilities are scanned, see Image scanning).
- Reading and legally reviewing the licence texts of the transitive tree. The policy trusts the
  SPDX identifier that the package metadata or syft's `LICENSE` detection reports; it does not
  prove that identifier is correct.
- PostgreSQL client libraries: checked automatically once they enter `go.mod`; nothing is added
  to the policy for them in advance.
