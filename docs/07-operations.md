# 07. Instalacja i eksploatacja

## Pierwsze uruchomienie

Jedna VM Linux z Docker Engine i Compose plugin. Proponowany punkt startu lab: 2–4 vCPU, 4–8 GB RAM, 40 GB dysku, hipoteza do pomiaru zależnie od liczby agentów, zadań i logów. Usługi: api/frontend, PostgreSQL, agent-gateway, dispatcher/scheduler, signer/issuer w odrębnej granicy, reverse proxy wewnętrzny lub zewnętrzny. Broker sesji opcjonalny i oddzielony. DB/issuer/IPC prywatne, bez Docker socket w aplikacji. Endpoint agenta dostępny wyłącznie z dopuszczonych sieci (lub jawnie zatwierdzonej trasy). mTLS egzekwowany na dedykowanym listenerze; proxy nie może ufać nagłówkowi certyfikatu od klienta. Wybrać TLS passthrough do gateway albo zweryfikowane termination z autentykowanym kanałem do backendu i usuwaniem podrobionych nagłówków. Enrollment HTTPS osobno od agent mTLS i UI. API nie służy do wydawania arbitralnych certyfikatów.

Docelowy install.sh sprawdza wymagania bez automatycznej zmiany firewalla/systemu, generuje pliki z podpisanego release, pyta o public URL, dozwolone sieci, katalog danych i zaufany CA, tworzy sekrety lokalnie, uruchamia Compose, sprawdza readiness, pokazuje lokalną komendę utworzenia pierwszego admina z TTY bez echa. Brak admin/admin, domyślnego klucza sesji, publicznego bootstrap endpointu. Odmowa startu przy błędnej konfiguracji. .env tylko niesekretne dane lub ścieżki, prywatne pliki z odpowiednimi prawami, Compose secret file refs. CA/issuer key oddzielony od sesji i signer key, offline root CA jeśli zgodne z ADR. Backup CA i rotacja udokumentowane.

Kreator pierwszego wejścia: nazwa instancji, timezone, scope sieci, stan agent gateway, opcjonalnie SMTP/SSO, stan backupu, Dodaj pierwszy serwer. Do działania wystarczy HTTPS, lokalny admin, polityka, issuer i pierwszy agent. Zero cyklicznych zadań, brak helpera i zdalnego dostępu domyślnie. Git i Keycloak nie są wymagane. Przykładowy config w examples jest propozycją schematu, nie działającą konfiguracją.

## Wdrożenie i konfiguracja

- Reproducible images i binarki, lockfiles, przypięte digest, SBOM, skan zależności i obrazów, podpisane release, checksums; osobne dev/test/prod. Bez latest w produkcji.
- Reverse proxy Nginx/NPM lub inny: zaufane forwarded host/proto tylko od zdefiniowanego proxy, limit request body/rate, SSE/WS timeouts, CSP, HSTS po potwierdzeniu HTTPS. Uwaga: obsługa mTLS musi być niezależnie zweryfikowana; nie zakładać że dowolny proxy ją poprawnie przenosi.
- Ustawienia UI: nazwa, strefa, użytkownicy, grupy, powiadomienia, integracje, enrollment; wrażliwe klucze CA/signer i polityka egress poza swobodną edycją operatora.
- OIDC Keycloak opcjonalne: Authorization Code + PKCE, walidacja issuer/aud/exp/nonce/state, JWKS rotation, jawne mapowanie ról, brak auto-link po email, lokalny fallback, test awarii IdP i wyłączenia konta.
- Import istniejących hostów: statyczny preview CSV/inventory, nie importować sekretów, nie aktywować automatycznie, per host enrollment. Pilot na wyizolowanym labie; równoległa praca z dotychczasowym systemem bez nakładających się zmian. Przełączenie po odbiorze.

## Obserwowalność

Liveness procesu i readiness DB/gateway/signer/issuer/katalog, bez sekretów. Dashboard statusu pokazuje przyczynę niedostępnej usługi i nie akceptuje niewykonalnych zadań jako sukces. Logi JSON, request/run/host IDs, redakcja; liczba agentów online/offline/unknown, cert expiry, incompatible versions, queued/running, oldest queue age, failed per typ, disk usage, backup age. Alerty na pad gateway/DB, wzrost backlogu, brak miejsca, nieudany backup, przeterminowanie certyfikatu, niepowodzenie zadań wg reguł. Opcjonalny syslog/monitoring później, bez pełnego NMS.

