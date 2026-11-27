# 08. Plan realizacji, backlog i bramki

Identyfikatory NEXT są zadaniami planu, nie utworzonymi GitHub Issues. Realizować małymi PR do dedykowanej gałęzi integracyjnej Next, nie do starego main. Każdy PR: konkretna funkcja, pozytywne i negatywne testy, instrukcja QA, review i aktualizacja dokumentacji. CI na pull_request, brak deploy na push. Nie tworzymy tutaj issues ani automatycznych PR bez osobnego zlecenia.

## 0. Fundament (NEXT-001..006)
- 001 ADR: zatwierdzenie modelu agent/hub zamiast Ansible, zakres SSH/RDP, model zagrożeń i granice.
- 002 ADR stack i licencje, protokół mTLS/WS vs gRPC, signer/issuer, wersje i kompatybilność OS, Connect toolchain.
- 003 Monorepo, lockfiles Python/Go/Node, lint/typecheck, .gitignore, secret scanning, minimalny README dev.
- 004 Compose dev, izolowana sieć lab, health, PostgreSQL migrations od pustej bazy.
- 005 CI PR: unit, integration, build, security scan, artefakty bez sekretów.
- 006 UX shell prototype, kontrakt OpenAPI i schemat protokołu, testy kontraktu cross-language.
Odbiór: świeży clone w lab uruchamia health i CI; zero dostępu do zdalnych hostów.

## 1. Logowanie, role, szkielet (NEXT-007..012)
- 007 Użytkownicy lokalni, Argon2, sesje, CSRF, throttle, bootstrap admin z TTY.
- 008 RBAC + host-group scope, testy IDOR, scoped tokens, audit login/role changes.
- 009 Routing Pulpit/Serwery/Zadania/Ustawienia; stany puste/loading/error, bez dead links.
- 010 Ustawienia instancji (URL, timezone, scope), status usług z rzeczywistych danych.
- 011 Ograniczony terminal skr, help/history/completion i CLI skeleton, wspólne API.
- 012 E2E auth, accessible keyboard/mobile, break-glass docs.
Odbiór: viewer nie mutuje, inna grupa nie widzi hosta, brak dowolnego shell i brak remote access.

## 2. Rejestr, protokół i kolejka lokalna (NEXT-013..021)
- 013 Host/Group/AgentIdentity, wersjonowanie, scope, retirement; lista i karta.
- 014 TaskVersion, schema params/output; fixed localhost/simulator action, wizard.
- 015 Immutable Plan, RBAC, expiry, podpis i nonce, stale checks.
- 016 Agent core Go: protokół, strict parser, lokalny journal, typed action registry, nie-root.
- 017 Gateway i issuer lab, mTLS, heartbeat, reconnect, revocation, rozjazd wersji.
- 018 Dispatcher, trwała kolejka, locking/fencing, Run/HostResult/Event, SSE, cancel/timeout/crash.
- 019 Audit i redaction, result pagination/filters, historia.
- 020 Test symulowanego agenta i rzeczywistego agenta na localhost lab, network boundary i brak root.
- 021 Security review: podpis/TTL/replay/TOCTOU, fałszywy agent, klon, credential leak, overload, recovery.
Odbiór: fixed lab action działa, zły podpis i dowolna akcja odrzucone, bez produkcyjnego ruchu. Przenieść wzorce testów #19–21 po adaptacji.

## 3. Pierwszy prawdziwy host i onboarding (NEXT-022..029)
- 022 Ping/agent.version/facts, typed output, nie-root adaptery Linux.
- 023 Enrollment invite/claim/attempt, hash/TTL/revoke/race, CSR i niezależny fingerprint.
- 024 Signed Linux agent package/release, systemd hardening, state layout.
- 025 Skrypt sakuraops-enroll.sh, --check, prompt/token-file, idempotencja, SELinux, rollback, Bats/ShellCheck.
- 026 Kreator Dodaj serwer, stan etapów, verify release instructions, konkretne komunikaty.
- 027 Cert renewal/revoke, suspend/de-enroll, reinstall i lokalny uninstall.
- 028 Lab QA: dwie różne dystrybucje, niedostępny host, zły CA, mismatch, expired/replayed token, klon, brak sudo i nowego listenera.
- 029 Odbiór UX i instrukcja dodawania hosta, pomiar czasu i poprawki.
Odbiór: jedno zaproszenie + jedno uruchomienie zweryfikowanego skryptu + niezależny fingerprint, host online i fakty. Brak ręcznego YAML/SSH key do automatyzacji. Tylko hosty lab.

