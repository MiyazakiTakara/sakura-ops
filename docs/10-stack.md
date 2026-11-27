# 10. ADR-002: jednolity stack Go + Svelte

Status: propozycja techniczna po decyzji właściciela o zmianie stacku (27.11.2026), do zatwierdzenia przed rozpoczęciem implementacji. Zakres bez zmian: Linux-only, browser-only, własny agent, pływający terminal skr, wbudowane zadania, opcjonalny SSH WWW/Git/Keycloak, jedna VM na start. Nie jest to wynik benchmarków ani gotowy system.

## Decyzja i uzasadnienie

| Warstwa | Proponowana technologia | Cel |
| --- | --- | --- |
| Hub API | Go, net/http, chi | Jeden język serwerowy i agenta, proste HTTP, context i cancellation |
| GUI | Svelte 5, TypeScript, SvelteKit adapter-static, Vite | Komponenty i routing, statyczny SPA bez Node w runtime |
| CSS | CSS variables/tokens i scoped CSS | Zachowanie istniejącej identyfikacji, bez konieczności przyjęcia gotowego design systemu |
| Terminal/edytor | xterm.js, CodeMirror 6 | Zachowanie interakcji, sprawdzone biblioteki zamiast własnego emulatora |
| Agent | Go, standardowa biblioteka, systemd | Binarka Linux, brak zależności od Python/Node, typowane akcje |
| Agent transport | grpc-go, Protobuf, mTLS | Dwukierunkowy stream, kontrakt i kompatybilność wersji |
| User API | REST/OpenAPI, SSE | Przeglądarka, skr, live events, jeden model autoryzacji |
| SSH WWW | Go, sprawdzona biblioteka SSH, WSS | Oddzielna sesja, bez własnego protokołu SSH |
| DB | PostgreSQL, pgx v5, sqlc, Goose | Typowane zapytania SQL i jawne migracje |
| Worker/scheduler | Go, PostgreSQL durable queue | Bez Redis/RabbitMQ na start, trwałe stany i fencing |
| Lokalny journal | trwały embedded store po spike (np. bbolt) | dedup i recovery, nie centralna DB |
| CLI | Go, Cobra | Opcjonalna binarka tekstowa, shared API client |
| Auth | Argon2id, Go OIDC/OAuth2/PKCE, HttpOnly sessions | Lokalny login i opcjonalny Keycloak, bez nowego IdP |
| Deploy | Docker Compose, systemd, podpisane artefakty | Prosta instalacja i aktualizacja |

Nie wybieramy konkretnych numerów wersji bez sprawdzenia aktualnego wsparcia, licencji, CVE oraz kompatybilności. Pin toolchains i zależności w etapie 0. Go nie jest automatycznie szybsze ani bezpieczniejsze w każdej aplikacji; korzyść to jeden typowany język systemowy i mniejsza liczba środowisk wykonawczych. Agent korzysta z istniejących narzędzi Linux, a jego binarka nie eliminuje wymagań OS/systemd/apt/dnf. Nie obiecujemy zerowego zużycia zasobów.

### Alternatywy

- Python/FastAPI + Go: szybkie iteracje API i istniejący kod, jednak dwa języki backendowe, dwa zestawy bibliotek, większa powierzchnia utrzymania. Zachować zachowanie i przypadki testowe, nie runtime.
- Rust (Axum/Tokio): dobra kontrola pamięci i kosztów, ale wyższy próg złożoności dla całego zespołu i agentów AI. Warto rozważyć przy udokumentowanej potrzebie, nie jako obowiązkową drugą technologię.
- .NET/ASP.NET Core: dojrzały i silnie typowany, lecz wciąż wymaga wyboru strategii runtime/deployment agenta i nie daje automatycznej przewagi nad Go w tym Linux-only projekcie.
- React: dojrzały i popularny, w istniejącej iteracji nie ma dużego wdrożonego React SPA do zachowania. Svelte 5 pozwala uprościć lokalne stany i komponenty; sprawdzić ergonomię integracji terminala. Jeżeli spike ujawni blokadę, decyzję o frontendzie można cofnąć osobnym ADR.

## Architektura

Modularny monolit dla usług domenowych, osobne procesy tylko tam, gdzie uzasadnione izolacją lub trwałością pracy. Jedno Go repo, cmd dla różnych binarek, brak niezależnych mikroserwisów per tabela. Serwer www obsługuje statyczny build, API i SSE. Agent gateway nie ma uprawnień admina, API nie ma dostępu do klucza CA ani dowolnego SSH. Signer i issuer pozostają odrębnymi granicami, nawet jeśli korzystają ze wspólnych pakietów Go. Worker/scheduler mogą korzystać ze wspólnej bazy z osobnymi rolami i ograniczeniami. SSH gateway oddzielny, opt-in. Nie scalać tych procesów dla wygody instalacji.

Go embed dla plików statycznych oraz migracji; fallback SPA wyłącznie dla tras aplikacji, nigdy dla /api lub innych zastrzeżonych ścieżek. SvelteKit adapter-static jako klient: brak SSR, server routes, node adapter i drugiego backendu. Haszowane assety z cache, index bez długiego cache, CSP bez niepotrzebnego unsafe-inline, brak danych sesji w bundle. API z same-origin cookies/CSRF. Klient generowany z OpenAPI (np. oapi-codegen po stronie Go i openapi-typescript po stronie TS, ostateczny wybór po spike); drift sprawdzany w CI.

