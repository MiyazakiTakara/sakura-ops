# 17. Testowanie i laboratorium

Stan: strategia, żadnych nowych testów jeszcze nie wykonano. Testy uruchamiane przez wykonawcę w dostępnym lab, nie przez produkcyjne hosty.

## Środowiska

- Unit: Go standard library tests, table-driven, fuzz, race; Svelte/Vitest, fixtures bez sekretów; offline.
- Integration: izolowany PostgreSQL i testowe certyfikaty, tymczasowe katalogi, symulator agentów, test gRPC mTLS na localhost/lab bridge. Oddzielny projekt Compose, brak routingu do produkcji.
- Browser: Playwright na lokalnym UI, rzeczywiste API i seed fikcyjnych danych; screenshoty 360/768/1280/1920 px, 200% zoom i klawiatura; nie fake PASS.
- Lab: jedna VM hub i dwie disposable VM Ubuntu 24.04 / Oracle Linux 8 jako P0; snapshot przed zmianami, odizolowane VLAN/virtual network, jawna allowlista egress do kontrolera i testowych repo. Rozszerzaj macierz zgodnie z 16. Nie wykorzystywać realnej infrastruktury uczelni w testach bez oddzielnej zgody.
- Security: odrębne testy API, agent, signer/issuer, helper, SSH gateway; brak testów ofensywnych poza lab.

## Test matrix

| Obszar | Krytyczne przypadki | Gate |
| --- | --- | --- |
| Go/TS/schema | parser duplicate key, nieznany field/action, types, fuzzy input, equal YAML/JSON digest, wszystkie 18 manifestów | 0/2 |
| Auth/RBAC | CSRF, session rotate/revoke, IDOR, scope list/count, self approval, reauth | 1/2 |
| DB/worker | fresh/upgrade migration, race claim/approval, unique occurrence/idempotency, fencing, timeout, crash unknown, retry read | 2/5 |
| Agent | cert/identity mismatch, wrong signer, replay, expiry, lost ACK, clone, reconnect, resource limits | 2/3 |
| Enrollment | verified artifact, expired/reused invite, mismatch, interrupted/repeated install, SELinux, revoke/uninstall | 3 |
| Adapters | cached APT/DNF, security unknown, unsupported, output normalization, units, no root/network | 4 |
| GUI/skr | navigation, actions, help/Tab modes, editor, focus/reflow, same API/plan, SSE reconnect, no dead controls | 1/4 |
| Scheduler | DST gap/fold, UTC, missed skip, lock, owner revoke, no overlap, plan stale | 5 |
| SSH | pinned host key, individual OS login, WSS origin, scope, expiry, revoke, blocked port/forward, no secret log | 6 |
| Privileged | local allowlist, exact effect, helper peer identity, canary failure, partial, recovery, no arbitrary root | 7/8 |
| Operations | backup/restore, CA/nonce reconciliation, upgrade/rollback, artifact provenance, limits/load | 6/8 |

## CI i komendy docelowe

Na pull_request: gofmt/lint, go test ./..., go test -race ./... gdzie wspierane, celowane fuzz z limitem czasu, govulncheck, go build, sqlc/proto/OpenAPI generation drift, PostgreSQL integration + Goose migrations; npm ci (lub zatwierdzony lockfile), svelte-check, Vitest, Playwright, build statyczny; shellcheck/Bats instalatora, schema fixtures, secret/license/SBOM scan. Polecenia finalizować wraz z NEXT-003/005, nie udawać że istnieją dziś. Actions pinned, minimalne permissions, brak sekretów i deployment z PR fork. Release job osobny, ręczny i chroniony. Nie wysyłać tokenów/realnych logów do artefaktów.

## Odbiór manualny

Dla każdego gate tabela: SHA, date, OS/arch, environment (lab only), case ID, komenda lub opis kroku, expected, actual, status PASS/FAIL/NOT RUN, sanitized evidence, reviewer. Wynik starego SHA nie zastępuje aktualnego. Negatywne przypadki wymagane. Wysokie ryzyko: snapshot/restore i health przed/po; problem zatrzymuje etap. Mierzyć RSS/CPU/latency/queue/reconnect bez wymyślonych progów; ustalić limity po pomiarze. Przed produkcją dokumentowany restore i oddzielny sign-off.
