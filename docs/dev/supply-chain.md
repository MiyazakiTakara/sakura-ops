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

The Go dev tools in `tools/go.mod` (buf, sqlc, goose, staticcheck, govulncheck, plugins) are not
shipped, not scanned for licences and not part of the SBOMs. They are covered by govulncheck only
as far as `make vuln` analyses the main module. Scanning them is a follow-up if they ever ship.

## Licence policy

File: `supply-chain/license-policy.json` (JSON so the checker needs only the standard library).

- `allowed`: explicit SPDX identifiers (MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, 0BSD,
  BlueOak-1.0.0, CC0-1.0, Unlicense, Python-2.0, MPL-2.0). MPL-2.0 is allowed for unmodified
  third-party use only: no patched or forked copy of an MPL file in this repository. Today it
  appears only in the dev tree (lightningcss, a build tool).
- `denied_ids` and `denied_prefixes`: `GPL-*`, `AGPL-*`, `LGPL-*`, `SSPL-*`, `BUSL-*`,
  Commons Clause, `LicenseRef-*`, `NOASSERTION`, `NONE`, `UNKNOWN`. The deny list wins over the
  allowlist; a file that lists the same identifier in both is rejected at load time. LGPL is
  denied by default; it is allowed only through a reviewed exception.
- `first_party`: names of this repository's own modules, skipped by the check.
- `exceptions`: exact `package` + `version` waivers, each with `reason` and `reviewer`. The list is
  empty. An exception that matches nothing is reported as a warning so stale entries get removed.

Evaluation (`internal/licensecheck`, unit tested):

- `A OR B` passes if any operand passes (the consumer may choose). `A AND B` requires all operands.
  `AND` binds tighter than `OR`; parentheses are supported.
- `X WITH Y` is one identifier and must be allowlisted verbatim; its base licence must not be
  denied.
- Several licence entries on one component are all required (joined with `AND`).
- A component with no licence, a free-text licence name, an unparseable expression or an identifier
  that is neither allowed nor denied fails as unknown or unlisted. Nothing unknown passes.
- A SBOM with no components fails, so a broken scan cannot pass silently.

Offenders are printed as `package`, `version`, `licence`, reason and SBOM file. The run also
prints a count per licence.

### Requesting an exception or a new allowed licence

Open a pull request that edits `supply-chain/license-policy.json`:

1. For a new allowed identifier: explain why it is compatible with distribution of a self-hosted,
   browser-delivered product, and whether it is copyleft; the owner must approve.
2. For an exception: add `package`, exact `version`, `reason` (what the licence really is, where
   the dependency is used, why it cannot be avoided) and `reviewer`. Exceptions are per version,
   so an upgrade needs a new review.
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

## Out of scope here

- Release signing, provenance and attestations of release artefacts: NEXT-024 and NEXT-046,
  docs/19-release-plan.md.
- Container image scanning (OS packages, base image licences and CVEs): there are no images yet.
  It must be added together with the Compose and Docker images (follow-up, to be tracked with
  that work).
- Reading and legally reviewing the licence texts of the transitive tree. The policy trusts the
  SPDX identifier that the package metadata or syft's `LICENSE` detection reports; it does not
  prove that identifier is correct.
- Licence and vulnerability scanning of `tools/go.mod` (dev tools, not shipped).
- PostgreSQL client libraries: checked automatically once they enter `go.mod`; nothing is added
  to the policy for them in advance.