Agent: osobny endpoint HTTP/2, gRPC bidirectional streaming, Protobuf z version negotiation, max frame/queue, reconnect, dedup, deadlines i cancel. TLS clientAuth i pinned trust, odwołanie certyfikatu zamyka stream. Enrollment nadal zwykłe HTTPS dla bootstrap, nie wymaga ważnego certyfikatu agenta przed rejestracją. Przy publicznym API reverse proxy nie może przekazywać tożsamości z niezaufanego nagłówka. Agentowy mTLS testować osobno; bez przechodzenia na WSS bez ADR. gRPC nie jest potrzebny w przeglądarce. Podpis zlecenia jest niezależny od transportu: nie podpisywać niejawnie deterministycznego Protobuf jako kanonicznego formatu; wersjonowane canonical bytes i fixtures dla weryfikacji. Nieznane typy akcji fail closed.

PostgreSQL: explicit transactions, row locks/leases, unique idempotency keys, durable outbox i recheck przy dispatch. LISTEN/NOTIFY tylko jako sygnał do odczytu stanu, nie trwała kolejka. Nie wysyłać SQL z przeglądarki. sqlc generuje typy Go z jawnych zapytań; Goose uruchamia migracje niezależnie z kontrolą blokady i wersji. Osobne role DB; connection pools, context timeouts. Agent nie ma dostępu do PostgreSQL.

## Wdrożenie i repo

`cmd/` (osobne executable), `internal/` (domeny i adapters), `db/migrations`, `db/queries`, `api/openapi`, `proto`, `frontend`, `builtin`, `deployment`, `scripts`, `tests`. Wspólne pakiety tylko z uzasadnionym kontraktem. Brak systemowych kluczy w repo i obrazie. Multi-stage build Node -> static dist -> Go -> minimalny non-root runtime; agent podpisane deb/rpm lub instalator w etapie 3, z binarką amd64/arm64 zależnie od macierzy testów. Test na rzeczywistych VM z SELinux i systemd, nie tylko kontenerach. Go bez CGO tam, gdzie potwierdzono działanie i zgodność; nie deklarować statyczności bez sprawdzenia artefaktu. Zewnętrzny PostgreSQL i izolowane komponenty nadal potrzebne; nie obiecywać pojedynczego procesu całego systemu.

## Bramka technologiczna NEXT-002/003/004/006

1. Minimalny Go HTTP health i autoryzacja testowa, statyczny Svelte build serwowany przez Go, bez Node w runtime. Pływające okno skr: Tab, historia, resize, CodeMirror i Svelte reactivity w przeglądarce.
2. OpenAPI generation i request/response parity; sqlc queries, migracje Goose na świeżym Postgres i upgrade testowego schematu.
3. Dwa testowe procesy Go z gRPC/mTLS; dwa różne certyfikaty, fail przy obcej CA, revoke, reconnect, deadline, max message, test HTTP/2 i ALPN w docelowej topologii proxy. Bez wykonania zadań na hostach.
4. Go build dla testowej architektury, pomiar RSS/CPU/rozmiaru artefaktów i latency na jawnie opisanym sprzęcie. Brak z góry założonej przewagi liczbowej nad FastAPI.
5. CI tylko PR: go test ./..., race dla wspieranej architektury, fuzz parsera/protokołu i govulncheck; lint, sqlc/proto/OpenAPI generation drift; svelte-check, Vitest, Playwright, integration Postgres, SBOM, podpisy artefaktów na release. Go moduły i Node lockfile, przypięte wersje generatorów, workflow permissions minimalne. Skan licencji zależności przed dystrybucją.
6. Review wyników i zatwierdzenie ADR; jeśli proxy, terminal lub dystrybucja agenta nie spełniają wymagań, poprawić architekturę przed dodaniem realnego wykonania. Nie uruchamiać produkcyjnych hostów.

## Źródła techniczne

- Go: https://pkg.go.dev/std i https://pkg.go.dev/embed
- Svelte TypeScript: https://svelte.dev/docs/svelte/typescript ; adapter-static: https://svelte.dev/docs/kit/adapter-static
- gRPC Go: https://grpc.io/docs/languages/go/basics/ ; authentication: https://grpc.io/docs/guides/auth/
- sqlc migrations: https://docs.sqlc.dev/en/latest/howto/ddl.html
- Cobra: https://github.com/spf13/cobra

To propozycja, nie zaimplementowana zmiana i nie wynik testów. Poprzedni main i PR #22 pozostają nietknięte.

Sakura Task v1: ścisły podzbiór YAML 1.2/JSON, JSON Schema Draft 2020-12 i walidacja semantyczna Go, canonical JSON dla digestu po testach, osobny rejestr kontraktów akcji. Protobuf transportuje podpisane pojedyncze akcje; nie jest językiem autora zadania. Biblioteki parsera, schema i canonicalizacji przypiąć po spike. Zobacz 14-workflow-engine.md.
