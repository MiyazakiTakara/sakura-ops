# 03. Architektura i bezpieczeństwo

## Decyzje techniczne (do zatwierdzenia w ADR)

Propozycja ADR-002: Go dla centralnego API, agenta, dispatcher/scheduler, signer/issuer, SSH gateway i opcjonalnego CLI skr (Cobra). HTTP net/http + chi, pgx/sqlc + Goose dla PostgreSQL. Svelte 5/TypeScript/SvelteKit adapter-static z Vite dla panelu, statyczne zasoby osadzane w binarce hubu lub serwowane jako pliki. Szczegóły i trade-off w 10-stack.md. Brak Pythona i Node.js jako runtime produkcyjnego. Terminal WWW: zachować interakcję skr i sprawdzone komponenty emulatora terminala/CodeMirror po przeglądzie licencji i bezpieczeństwa. Późniejszy SSH w przeglądarce przez wydzielony gateway; bez klienta natywnego. Linux jest jedynym zarządzanym OS w obecnej roadmapie. Osobne komponenty z jasno typowanym kontraktem. Nie pinujemy dziś arbitralnie numerów bibliotek: etap 0 sprawdza kompatybilność i ustala lockfile/digest. Jedna VM Docker Compose, bez obowiązkowego Kubernetes, Redis, chmury ani Ansible. Agent Linux jako jawna usługa systemd.

    Panel WWW / terminal skr / opcjonalny tekstowy CLI
                 | HTTPS
          Reverse proxy / API
                 | transakcyjne dane, policy, podpis planu
              PostgreSQL
                 | kolejka trwała, outbox
          Dispatcher / Agent Gateway
                 ^ wychodzący kanał mTLS z agenta
                 |
       Agent (nie-root) na zarządzanym hoście
                 | lokalne typowane akcje
       Adaptery OS / opcjonalny ograniczony helper
     
       Karta SSH WWW -> autoryzacja sesji -> osobny SSH gateway
                 -> dozwolona usługa hosta, normalny login

API nie wykonuje komend OS, nie montuje docker.sock, nie ma klucza CA agenta ani dowolnego dostępu do hostów. Agent Gateway uwierzytelnia wyłącznie urządzenia, nie użytkowników. Dispatcher dostarcza tylko podpisane, zatwierdzone zlecenia. Scheduler tworzy plany w granicach grantów, nie ma kluczy agenta. Osobny signer/issuer z minimalnymi uprawnieniami i ograniczonym protokołem; podpis sam nie chroni przed pełną kompromitacją signera, dlatego agent niezależnie egzekwuje lokalną allowlistę i profile. Broker sesji oddzielony od automatyzacji.

## Struktura repo

cmd/{sakuraops-server,sakuraops-agent,sakuraops-worker,sakuraops-agent-gateway,sakuraops-signer,sakuraops-issuer,sakuraops-helper,sakuraops-ssh-gateway,skr}; internal/{api,auth,hosts,enrollment,catalog,policy,runs,schedules,audit,integrations,agent,console,sshaccess,store}; db/{migrations,queries}; api/openapi; proto/sakuraops/agent/v1; frontend/src/{routes,lib}; builtin/catalog; scripts/; deployment/; tests/{integration,e2e,security,lab}; docs/. Wspólne wersjonowane kontrakty OpenAPI/Protobuf, generowane typy, testy kompatybilności. Jedno repo Go, ale osobne binarki i granice uprawnień. Katalog wbudowany z release, nie plik wykonywalny przekazywany przez operatora.

## Tożsamość i sieć

Agent sam łączy się do jednego zatwierdzonego adresu kontrolera po HTTPS (proponowany gRPC bidirectional stream po HTTP/2 i mTLS; osobny listener urządzeń, test ALPN i proxy w etapie 0, bez cichego fallback do słabszej autoryzacji). Żadnego nowego portu nasłuchu agenta w podstawowym profilu. Sieć z NAT działa, jeśli agent może dotrzeć do kontrolera i polityka sieci to dopuszcza. TLS z zaufanym CA, weryfikacja nazwy serwera, bez insecure/TOFU; osobna CA urządzeń, klucz CA nie na gateway, unikalny klucz prywatny generowany lokalnie przez agenta, certyfikat clientAuth z agent ID i ograniczonym czasem życia. Klucze plikowe 0600, właściciel odpowiedniej usługi; sprzętowy store później. Certyfikat przypisany do host ID i tożsamości, nie hostname/IP. Rejestr certyfikatów: serial, fingerprint, status, expiry, version; odwołanie zamyka istniejące połączenie i blokuje kolejne, nie tylko oczekiwanie na expiry. Rotacja przed wygaśnięciem z testem, bez automatycznego zaufania do innego kontrolera. Klon VM nie może kopiować enrollment identity; konflikt równoczesnych agentów blokuje host i wymaga wyjaśnienia.

