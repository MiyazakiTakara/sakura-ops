# 09. Terminal, ciągłość projektu i założenia do zachowania

## Decyzja produktowa

Terminal skr nie jest narzędziem pomocniczym, lecz jednym z dwóch równorzędnych interfejsów SakuraOps. GUI upraszcza codzienną obsługę, skr daje sprawność osobie technicznej, obie ścieżki prowadzą do tych samych usług, kontroli dostępu i historii. Linux-only dotyczy zarządzanych hostów; użytkownik potrzebuje jedynie przeglądarki. Opcjonalny CLI Linux to zwykły tekstowy klient API, nie aplikacja desktopowa.

Źródła: poprzedni main @ fdb928e, docs/UI_DESIGN.md, docs/CONSOLE.md, backend/app/commands.py, console_completion.py, frontend/src/{terminal,line_editor,completion}.js oraz dokumentacja preflightu i wykonawcy. Zachowanie poniżej jest kontraktem planowanym, nie deklaracją działającego kodu.

## Macierz zachowania

| Poprzedni pomysł | Decyzja v2 |
| --- | --- |
| Grafit, przygaszony róż, zielony prompt, zwarta kapsuła po lewej | Zachować kierunek i kolory, testować WCAG AA i reflow |
| Pływające okno terminala, drag/resize/minimize/maximize/close | Zachować, jako jedyne ruchome okno, od pierwszej funkcjonalnej wersji |
| Prompt skr, tryby exec/config/config-server | Zachować i rozwijać z autoryzacją każdego polecenia |
| Help, ?, historia, Tab, Ctrl+L, edycja linii | Zachować kontekstowe podpowiedzi i sprawdzone zachowania |
| Edytor CodeMirror, edit/nano, Ctrl+S | Zachować dla zdefiniowanych plików roboczych i później repozytoriów, bez dostępu do FS systemu |
| show status/version/servers/jobs, configure terminal | Zachować zgodne komendy i tryby; stare aliasy tam gdzie semantyka identyczna |
| Wspólne API GUI i CLI | Zachować; parser nie jest alternatywną ścieżką uprzywilejowaną |
| Git, sync, edycja, diff, commit, push | Opcjonalny zaawansowany moduł, typowane definicje, nie wymóg onboardingu |
| Inventory i playbooki Ansible | Zastąpić rejestrem hostów/grup oraz katalogiem typowanych akcji, opcjonalny statyczny import |
| SSE, kolejka, live log, historia, cancel | Zachować, zunifikować model Run/HostResult/Event |
| Preflight, pin SHA, jawne zgody, podpisy, nonce, izolacja | Zachować koncepcję, rozszerzyć o wersję akcji, frozen targets i lokalną politykę agenta |
| Lokalne logowanie i opcjonalny Keycloak | Zachować, lokalne konto awaryjne i niezależne uprawnienia |
| Lokalny runner, SSH i Ansible ping jako oddzielne panele | Zastąpić jednym katalogiem zadań i jednym przepływem wykonania |
| Interaktywny shell na zarządzanym Linux | Nowa osobna karta SSH WWW, później i tylko po autoryzacji; nie shell w skr |

## Okno i zachowanie

Kapsuła nawigacji Terminal przywołuje lub przywraca istniejące okno. Przesuwanie za pasek tytułu, resize krawędzi/narożnika, maksymalizacja do obszaru aplikacji, przywrócenie poprzedniego rozmiaru, minimalizacja z zachowaniem stanu i close. Stan nie jest tracony podczas routingu; po zmianie viewportu okno nie wychodzi poza dostępny obszar. Przy zmianie rozmiaru dostępna alternatywa klawiaturowa, widoczne etykiety i fokus. Na małych ekranach pełny obszar. Escape zamyka dialog, nie przerywa przypadkowo zadania. Zamknięcie skr nie anuluje runu w tle; jego wynik pozostaje w historii. W przypadku aktywnego SSH potwierdzenie rozłączenia lub minimalizacja. Bez ukrytych zakładek i dekoracyjnych przycisków.

Prompt skr >, skr(config)# i skr(config-server:alias)# (nazwy ostatecznie zgodne z parserem). Up/down po historii bieżącej sesji, Ctrl+L czyści ekran lokalnie, Tab uzupełnia wyłącznie dozwolone polecenia i cele widoczne dla użytkownika. ? i help pokazują dostępne komendy w danym trybie. Brak sekretów w historii i completion; potencjalnie wrażliwe dane nie są argumentami. Input ma limity długości, typowane argumenty, czytelny błąd i kod. Output tekstowy i strukturalny bez renderowania nieufnego HTML/escape sekwencji do UI. Tryb jest stanem interfejsu, nie uprawnieniem.

## Projekt poleceń i parzystość GUI

Przykłady składni docelowej; implementować tylko komendy mające działające endpointy. Zachować dawny styl show/configure terminal zamiast wymagać nowego prefixu w pływającym oknie. Zewnętrzny CLI może używać prefixu skr, np. skr show servers, i --json.

