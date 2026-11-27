# AGENTS.md — SakuraOps Next

Instrukcja dla każdego agenta AI i osoby wykonującej prace w repozytorium. Dotyczy całego drzewa Next. Nie jest upoważnieniem do działania na rzeczywistej infrastrukturze, ujawniania danych lub automatycznego scalania.

## Cel i niezmienne granice

- Self-hosted, Linux-only, browser-only. Go (hub, agent, worker, opcjonalny CLI), Svelte 5/TypeScript, PostgreSQL, gRPC/Protobuf/mTLS, REST/OpenAPI/SSE, WSS tylko dla późniejszego SSH. Szczegóły docs/10-stack.md. Zmiana decyzji wymaga ADR i zgody właściciela.
- Pływający terminal skr i CodeMirror to funkcje podstawowe. Zachować UX z DESIGN.md i docs/09-terminal.md. skr to kontrolowany interpreter, nigdy Bash ani powłoka kontrolera.
- Jeden Sakura Task v1 YAML/JSON dla wszystkich zadań, w tym 18 builtin. Jeden parser, plan, worker, polityka i historia. Akcje i adaptery skompilowane w Go; nie przyjmować dowolnego skryptu, pluginu ani polecenia z sieci.
- Lokalny login działa niezależnie od opcjonalnego Keycloak. Zero obowiązkowego Git, Ansible, chmury, Redis, Kubernetes lub klienta desktopowego.
- Domyślnie D0 nie-root, bez helpera, SSH, zmian, automatycznego restartu, harmonogramów modyfikujących ani dostępu produkcyjnego. Rozszerzenia osobno za zgodą właściciela.
- Nie rozluźniać TLS, RBAC, CSRF, host key pinning, podpisów, nonce, izolacji, blokad, wersjonowania, walidacji schematu ani lokalnej polityki w celu przejścia testu.
- Repo publiczne. Sekrety, prawdziwe inventory, adresy wewnętrzne, logi z danymi użytkowników, hasła, tokeny i klucze nigdy w commitach, issue, PR, fixture ani screenshotach. Tylko fikcyjne nazwy i dane testowe.

## Przed rozpoczęciem zadania

1. Przeczytaj README, AGENTS.md, DESIGN.md, aktualne issue NEXT i wskazane dokumenty; sprawdź git status, HEAD, zdalną gałąź, otwarte PR i konflikty zakresu. Nie przyjmuj, że poprzedni agent zakończył zadanie.
2. Ustal acceptance, granice, zależności, testy i ryzyko. Jedna logiczna zmiana na branch/PR. Nie zaczynaj kolejnego gate przed odbiorem poprzedniego. Jeśli kontrakt jest niejasny, zapisz blocker i konkretną propozycję; nie wymyślaj niejawnie publicznego API.
3. Bazą Next jest v2/native-agent-blueprint do jawnej zmiany przez właściciela. Gałąź robocza np. next/014-task-schema. PR wyłącznie do uzgodnionej gałęzi integracyjnej Next, nigdy do legacy main. Nie resetuj, force-pushuj, kasuj cudzych zmian, nie zmieniaj default branch ani nie scalaj bez aktualnej zgody. Zachowaj istniejący PR #22.
4. Najpierw testowalny kontrakt i negatywne testy, następnie najmniejsza działająca ścieżka. Aktualizuj dokumentację z kodem. Nie obiecuj niezaimplementowanych możliwości. Wykonuj zadanie samodzielnie w dostępnym środowisku, nie wymagaj od właściciela przepisywania kodu.

## Standard kodu

Go: gofmt, context/deadline, jawne błędy z przyczyną, typowane DTO, małe interfejsy przy granicach, bez globalnego mutable state, kontrola współbieżności, transakcje, rollback i idempotencja. Zakaz os.Exec przez shell i składania argv z niezwalidowanego inputu; nie przenosić tej reguły na nieograniczoną powłokę w agencie. sqlc/pgx, parametryzowane SQL, Goose forward migration, test upgrade. Nie ukrywać błędów ani nie pomijać testów.

Svelte: TypeScript strict, komponenty o jednej odpowiedzialności, rzeczywiste stany empty/loading/error/forbidden/stale, semantic HTML, dostępne klawiaturą kontrolki, focus, reflow. Shared client API z generowanego kontraktu. GUI nie autoryzuje, serwer egzekwuje wszystko. Zgodność DESIGN.md; brak dekoracyjnych statystyk, martwych przycisków i fikcyjnego sukcesu.

Protobuf/OpenAPI/JSON Schema: contract first; nie zmieniać typu istniejącego pola ani numeru protobuf, reserve usunięte numery; zmiana semantyki = wersja/ADR. Generator przypięty w repo, drift check. Nie podpisywać przypadkowej serializacji protobuf. Test fixture dla canonical bytes. Wersje akcji immutable; nie nadpisywać wbudowanych manifestów opublikowanej wersji.

## Bezpieczeństwo i środowisko

Tylko jawnie autoryzowany lab, bez połączeń do produkcji, skanowania sieci, instalacji na rzeczywistym hoście, rotacji kluczy produkcyjnych, wdrożeń lub restartów bez odrębnej zgody. Używać symulatora, disposable VM, lokalnej sieci i sztucznych tożsamości. Testy helpera i SSH tylko w odizolowanym lab. Nie usuwaj istniejących danych, nie importuj sekretów. Nie wypisuj raw tokenów, env, prywatnych kluczy, pełnych logów czy zawartości terminala. Zgłaszaj podatności prywatnym kanałem, nie w publicznym issue.

## Odbiór i raport

Patrz docs/17-testing.md i docs/18-development-rules.md. Każdy PR ma zakres NEXT, testy pozytywne i negatywne, regresję, instrukcję QA, zagrożenia, migracje i recovery jeśli dotyczy, zmiany docs, wyniki i ograniczenia. CI tylko pull_request; release ręcznie zatwierdzane. Nie omijaj CI, nie dodawaj skip/xfail bez issue i uzasadnienia. PASS tylko przy rzeczywistym uruchomieniu, NOT RUN gdy brak środowiska; link do runu i SHA. Wymagaj aktualnego wyniku dla HEAD, nie starego commita. Manualne QA jest obowiązkowe dla ryzykownych funkcji; właściciel akceptuje produkcję oddzielnie. Jeśli nie masz uprawnień lub narzędzia, powiedz konkretnie co nie zostało zrobione. Nie raportuj planu jako wdrożenia.

Po pracy: sprawdź diff i brak sekretów, zapisz commit(y), wypchnij własny branch, otwórz PR gdy uzgodnione, zaktualizuj checklistę issue i stan gate. Nie zamykaj issue przed faktycznym odbiorem. Zapisz: SHA, testy, co zmieniono, co nie, blokery, następny krok. Nie obiecuj pracy w tle bez rzeczywistego mechanizmu.
