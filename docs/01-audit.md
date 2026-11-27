# 01. Audyt punktu wyjścia i decyzja o nowym kierunku

## Zakres

Stan odczytany przez GitHub 27.11.2026: [main @ fdb928e](https://github.com/MiyazakiTakara/sakura-ops/tree/fdb928eb880eb74c8296176ee57d0f63bf6fd5fd), ostatni commit 20.11.2026, scalony [PR #21](https://github.com/MiyazakiTakara/sakura-ops/pull/21). [PR #22](https://github.com/MiyazakiTakara/sakura-ops/pull/22) pozostaje otwarty (head a9ae0474aa5c5a3b755b60753ae7ca0a5e763330). Sprawdzono drzewo, historię PR, README, roadmapę, dokumentację UI, preflightu i wykonawcy, modele, endpointy, strukturę dashboardu, CLI, interfejsy modułów i workflow CI. To przegląd architektury i wybranych fragmentów kodu, nie pełny audyt bezpieczeństwa wszystkich linii. Nie wykonano testów ani połączeń do zarządzanych hostów.

| Obszar | Stan faktyczny | Wniosek dla Next |
| --- | --- | --- |
| Produkt | README zapowiada Keycloak, React/TS, Typer, Ansible Runner i Compose; nie są to w całości dostarczone funkcje | nowy zakres bez wymaganego Ansible i Git |
| UI | backend/app/static/dashboard.html ma osobne sekcje planowania, preflight, lokalnego testu, SSH i ping; harmonogramy/ustawienia nieaktywne | jeden kreator, jeden katalog i historia |
| Terminal | frontend/src/terminal.js, completion.js, line_editor.js; testy JS; pływające okno | zachować wygląd i interakcję skr, nie mylić z prawdziwym SSH |
| Auth | lokalny admin Argon2/SQLite, sesje i CSRF; rola User | zachować zasady i testy, rozwinąć RBAC, optional OIDC |
| DB | SQLAlchemy/SQLite, osobne LocalTestJob, SSHDiagnosticJob, AnsiblePingJob, PreflightReview; osobna DB receipts | wspólny Run/HostResult/Event; migracje PostgreSQL |
| Git | PR #9–13 konfiguracja, sync, edycja, diff, commit, jawny push | Git nie jest wymagany; ewentualny import lub wersjonowanie definicji później |
| Inventory | PR #14 statyczne INI/YAML, preview grup, planowanie | własny rejestr hostów, ewentualny import statyczny |
| Test lokalny | PR #15 kolejka, live log, cancel, localhost | wzorzec stanów i testów, nie zależność wykonawcza |
| SSH/ping | PR #16–17 opt-in, osobne tożsamości i host pinning, allowlista; poprzednie testy opisane w docs | sesje SSH to osobny produktowy moduł, nie boczne wejście do automatyzacji |
| Preflight | PR #19, digest SHA i ograniczony zbiór modułów, zatwierdzenie nie wykonuje | zachować immutable plan i ponowną walidację, zamienić moduły na typowane akcje agenta |
| Executor | PR #20, osobny UID/Unix socket, SO_PEERCRED, admission-only. PR #21 HMAC, nonce, TTL, replay protection. Nie wykonuje zdalnie Ansible | przenieść ideę granic, podpisów i testów negatywnych; nowy agent z niezależną polityką |
| PR #22 | diagnostic_policy.py, docs/FIXED_PING_GATE.md, test_diagnostic_policy.py. Stała polityka jednego hosta testowego, bez połączenia, fingerprint jeszcze nie egzekwuje known_hosts | inspiracja dla pierwszej stałej akcji, nie funkcja main, pozostawić PR bez zmian |
| Konsola | commands.py v0.2.0-dev, show/configure/edit/repository/task/ssh; cli/skr/__init__.py pusty | zbudować rzeczywiste CLI ze wspólnym API; brak dowolnego system shell w skr |
| CI/deploy | checks.yml testuje wybrane pytest, Node i build terminala; trigger push na main i jedną gałąź oraz PR; brak kompletnego Compose/migracji/lockfile Pythona w drzewie | powtarzalne buildy, pełne testy, PR-only CI, wydania bez automatycznego deploy |

Roadmapa starej wersji wskazuje nieaktualny następny PR #19. Nie traktować dokumentów o planach jako dowodu dostarczonej funkcji. Receipt validated nie jest sukcesem zadania. Repo jest publiczne; w starszych materiałach diagnostycznych występuje rzeczywisty prywatny adres i nazwy hostów. Nowy plan ich nie kopiuje. Właściciel powinien osobno ocenić widoczność repo i potrzebę oczyszczenia historii; w tym zadaniu nic nie zmieniamy.

## ADR-000 (propozycja do akceptacji): agent zamiast Ansible

Rozważono trzy modele:
- Ansible push: dojrzałe moduły, agentless, duża gotowa biblioteka; wymaga SSH, inventory i poświadczeń, złożone bezpieczeństwo dowolnych playbooków.
- Własny kontroler SSH bez agenta: mało instalacji, ale nadal utrzymanie dostępu i rozbieżności systemów, brak stałego kanału heartbeat bez dodatkowej usługi.
- Własny jawny agent z wychodzącym kanałem mTLS: prostsze doświadczenie onboarding/status, typowane zadania, brak potrzeby dostępu SSH do automatyzacji i jednolity protokół. Koszt to dystrybucja, aktualizacja, bezpieczeństwo i utrzymywanie adapterów na wielu systemach.

Rekomendowany wariant trzeci. Nie piszemy własnego SSH, RDP, kryptografii, menedżera pakietów ani systemd. Wykorzystujemy istniejące protokoły i lokalne mechanizmy OS. Zaczynamy od Linux i ograniczonego katalogu. Zdalne sesje SSH realizujemy osobno z normalnym uwierzytelnieniem do konta użytkownika; RDP później na wspieranych platformach. Jeśli użytkownik ostatecznie wybierze Ansible, zachować plan UX i rejestru, ale zmienić warstwę wykonawczą w osobnym ADR; nie wdrażać dwóch silników naraz.

## Strategia przepisania

Nie usuwać poprzedniej wersji, nie scalać planu jako zwykłego PR do starego main. Nowa implementacja na dedykowanej gałęzi integracyjnej. Importować małe sprawdzone fragmenty wraz z testami po przeglądzie zależności, licencji i granic zaufania. Nie kopiować baz, poświadczeń, prawdziwych inventory. Dotychczasowe testy stanowią materiał regresyjny, nie dowód bezpieczeństwa nowego systemu. Dane migrować dopiero po zaprojektowaniu i testach jawnego importu.