Proponowane ustawienia początkowe do pomiaru: heartbeat co 30s z jitter, offline po 90s, reconnect backoff 2–60s z jitter, max message 256KiB, bounded queues, timeout handshake i idle. Heartbeat jedynie podstawowy stan i capabilities; fakty/zasoby zbierane typowanymi akcjami, nie masowe logi. Agent raportuje protocol version, build digest/version, capability set, boot ID, monotonic seq. Hub nie ufa deklarowanej wersji jako dowodowi integralności hosta. Negotiation zgodnych wersji, unknown = fail closed. Połączenie bez jawnego mTLS/ważnego certyfikatu nie przyjmuje zadań. Nie mylić szyfrowania z autoryzacją.

## Model danych

UUID, UTC w DB, timezone w UI, FK/unique/transakcje, soft retirement, immutable versions. Najważniejsze encje:

| Encja | Pola i reguły |
| --- | --- |
| User / Role / Scope / ApiToken | role, grupy, active; token hash, scope, expiry, revoke, last_used; OIDC subject+issuer |
| Host / Group / HostGroup | alias unique, opis, lab/prod, enrollment_state, capabilities, host version, agent ref, access policy; adres opcjonalny dla diagnostyki |
| AgentIdentity / AgentCertificate | host UUID, public key fingerprint, serial, issued/expires, revoked, protocol version, last_seen, boot_id; prywatny klucz nigdy w hubie |
| HostObservation | source_run, observed_at, ttl, system, typed facts; brak domyślnych wartości sugerujących sukces |
| EnrollmentInvite / Attempt | hash tokenu, expiry, expected host/profile, state, CSR fingerprint, creator, nonce, claimed_at; atomic claim |
| Task / TaskVersion | stable ID, immutable version/digest, action ID, platform, capabilities, risk, params/output schemas, timeout, policy |
| Plan / Approval | frozen host IDs+versions+capabilities, agent identities, params digest, task digest, actor, policy revision, expiry, risk, approver, maintenance |
| Run / HostResult / RunEvent | plan ID, per-host command ID, idempotency key, lease, state, timestamps, counters, result kind, error code, seq |
| Schedule / Occurrence | rule, timezone, owner, grant, target snapshot rule, next_at, status, unique(schedule, scheduled_at) |
| SessionPolicy / Session / SessionEvent | host/service allowlist, actor, protocol, expiry, start/end, bytes optional, reason, status, revoke; nie hasło SSH |
| AuditEvent | actor, action, object, reason, time, request_id, plan digest, result; sanitized append-only dla API |

Rejestr hostów w DB jest źródłem prawdy. Opcjonalny statyczny import z inventory/CSV: preview/mapowanie/konflikty, bez importu sekretów i bez uruchamiania pluginów. Import nie instaluje agenta ani nie aktywuje hosta. Eksport sanitized JSON/CSV i opcjonalne prywatne wersjonowanie manifestów później.

## API v1 (projekt)

- /auth/login,/logout,/me; /settings; /system/status; /users,/roles,/tokens.
- GET/POST /hosts; GET/PATCH /hosts/{id} (If-Match); POST /hosts/{id}/retire; /hosts/{id}/observations; /groups.
- POST /enrollments; GET /enrollments/{id}; POST /enrollments/{id}/claim (osobny ograniczony bearer); /approve; /revoke; /renew (uwierzytelnienie urządzenia).
- GET /tasks; /tasks/{id}/versions/{version}; POST /plans; GET /plans/{id}; POST /plans/{id}/approve; POST /runs (plan_id + idempotency key); GET /runs; /runs/{id}; POST /runs/{id}/cancel; GET /runs/{id}/events (SSE Last-Event-ID); /results.
- /schedules (CRUD w granicach grantów), /pause,/resume; /audit.
- POST /sessions/plan; /sessions/{id}/approve; /sessions/{id}/connect; /sessions/{id}/revoke; GET /sessions (moduł późniejszy).

