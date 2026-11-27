# ADR-0002: Stack Go + Svelte + PostgreSQL

- Status: **Proposed** (akceptacja oczekuje na właściciela)
- Data: 2026-11-27
- Decydent: właściciel projektu
- Zakres NEXT: NEXT-002. Oparty na [10-stack.md](../10-stack.md), uzupełniony o wyniki spike P01 ([spike-results.md](../dev/spike-results.md)).
- Zależy od: [ADR-0001](0001-agent-hub-linux-browser-only.md).

## Kontekst

ADR-0001 wymaga jednego typowanego języka dla huba, agenta i workera, przeglądarkowego GUI z terminalem skr oraz trwałego, transakcyjnego magazynu. Zmiana stacku z poprzedniej iteracji została zadeklarowana przez właściciela 27.11.2026 ([10-stack.md](../10-stack.md)). Ten ADR zamienia propozycję w zapis z wersjami przypiętymi w repozytorium.

## Decyzja

| Warstwa | Wybór (wersja przypięta w repo) | Licencja |
| --- | --- | --- |
| Język, hub, agent, worker | Go 1.27.4 (`go.mod`), stdlib `net/http` | BSD-3-Clause (Go) |
| Transport agenta | `google.golang.org/grpc` v1.85.0, `google.golang.org/protobuf` v1.36.12, HTTP/2 + mTLS (TLS 1.3, ALPN h2) | Apache-2.0, BSD-3-Clause |
| GUI | Svelte 5.59.2, SvelteKit 3.4.1 + `adapter-static` 4.0.1 (SPA), Vite 8.4.0, TypeScript 6.0.3 strict | MIT (TypeScript: Apache-2.0) |
| Terminal i edytor | `@xterm/xterm` 6.0.0, CodeMirror 6 (`codemirror` 6.0.2, `@codemirror/lang-yaml` 6.1.3) | MIT |
| Kontrakty | Protobuf przez `buf` 1.74.0, `protoc-gen-go` v1.36.12, `protoc-gen-go-grpc` 1.6.2; REST/OpenAPI + SSE dla GUI i CLI | Apache-2.0 / BSD-3-Clause |
| DB | PostgreSQL (wersja nie wybrana, P02), pgx v5 (jeszcze niedodany), `sqlc` 1.31.1, `goose` v3.30.2 | sqlc, goose: MIT; PostgreSQL i pgx: niezweryfikowane w tym repo |
| Jakość i bezpieczeństwo | staticcheck 2026.2.2 (v0.8.2), govulncheck v1.10.0, gitleaks 8.30.1 | MIT / BSD-3-Clause / MIT |
| Testy frontendu | Vitest 5.1.2, svelte-check 4.7.8, Prettier 3.10.1 | MIT |

Źródła wersji: `go.mod`, `tools/go.mod`, `frontend/package-lock.json`, `scripts/install-gitleaks.sh`. Licencje odczytane z plików LICENSE i metadanych pakietów ([spike-results.md](../dev/spike-results.md)); licencje zależności przechodnich nie były skanowane.

Pozostałe elementy bez zmian względem [10-stack.md](../10-stack.md):

- **Transporty**: gRPC (bidi stream) po HTTP/2 + mTLS dla agentów, osobny listener i osobne CA urządzeń; REST/OpenAPI + SSE dla GUI i CLI ([23-api-contract.md](../23-api-contract.md)); WSS wyłącznie dla późniejszego SSH.
- **Granice**: signer/issuer oraz agent gateway jako odrębne procesy i uprawnienia, nie scalane dla wygody ([03-architecture.md](../03-architecture.md)).
- **Podpisy zleceń**: proponowane Ed25519 po canonical JSON (nie po serializacji protobuf), domain separator i `key_id` ([13-agent-protocol.md](../13-agent-protocol.md)); fixture canonical bytes przed użyciem. Wybór algorytmu pozostaje do potwierdzenia w NEXT-006.
- **Macierz OS**: [16-linux-compatibility.md](../16-linux-compatibility.md).