Proponowana retencja: logi szczegółowe 30 dni, wyniki 90, audyt 365; dostosować do wymagań instytucji i pojemności. Nie usuwać aktywnych runów ani zdarzeń potrzebnych do odtworzenia blokad. Limity per run i globalne, ostrzeżenie przed zapchaniem; archiwizacja/eksport z RBAC. Nie przechowywać treści zdalnych sesji w MVP.

## Backup i recovery

Codzienny szyfrowany backup PostgreSQL, konfiguracji i artefaktów; oddzielne bezpieczne kopie CA/issuer, signer i kluczy szyfrujących zgodnie z polityką. Master key nie razem z kopią DB. Nie umieszczać sekretów w repo. Backup przed upgrade, kontrola checksum i wieku. Proponowane RPO 24h / RTO 4h do potwierdzenia w teście. Restore w odizolowanej sieci: zatrzymaj dispatcher/scheduler/broker, odtwórz wersje obrazów, DB i klucze, zweryfikuj migracje, sesje/tokeny według procedury, oznacz stare runy interrupted/unknown. Przed otwarciem gateway reconcile odwołanych certyfikatów, tokenów i nonce z niezależnym rejestrem lub wymuś rotację, aby backup nie przywrócił wycofanej tożsamości. Nie wysyłać starych zleceń, agent journal i command IDs zachowane, odtworzone granty wstrzymane do review. Harmonogramy nie nadrabiają zmian. Drill raz na kwartał (propozycja), zmierz czas i dokumentuj.

Awaria huba: agent nie uruchamia autonomicznych nowych operacji ani nie przyjmuje nowych kontrolerów, dokańcza lub bezpiecznie przerywa trwającą akcję według profilu, lokalny journal bounded; reconnect i raport wyników po odzyskaniu. Awaria agenta: brak automatycznego ponowienia zmian; stan unknown i ręczna weryfikacja. Kompromitacja hosta: revoke identity, blokada sesji i zadań, osobna procedura reakcji, rotacja. Kompromitacja huba/signer: wstrzymaj wykonanie, revoke/rotate trust, lokalna procedura zawieszenia agenta, review wszystkich runów, nie przywracaj zaufania automatycznie.

## Upgrade centralny i agenta

Release notes, test migracji Goose (osobny jednorazowy migrator, advisory lock, minimalne uprawnienia), miejsce, backup, wstrzymanie nowych runów i drain, wstrzymaj brokera zgodnie z oknem. Pobranie podpisanych obrazów, migracja, health, smoke localhost i lab agent, kompatybilność protokołu N/N-1 tylko jeśli przetestowana, aktywacja harmonogramów po review. Rollback obrazu tylko gdy schema kompatybilna; inaczej odtworzenie backupu z jawnym RPO i reconciliacją. Nie obiecywać bezprzerwowego upgrade na jednej VM. Agent najpierw ręczny upgrade podpisanym pakietem; centralny upgrade później osobnym ograniczonym helperem i testem canary/recovery. Panel i terminal aktualizowane razem z kontrolerem; nie ma osobnych wydań klienta desktopowego. SSH gateway ma osobny health check, drain, test revoke i zgodność z wersją UI.

## Produkcja

Plan nie zatwierdza żadnych zmian na żywych serwerach. Produkcja wymaga jawnej akceptacji właściciela, sieciowych granic, testu restore, ręcznego pilota odczytowego i osobnej zgody na helper/remote access. Dla krytycznych usług zależności i okna zatwierdza właściciel. Utrzymać możliwość powrotu do dotychczasowego systemu bez podwójnego wykonywania zmian.

## Runtime v2 (propozycja)

Wielostopniowy build: SvelteKit static + Vite w etapie Node, kompilacja Go z wbudowanym frontendem i migracjami, osobne artefakty agent/gateway/helper/issuer/signer/worker. Node i Python nie są potrzebne w kontenerach runtime. W produkcji PostgreSQL, reverse proxy, hub i wydzielone procesy zgodnie z granicami zaufania. Agent jako podpisany pakiet/binarka systemd, bez kontenera i osobnego interpretera. gRPC wymaga HTTP/2/ALPN; oddzielny endpoint urządzeń z bezpośrednim mTLS lub zatwierdzonym passthrough. NPM może obsługiwać WWW, ale nie zakładać obsługi mTLS urządzeń bez testu. Szczegóły 10-stack.md.