| Komenda | Funkcja GUI / zasada |
| --- | --- |
| help, ?, show commands | Pomoc z aktualnego rejestru i RBAC |
| show version, show status | Stan instancji, bez sekretów |
| show servers, show server ALIAS, show groups | Lista i szczegóły hostów z tym samym scope |
| show tasks, show task ACTION | Katalog zadań i schema |
| task plan ACTION --host ALIAS / --group GROUP | Ten sam preflight co kreator, jawne cele, frozen plan |
| task run PLAN_ID | Uruchom po wymaganej zgodzie; nigdy nie auto-approve |
| task approve PLAN_ID | Wymagana rola, niezależna osoba dla ryzykownych zmian |
| show jobs, show job RUN_ID | Historia i wyniki per host |
| task cancel RUN_ID | Żądanie anulowania, nie obietnica rollback |
| configure terminal | Tryb konfiguracji, bez automatycznego wzrostu uprawnień |
| server ALIAS, description TEKST, end, exit | Edycja zatwierdzonych pól rejestru, walidacja i audyt |
| show schedules | Harmonogramy i blokady |
| files, cat, edit, nano | Ograniczony workspace, edytor, bez arbitralnych ścieżek |
| show repositories, repository sync NAME | Moduł Git, jeśli włączony |
| repository edit/status/commit/push | Walidowany edytor, diff i osobne potwierdzenie commita/push |

Nie zachowujemy komend ssh diagnose/ansible ping jako starych runnerów; nowe akcje diagnostyczne są w katalogu. Stare aliasy mogą kierować do identycznej, bezpiecznej operacji tylko po jawnej dokumentacji, nie do innego działania. Komendy nieobsługiwane zwracają komunikat, nie wykonują polecenia OS. Brak eval, shell metaznaków interpretowanych przez system, arbitrary subprocess lub su. Parser lexuje i waliduje, a usługa aplikacyjna niezależnie egzekwuje role, scope, wersję, plan, idempotency i audyt.

## Edytor i Git

Wbudowany edytor od etapu fundamentu pozwala edytować bezpieczne notatki/definicje robocze z ograniczeniami ścieżek, rozmiaru, konfliktów wersji (ETag), encodingu i symlinków. Ctrl+S zapisuje lokalnie, nie na serwerze zarządzanym. Ostrzeżenie o niezapisanych zmianach, diff, anulowanie, błąd konfliktu. Nie pokazywać niezrealizowanych operacji.

Opcjonalne repozytoria: Git URL i branch walidowane, dedykowane credentials poza Git, ograniczona sieć i workspace, bez hooków, automatycznego wykonywania konfiguracji repo i dowolnych pluginów. Repozytorium importuje wyłącznie zatwierdzone typowane deklaracje katalogu/grup, bez dowolnych skryptów, YAML jako dane z limitami i schema. Sync fast-forward lub jawny konflikt, edycja, diff i commit z autorem, osobny jawny push bez force. Snapshot SHA i digest przy planowaniu. Zmiana definicji unieważnia stary plan; push nie uruchamia runu. Podstawowe zadania oraz dodawanie serwera działają bez Git, nie trzeba konfigurować repozytorium na starcie.

## Prawdziwy SSH

Po oddzielnym odbiorze modułu 06-remote-access.md akcja Połącz SSH dodaje kartę SSH w tym samym oknie. Każda karta pokazuje własny typ (SakuraOps skr albo SSH: host/konto). Input i output są rozdzielone, karta SSH nigdy nie przekazuje danych do interpretera skr, a skr do PTY. Osobny grant, weryfikacja klucza hosta, indywidualna autoryzacja OS. Gateway egzekwuje scope i rozłączenie. Nie ma desktopowej aplikacji.

## Testy i bramki

- Migracja UI: zachowanie wyglądu z poprzedniego projektu, test drag/resize/min/max/close/route/focus, zbyt mały viewport, 200% zoom, 360/768/1280/1920 px, keyboard-only.
- Parser: quoted arguments, Unicode, invalid mode, help/Tab filtrowane po scope, historia bez sekretów, Ctrl+L bez backend side effect, limity, odmowa shell metaznaków, IDOR, CSRF, brak uprawnień.
- Parzystość: identyczny plan i polityka dla GUI i skr, wygasły plan, zmiana hostów, replay, cancel, brak pomijania aprobaty. SSE reconnect i powrót do wyniku po zmianie strony.
- Edytor/Git: path traversal, symlink, invalid schema, zły SHA, konflikt i niezapisane zmiany, unauthorized push, brak auto-run po zapisie/sync.
- SSH: testy 06-remote-access.md oraz rozdzielenie kanałów, nieautoryzowany tab, utrata sieci, close/restore i cofnięcie zgody.
- Reuse: każdy przenoszony moduł z main wymaga przeglądu, adaptacji i nowych testów; nie kopiować bezpośrednio legacy execution ani starych baz z sekretami.

Dopiero po powyższych testach uznajemy odpowiednią funkcję za dostarczoną. Nie wykonywano ich w gałęzi dokumentacyjnej.

## Implementacja w nowym stacku (ADR-002)

Parser i rejestr poleceń w Go, współdzielone DTO i usługi z REST API; CLI Cobra tylko jako klient API, nie duplikacja polityki. Svelte store dla geometrii i stanu okna; xterm.js dla terminala, CodeMirror 6 dla edytora. Nie uruchamiać Node na produkcji. Testy regresji starych zachowań przez Vitest/Playwright i Go, bez kopiowania starego parsera Python w runtime. Kontrolowany skr przez REST i SSE, prawdziwy SSH przez oddzielny WSS gateway. Zachowanie i uprawnienia nie zmieniają się przy wymianie stacku.