Każde mutujące API z kontrolą RBAC, zakresu hostów, CSRF przy sesjach, audytem i idempotency. OpenAPI, paginacja cursor, błędy code/message/request_id/field_errors, kody 401/403/409/422/429, brak traceback w UI. SSE sprawdza uprawnienia również po zmianie roli, ogranicza połączenia i pozwala wznowić strumień. SameSite i same-origin domyślnie. Certyfikat urządzenia nie daje dostępu do API użytkownika. API tokeny hashowane, jawne scopes, revocable; opcjonalny CLI przechowuje tokeny w systemowym store, nie w historii; przeglądarka używa HttpOnly cookies, nie trwałych bearer tokens w localStorage.

## RBAC i zgody

Viewer: odczyt zasobów/wyników w dozwolonej grupie. Operator: diagnostyka i tworzenie planów w grupie. Approver: zatwierdza zmiany w swoim zakresie, nie własne ryzykowne zadanie. RemoteAccess: odrębne uprawnienie do konkretnego hosta/protokołu; nie wynika z samej roli operatora. Admin: rejestracja, użytkownicy, polityka, integracje; nie omija wymogu drugiej osoby na produkcji. Service account scheduler: grant konkretnej wersji zadania, parametrów i zakresu; revocation ownera unieważnia grant. Lokalny break-glass z silnym hasłem, audytowany. Opcjonalne OIDC/Keycloak: przypięty issuer i jawne mapowanie, brak implicit email linking, local fallback. Wymagać ponownego uwierzytelnienia przy ryzykownych akcjach i sesjach według polityki.

## Plan, protokół wykonania i stany

Preflight rozwiązuje grupy do host IDs i stanu, sprawdza RBAC, capabilities, wersję agenta, lokalny profil, parametry, typ zmiany, blokady, okno i online. Tworzy immutable plan (proponowany TTL 10 min), prezentuje prosty opis i techniczny manifest. Przed enqueue i wysłaniem ponowna walidacja, zmiana hosta/wersji/grantu/polityki/parametrów = plan_stale. Zlecenie tylko dla jednego agent ID, podpisane przez signer. Agent ufa jedynie przypiętym publicznym kluczom podpisu zgodnym z wersją protokołu; CA transportowa nie wystarcza do podpisania akcji.

Przykładowy kontrakt CommandEnvelope: protocol_version, command_id, run_id, agent_id, action_id, action_version, params, params_digest, plan_digest, policy_revision, issued_at, expires_at, nonce, approval_class, timeout, signature. Protobuf jako transport, lecz podpis nad niezmiennym, jawnie wersjonowanym canonical payload (np. RFC 8785 dla ograniczonego JSON po testach); nie zakładać, że deterministyczna serializacja protobuf jest kanoniczna. Używać standardowej biblioteki kryptograficznej, sprawdzać dokładne podpisane bajty, ścisły schemat, odrzucanie nieznanych pól dla wersji akcji, limity, clock skew i synchronizację zegara. Agent sprawdza własny ID, podpis, TTL, lokalną politykę, schema/action/version, dostępne capabilities i stan; zapisuje command ID/nonce atomowo w trwałym lokalnym journal przed wykonaniem. Nie ufa wyłącznie serwerowemu UI. Unknown action odrzucone. Komunikaty AgentHello, Heartbeat, CommandOffer, CommandAck, CommandEvent, CommandResult, CancelRequest, ResultAck. Event seq i deduplication, bounded buffer, wynik po reconnect; brak arbitralnego script content.

Plan pending_approval; run queued, running, succeeded/partial_failed/failed/cancelled/timed_out/interrupted/blocked; host result unknown po utracie kontaktu. Agent ack accepted nie jest wykonaniem. Hub wysyła co najwyżej powtórzenie tego samego command_id dla deduplication, nie nowej operacji. Agent nie ponawia wykonania komendy o zużytym ID. Po awarii podczas akcji zmieniającej nie da się zagwarantować exactly-once na zewnętrznym OS; oznacz unknown, zablokuj zmiany do ręcznej reconciliacji. Akcje odczytowe mogą dostać nowe polecenie po jawnej polityce retry. Rozróżnić expiry przed startem i timeout działania. Offline host: odczyt z ograniczonym TTL albo skipped; zmiany nie czekają po oknie konserwacyjnym.

## Agent i uprawnienia OS