## Decyzje i odstępstwa ujawnione w P01

| Temat | Decyzja | Uzasadnienie |
| --- | --- | --- |
| Router HTTP | stdlib `ServeMux` zamiast `chi` (10-stack wymienia chi) | Na obecnym zakresie (healthz, `/api/` 404, SPA) wystarcza; zależność dopiero gdy routing tego wymaga. Do rewizji przy P02/P05. |
| TypeScript 6 | `^6.0.3`, nie 7.x | SvelteKit 3.4.1 ma peer `typescript ^6.0.0`; 7.x dawało ERESOLVE, nie wymuszono. |
| Konfiguracja SvelteKit 3 | w `vite.config.ts`, tsconfig rozszerza `$app/tsconfig`, brak `$lib` | `svelte.config.js` i `$lib` zostały usunięte w SvelteKit 3. Ekosystem szybko się zmienia: lockfile obowiązkowy. |
| Osadzanie UI | `internal/webui/dist` kopiowane przez `make web-build`, w git tylko `.gitkeep` | `go:embed` nie sięga poza pakiet; czysty checkout kompiluje się i testuje, serwer zwraca 503 `frontend_not_built` dla tras aplikacji. |
| CSP | hashe `sha256-` skryptów liczone z osadzonego `index.html` przy starcie; `style-src 'unsafe-inline'` | Skrypt startowy SvelteKit zawiera znacznik wersji zmienny w każdym buildzie. `unsafe-inline` dla stylów jest **zaakceptowanym, udokumentowanym ryzykiem** (atrybuty style Svelte i xterm.js); nie obejmuje skryptów. |
| sqlc | uruchamiany przez `go tool`; zbudował się i działa **bez CGO** (zweryfikowane `CGO_ENABLED=0 ... sqlc version` -> v1.31.1) | Brak potrzeby obejść. gcc jest potrzebny tylko dla detektora wyścigów (`go test -race`). |
| Moduł narzędzi | osobny `tools/go.mod` z dyrektywami `tool` | Narzędzia dev nie wchodzą do grafu zależności produkcyjnych. |
| Rozmiar bundla | trasa spike 767 KB raw / 222 KB gzip | Do optymalizacji w NEXT-011 (podział xterm.js/CodeMirror na trasy). |

Pozostałe niezgodności z [10-stack.md](../10-stack.md): Cobra dla CLI i pgx nie są jeszcze użyte w kodzie (brak CLI i DB w P01); to plan, nie odstępstwo.

## Dowody wykonalności P01