## 4. Użyteczny MVP diagnostyczny (NEXT-030..036)
- 030 Zasoby/dyski/uptime, typed observations i freshness.
- 031 Status usług, pakietów, certyfikatów i potrzeby reboot, minimalne output fields.
- 032 Audyt aktualizacji Debian/Ubuntu, potem RHEL/Oracle Linux; cache age/unknown, bez instalacji i refresh.
- 033 Pulpit Wymaga uwagi, lista, karta, filtry, szybkie akcje, prawdziwe dane i daty.
- 034 Katalog i wizard, batch results, retry odczytów i eksport sanitized.
- 035 CLI server/task/run --json i parity uprawnień, help.
- 036 UX E2E, keyboard/mobile, lab regression i brak nieautoryzowanych zmian.
Odbiór MVP: od pustej instalacji do rejestru hostów, diagnostyki, raportu aktualizacji/usług/zasobów. Wszystkie profile modyfikujące i access connector wyłączone.

## 5. Harmonogramy i powiadomienia (NEXT-037..041)
- 037 Scheduler UTC/IANA, occurrence unique, scoped grant, DST/missed runs.
- 038 UI harmonogramów, pause/resume, next dates i blocked reason.
- 039 Lock/no overlap, restart/recovery, owner revoke, stale plan.
- 040 SMTP optional, test, dedup/throttle i redakcja.
- 041 E2E audytu codziennego i awarii DB/gateway/agent; tylko D0.
Odbiór: brak nakładających się runów per host i powtórek po awarii, brak bypass RBAC.

## 6. Connect i read-only pilot (NEXT-042..051)
- 042 Connect ADR, protokół autoryzacji desktop, host key pinning, normalny OS login, privacy/audit model.
- 043 Windows client: login, lista hostów, search/favorites, systemowy token store.
- 044 SSH bezpośredni przez istniejącą sieć/VPN, bez shell interpolation, scope, host pin, rozłączenie i błędy.
- 045 Testy SSH lab, key mismatch, user identity, input injection, token expiry, audyt ograniczeń wariantu A.
- 046 Podpisane obrazy i release Connect, install.sh, konfiguracja pierwszego startu.
- 047 Reverse proxy/mTLS, backup/restore, retencja, monitorowanie, upgrade/rollback.
- 048 Opcjonalne OIDC Keycloak z local fallback, test awarii i mappingu.
- 049 Statyczny import/eksport inventory/CSV bez sekretów i automatycznej aktywacji.
- 050 Load/reconnect storm tests i limity; ustalenie wspieranych wersji.
- 051 Pilot odczytowy i SSH wyłącznie na jawnie wybranych hostach, odbiór ownera/security.
Odbiór: świeża instalacja, backup/restore drill, odczyt i normalna sesja SSH. A nie daje centralnego egzekwowania końca sesji bezpośredniej; nie obiecywać tej funkcji.

## 7. Kontrolowane zmiany (NEXT-052..059)
- 052 ADR privileged helper, Unix peer credentials, oddzielna lokalna polityka i podpis, brak szerokiego sudo.
- 053 Maintenance, second-person approval, explicit target, canary, stop-on-failure, rozbieżność planu.
- 054 D1 package metadata refresh, repo/GPG/lock.
- 055 Z2 security update i selected update, pre/post checks, bez samoczynnego reboot; backup/recovery.
- 056 Service reload/restart/start-stop allowlisted, config validate i health.
- 057 UI potwierdzeń, blokady, efekty, operator vs approver.
- 058 VM snapshot QA: partial failure, cancel/unknown, brak dowolnych komend, recovery.
- 059 Odrębny odbiór każdej klasy, włączenie per host/grupa po zgodzie.
Odbiór: operator nie może uruchomić dowolnego roota; test konkretnej granicy helpera, nie deklaracja oparta tylko na typie formularza.