Agent działa pod dedykowanym nie-root UID, jawna jednostka systemd z minimalnym filesystem access, NoNewPrivileges, sandbox/hardening zgodnie z wymaganiami odczytów, limity CPU/RAM/PID/czasu/logów. Agent może odczytywać tylko potrzebne fakty i własny stan. Standardowe operacje są skompilowanymi akcjami z typed params i adapterami OS. Nie przyjmuje raw shell, skryptów, URL do wykonania, dowolnej ścieżki, pluginu ani polecenia z logu. Wywołania niezbędnych programów systemowych przez stałą ścieżkę i listę zwalidowanych argumentów, bez shell expansion, z czystym env, timeout i ograniczeniem outputu. Zależności i supported versions testowane, unsupported oznaczone.

Profil zmian: opcjonalny, instalowany osobno przez uprawnionego lokalnego admina. Osobny root-owned helper z Unix socket/peer credentials i własną walidacją podpisu, klasą akcji, parametrami i lokalną allowlistą. Agent bez helpera nigdy nie instaluje pakietów ani nie restartuje usług. Nie przyznawać szerokiego sudo agentowi, nie używać dowolnego shell w helperze. Helper rozpoznaje tylko konkretne typy: refresh cache, zatwierdzony pakiet, zatwierdzona usługa, reboot z dodatkowymi warunkami. Katalog zadań nie rozszerza lokalnej polityki automatycznie. Root-level package scripts mogą mieć szerokie skutki; nie obiecywać izolacji dowolnego pakietu. Aktualizacja agenta wymaga zweryfikowanego release i odrębnej polityki instalacji; nie jest generic remote execution.

## Kolejka i harmonogramy

PostgreSQL: claim kolejki FOR UPDATE SKIP LOCKED, unikalny idempotency key, outbox, lease + fencing, lock per host oraz osobny lock package managera po stronie OS. Proponowane limity: 2 równoległe runy na start, 5 hostów równolegle w diagnostyce, zmiany po jednym, timeout ping 30 s, facts 120 s, audyt 300 s, zmiany do 1800 s zależnie od akcji; dostroić w labie. Blokada hosta nie znika automatycznie, jeżeli proces lokalny może nadal działać. Cancel queued atomowo; running współpracująco lub przerwanie własnego process group, zachować unknown gdy brak dowodu, nie zabijać managera pakietów w niebezpiecznym momencie, nie obiecywać rollbacku. Canary, stop-on-failure dla zmian.

Scheduler UTC+IANA timezone, occurrence unique, DST pominięta godzina = skip z audytem, powtórzona = raz; missed runs domyślnie skip, nie nadrabianie. Nie nakładać jobów per host. Dynamiczne grupy tylko przy jawnym grancie odczytu; zmiany zamrożony zakres do ponownej akceptacji. W MVP tylko diagnostyka automatyczna. SMTP opcjonalnie, dedup/throttle, bez sekretów.

## Zagrożenia i kontrola

Model: nadużycie operatora, kradzież invite/token/klucza, klon agenta, fałszywy kontroler, MITM, replay/TOCTOU, modyfikacja envelope, malicious params, lokalne przejęcie hosta, przejęcie huba, SSRF/pivot przez broker, log XSS, przeciążenie, rozjazd wersji i awaria. Agent nie ufa wynikowi z innego urządzenia i nie autoryzuje się sam do innych hostów. Nie da się uznać danych kompromitowanego hosta za wiarygodne; oznaczyć źródło i czas. Hub nie przyjmuje HTML/logu bez escaping. Redakcja strukturalna i allowlist output fields. Wbudowane zadania bez zbierania treści plików prywatnych, sekretów, keystrokes, ekranów. Audit append-only dla API; lokalny admin DB nadal może go zmienić, opcjonalny niezależny eksport później. Testy bezpieczeństwa osobno dla API, agenta, helpera, CA/rotacji i brokera sesji. Zdalny dostęp nie może być tunelowym obejściem tych ograniczeń.

## Zgodność z poprzednim terminalem i Git

Parser skr i GUI wywołują identyczne usługi aplikacyjne, a kontekst trybu nie jest uprawnieniem. Osobne schematy requestów i transport dla skr, operacji edytora i sesji SSH. Terminal nie przyjmuje dowolnego polecenia OS. Zachować interfejsy repozytoriów Git jako opcjonalny moduł importu i wersjonowania definicji typowanych zadań, nigdy kodu wykonywalnego agenta; osobne uprawnienia, potwierdzony diff, pin SHA, walidacja symlink/path i jawny push. Żaden commit/push nie uruchamia zadań. Szczegóły w 09-terminal.md.
