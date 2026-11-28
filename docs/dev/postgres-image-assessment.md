# PostgreSQL image assessment (2026-11-28)

Status: **assessment only. No risk accepted, no exceptions added (`supply-chain/image-scan-ignore.json`
stays empty), owner decision pending, review date 2026-12-28.** Counts are scanner output, not
verified exploitability. Linked from docs/dev/supply-chain.md (Image scanning, Known limitations).

## Method

- Official `postgres` image variants, tags resolved on 2026-11-28 via the registry API (linux/amd64
  manifest of each index). No Docker was used.
- grype 0.122.1 (the pinned version), vulnerability DB schema v6.1.10 built 2026-11-28.
- Two modes per image: `grype registry:postgres@<digest>` (comparable to the repo's `docker:` scan)
  and `grype sbom:` on a syft SBOM of the same image, which additionally sees the Go stdlib in `gosu`.
- Severity and fix state are grype's (Debian tracker, Alpine secdb, NVD, GitHub, Go vuln DB).
  Reachability was not tested (no govulncheck, no runtime inspection, no per-CVE function triage).

## Summary (registry mode)

The current pin `postgres@sha256:64c8b0d3...a9328` is the index digest of `postgres:18-trixie` as of
2026-11-28, so it is not stale: refreshing it today gains nothing.

| Image | Matches | Distinct IDs | Crit | High | HC distinct | HC fixed / not-fixed / wont-fix / unknown |
|---|---|---|---|---|---|---|
| 18-trixie (current pin) | 372 | 146 | 1 | 34 | 35 | 0 / 7 / 28 / 0 |
| 18-bookworm | 547 | 190 | 3 | 39 | 42 | 1 / 17 / 24 / 0 |
| 18-alpine (3.24.4) | 40 | 26 | 0 | 16 | 16 | 11 / 0 / 0 / 5 |
| 17-trixie | 372 | 146 | 1 | 34 | 35 | identical to 18-trixie |
| 17-alpine | 40 | 26 | 0 | 16 | 16 | identical to 18-alpine |

Debian "wont-fix" means the tracker marks the issue ignored or postponed for the release; it is not a
statement that it is harmless. Alpine "unknown" means no fix recorded in secdb (matched via NVD).
Counts are not comparable one to one across distributions (different data sources, 53 vs 148 packages).

## Distinct HIGH/CRITICAL of the current pin, by package

Exposure classes: **server** = linked into the postgres server or libpq path; **entrypoint** = only
the container entrypoint; **tooling** = present in the image but not used by server or entrypoint;
**uncertain** = not verified.

| Package family (installed) | Advisories | Fix state | Exposure |
|---|---|---|---|
| openssl, libssl3t64, openssl-provider-legacy 3.5.9-1~deb13u1 | 9 High | wont-fix | server (TLS, pgcrypto); CLI and legacy provider are tooling |
| libxml2 2.12.7+dfsg+really2.9.14-2.1+deb13u3 | 1 Critical, 7 High | wont-fix / not-fixed | server (SQL/XML functions), uncertain whether the hub schema uses XML |
| glibc (libc6, libc-bin, locales) 2.41-12+deb13u5 | 1 High | wont-fix | server; affected function not examined |
| util-linux family 2.41.5-0+deb13u1 | 4 High | wont-fix | mostly tooling (mount, login); uncertain |
| krb5 / libgssapi 1.21.3-5+deb13u2 | 1 High | wont-fix | only if GSS auth is enabled (not default); uncertain |
| libsasl2 2.1.28+dfsg1-9 | 1 High | wont-fix | via libldap, only if LDAP is configured; uncertain |
| libsqlite3-0 3.46.1-7+deb13u2 | 3 High | wont-fix | not linked by postgres; tooling, uncertain |
| perl 5.40.1-6+deb13u1 | 2 High | wont-fix | tooling (entrypoint is bash) |
| gnupg 2.4.7-21 | 1 High | wont-fix | tooling (build-time key verification) |
| libstdc++6, libgcc-s1 14.2.0-19 | 2 High | wont-fix | server (via ICU); affected component not examined, uncertain |
| ncurses 6.5+20250216-2 | 1 High | wont-fix | tooling |
| libacl1 2.3.2-2+b1 | 2 High | wont-fix | tooling, uncertain |

No findings in the PostgreSQL server package or libpq themselves in registry mode.

## Go stdlib in gosu (not seen by the gate)

gosu v1.20.0 (go1.24.6) is in every variant. Registry mode reports zero Go matches; the SBOM mode
reports 61 distinct Go stdlib advisories (3 Critical, 32 High, 24 Medium, 2 Low), fixed only in
newer Go. Identical across variants, so it does not discriminate between them. gosu runs once in the
entrypoint (only when started as root) with fixed arguments, no listener and no untrusted input; most
advisories concern net/http, crypto/tls, archive and similar packages it does not use. Classified
entrypoint-only, likely unreachable, **uncertain** (not verified with govulncheck). The cause of the
scanner difference is not verified; whether the `docker:` mode behaves like `registry:` was not tested
locally (CI reported the same 111 HIGH/CRITICAL as the registry scan). Fix path: an upstream rebuild
with newer gosu and Go.

## Deltas between variants

- 17 vs 18: identical findings for both Debian and Alpine. The PostgreSQL major does not change exposure.
- bookworm vs trixie: strictly worse (23 shared, 19 only in bookworm, 3 Critical vs 1, older libraries
  such as glibc, sqlite, krb5, zlib, util-linux). Debian 12 support end date not verified.
- alpine vs trixie: 15 shared (same 9 OpenSSL ids and 6 libxml2 ids), 20 disappear (glibc, util-linux,
  perl, gnupg, krb5, sasl2, sqlite, libacl, ncurses, gcc libs, 2 libxml2), 1 new (libxml2
  CVE-2026-11979, fixed in 2.13.9-r5). Critical 1 to 0. 11 of 16 have a fix in the Alpine repository
  that the image published on 2026-11-18 does not yet contain; a newer rebuild may pick them up.
- OpenSSL (9 HC ids) is in the server path on every variant; switching base does not remove it.

## Options for the owner

1. **Status quo:** keep `18-trixie` at the current digest (already the newest). Re-check on the next
   upstream rebuild; most findings are Debian wont-fix, so a refresh changes little until Debian ships fixes.
2. **Scheduled digest refresh** by a manually approved PR, so fixes arrive when Debian ships them.
3. **18-alpine:** about 3x fewer packages, no Critical, more findings with an available fix. Trade-offs:
   musl instead of glibc (behaviour, performance, extension and tooling assumptions); **collation and
   locale differences, and moving an existing cluster between glibc and musl can corrupt text index
   ordering and needs dump/restore or reindex**; fewer locales; different advisory feed; JIT/LLVM
   availability may differ; Debian is the reference platform for upstream testing. Lower counts alone
   are not sufficient reason; for a greenfield database the collation concern is smaller.
4. **bookworm:** not recommended on exposure grounds.
5. **Go stdlib gap:** decide whether the gate should evaluate it. Doing so would fail the unchanged
   policy on every variant until upstream rebuilds with newer Go. Do not self-build the official
   image without owner approval.

## Not determined

Reachability of any advisory in the actual server; whether the hub schema or extensions use XML, GSS,
LDAP or TLS features; the cause of the registry/SBOM difference for gosu; Debian 12 and 13 support end
dates; Alpine "unknown" fix states.