## 8. Broker sesji, reboot i gotowość operacyjna (NEXT-060..069)
- 060 ADR relay/access connector: osobna tożsamość, lokalne service IDs, zakres, egress, session lifecycle.
- 061 Short-lived signed grant, broker/connector, wyłącznie zatwierdzony SSH service, revoke/idle/max duration.
- 062 Security tests broker: inny host/port, replay, expiry, pivot/SSRF, revoke i log leaks. Nie włączać B przed odbiorem.
- 063 Reboot jawnie, boot ID/health, unknown, canary/wykluczenia.
- 064 Config deploy wyłącznie zatwierdzone artefakty i ścieżki, backup/validate/atomic/postcheck.
- 065 Agent upgrade signed package, drain, canary, kompatybilność, rollback i recovery.
- 066 Testy awarii i odtworzenia per operacja, uzgodniony runbook.
- 067 Ewentualne harmonogramy zmian tylko po osobnym review, grant, frozen target i okno; wyłączone domyślnie.
- 068 Dokumentacja użytkownika/admin/security/DR, testy aktualizacji i dostępności.
- 069 v1.0 readiness review, pilot, decyzja o przełączeniu default branch/release, tag starej wersji.
Odbiór: formalny sign-off, restore, sieć, audyt, revocation, rollback procedure. Bez automatycznego scalenia z dotychczasowym main.

## 9. Rozszerzenia (NEXT-070..077)
- 070 Kompozycja zadań z dozwolonych kroków i typed params, bounded DAG, review.
- 071 Windows agent jako osobny adapter i model usług/aktualizacji; bez parytetu na siłę.
- 072 RDP Connect do wspieranych systemów, weryfikacja cert/NLA, redirections i scope, osobny security review.
- 073 Linux/macOS Connect po ustaleniu toolchain i testach.
- 074 Integracje monitoringu/CMDB, raporty i eksport audytu.
- 075 Opcjonalne wersjonowanie definicji zadań/import Git, immutable manifests, brak execution on push.
- 076 Opcjonalna integracja z Ansible lub urządzeniami sieciowymi wyłącznie po osobnej analizie potrzeb, nie zależność rdzenia.
- 077 Skalowanie wielu stref i HA po pomiarach oraz uzasadnieniu.
Odbiór osobny per rozszerzenie, bez obietnicy wszystkich platform w MVP.

## Macierz testów

| Warstwa | Obowiązkowe przypadki |
| --- | --- |
| Unit | parsing, schema, policy, canonical signatures, status transitions, DST, redaction |
| API integration | DB migrations/transactions, RBAC/IDOR, CSRF, stale/revoke, concurrent claim, locks, SSE resume |
| Agent/security | mTLS, zły CA/cert, klon, replay/TTL, tamper, unsupported action, params injection, nie-root, helper deny, resource pressure, no secrets |
| Lab | dwa systemy, heartbeat/reconnect, offline, power loss, cancel, partial result, stale cache, local journal dedup |
| Enrollment | Bats/ShellCheck, verified release, VM install, idempotence, SELinux, expiry, revoke, mismatch, cleanup/uninstall |
| Remote | prawidłowy SSH, host key mismatch, normalny OS login, scope, token expiry, forbidden port/host, revoke w B, no pivot, no password in logs |
| UI | login, add server, run wizard, forbidden, empty/error/loading, history, keyboard/mobile, no dead controls |
| Operations | clean install, upgrade, backup/restore, rotation, reconnect storm, revoked identity reconciliation |
| Privileged | exact effect, canary fail, postcheck, privilege boundaries, package/service locks, recovery |

Definition of Done: code review, CI PASS rzeczywiście zweryfikowane, kryteria acceptance, testy negatywne, migracja/rollback jeżeli dotyczy, docs i brak sekretów. Manual QA odnotowuje komendy, środowisko testowe, commit SHA, oczekiwany/rzeczywisty wynik; nie kopiować realnych danych do publicznego repo. Brak środowiska = NOT RUN, nie PASS. Ryzykowny PR bez QA nie scalać. Etapy kończą się odbiorem, nie arbitralną datą.

## Decyzje do akceptacji przed implementacją

- Model agent/hub bez Ansible; czy SSH wystarcza na start, czy RDP ma priorytet po MVP.
- Dokładny stack, protokół streaming, issuer/signer, mechanizm rotacji i ograniczenia lokalnego helpera.
- Czy Connect ma własne okno terminala, czy na start systemowy klient; toolchain Qt/C++ jest propozycją.
- Macierz OS/wersji, sieci i limity na podstawie testów, retencja.
- Czy i kiedy broker sesji za NAT jest naprawdę potrzebny; bezpieczeństwo przed wygodą tunelowania.
- Procedura przejścia ze starego repo i prywatnych inventory, ewentualne oczyszczenie publicznej historii.
- Ostateczny design review i jasny motyw dopiero gdy będzie wymagany.

**Pierwszy PR implementacyjny:** NEXT-001..003 (ADR i struktura projektu), następnie lokalny Compose i CI. Nie zaczynać od produkcyjnego agenta root, brokera sesji ani rozbudowanego dashboardu bez działającej ścieżki.