Tylko to, co faktycznie zweryfikowano. Commit `4a309e4`, merge `3122e43` (PR #44). CI na GitHub: [run 46009364154](https://github.com/MiyazakiTakara/sakura-ops/actions/runs/46009364154), event `pull_request`, head `4a309e4`, trzy joby (Go, Web and embedded build, Secret scan) zakończone `success`.

- Hub Go z `/healthz`, osadzonym SPA bez Node w runtime, regułami fallbacku, nagłówkami bezpieczeństwa i CSP bez `unsafe-inline` dla skryptów; testy handlerów i fuzz.
- gRPC/mTLS między dwoma procesami: dwa różne certyfikaty klienta, odrzucenie obcej CA, braku certyfikatu, złego EKU, wygasłego certyfikatu, złej nazwy serwera i TLS 1.2; odwołanie przy handshake i na otwartym streamie (z zamknięciem transportu, także pod backpressure); reconnect z ograniczonym backoffem; deadline; limit 256 KiB; TLS 1.3 i ALPN h2 na połączeniu bezpośrednim.
- Generacja Protobuf z przypiętymi narzędziami i kontrola dryfu; `go test -race`, staticcheck, govulncheck, svelte-check, Vitest (18), build z osadzonym UI, gitleaks.
- Pomiary na jednym laptopie WSL2 amd64 (rozmiar, RSS, handshake): [spike-results.md](../dev/spike-results.md); jednokrotne, orientacyjne.

To jest dowód wykonalności kierunku, nie odbiór produktu ani bramki wydania.

## Bramki weryfikacji odroczone

Pełna tabela statusów: [spike-results.md, "Technology gate status"](../dev/spike-results.md#technology-gate-status-docs10-stackmd). Nie zostały wykonane i nie wolno ich przedstawiać jako zrobionych:

| Bramka | Właściciel (NEXT, issue) |
| --- | --- |
| Interakcja w prawdziwej przeglądarce (przeciąganie, resize, Tab, historia, klawiatura, 360-1920 px, 200% zoom) | NEXT-006 / NEXT-011 (#24 / #26) |
| PostgreSQL: zapytania sqlc, migracje Goose na świeżej bazie i upgrade | NEXT-004 / NEXT-006 (#24) |
| Generacja OpenAPI i parity request/response | NEXT-004 / NEXT-006 (#24) |
| Prawdziwa topologia reverse proxy (HTTP/2, ALPN, passthrough vs terminacja) | NEXT-017 (#28), NEXT-047 (#36) |
| Tożsamość agenta i trwałość odwołań (persistencja, wiele replik, CRL/OCSP, keepalive, limity strumieni, duplikaty połączeń) | NEXT-017 (#28) |
| Obciążenie i reconnect storm | NEXT-050 |
| arm64 i sprzęt docelowy | NEXT-050 / NEXT-071 |
| Pełne CI: fuzz w CI, Playwright, integracja PostgreSQL, SBOM, skan licencji, podpisy artefaktów | NEXT-005 (#24) |
| Uwierzytelnianie testowe w minimalnym serwerze | NEXT-007 / NEXT-008 (#25) |

## Rozważone alternatywy

- **Python/FastAPI + Go**: szybkie iteracje, ale dwa języki backendowe i większa powierzchnia utrzymania; zachowujemy zachowanie i przypadki testowe, nie runtime.
- **Rust (Axum/Tokio)**: dobra kontrola pamięci, ale wyższy próg złożoności; do rozważenia przy udokumentowanej potrzebie.
- **.NET/ASP.NET Core**: dojrzały, lecz bez przewagi nad Go dla projektu Linux-only i wymaga osobnej strategii runtime agenta.
- **React**: brak dużego wdrożonego React SPA do zachowania; Svelte 5 upraszcza lokalne stany. Integracja xterm.js i CodeMirror ze Svelte nie wykazała blokad w spike, więc decyzja stoi; w razie blokady cofnięcie osobnym ADR.

## Konsekwencje

- Jeden język serwerowy i agenta, jedna binarka huba z osadzonym UI, brak Node w runtime. Go nie jest automatycznie szybsze ani bezpieczniejsze: korzyścią jest mniej środowisk wykonawczych, nie obiecujemy zerowego zużycia zasobów ani przewagi liczbowej.
- Szybko zmieniający się SvelteKit/Vite wymaga przypiętych wersji, lockfile i przeglądu przy każdej aktualizacji.
- Zależność od utrzymania gRPC i generatorów; dryf generowanego kodu pilnowany w CI.
- Dwa ryzyka jawnie przyjęte: `style-src 'unsafe-inline'` oraz rozmiar bundla spike.

## Kryteria akceptacji (Proposed → Accepted)

1. Właściciel przegląda [spike-results.md](../dev/spike-results.md) i ten ADR.
2. Bramki z sekcji "Bramki weryfikacji odroczone" są śledzone w wymienionych issue, ale **nie blokują** akceptacji kierunku (Go + Svelte + PostgreSQL). Blokują odpowiednie późniejsze etapy, jeśli się nie powiodą; wtedy ADR wymaga korekty lub zastąpienia nowym ([README](README.md)).

Ten ADR nie jest zaakceptowany. Akceptacja nie upoważnia do dostępu produkcyjnego, helpera ani SSH.
