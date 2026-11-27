# 14. Sakura Task v1: jednolity język zadań

Status: projekt kontraktu. Linux-only, Go + Svelte, bez Ansible. Nie ma jeszcze parsera, silnika ani działających adapterów. Schema jest projektem walidacji struktury, nie kompletnym walidatorem. Wszystkie 18 planowanych zadań wbudowanych ma własny manifest w ../builtin/tasks/; wszystkie zadania użytkownika i wbudowane używają identycznego formatu, walidacji, planowania, kolejki, polityki, wykonania i historii. Nie tworzyć osobnych runnerów dla builtin.

## Format

YAML 1.2 w ścisłym podzbiorze JSON albo JSON, jeden dokument, apiVersion=sakuraops/v1, kind=Task. metadata: stabilny slug id, dodatnia wersja definicji, name, opcjonalne description. inputs: mapa typowanych parametrów (string, integer, boolean, string_list), label, opcjonalny default i enum dla skalarów; brak default oznacza wymagany parametr. Typ default/enum musi odpowiadać typowi input; string_list nie ma enum w v1. Bez sekretów w definicji. steps: sekwencja 1..32 kroków z unikalnym id, action ID i version kontraktu akcji, opcjonalnym with i when.

with: literal skalarny albo lista skalarów, lub referencja {input: nazwa}, {output: {step: id, field: nazwa}}. Nie ma interpolacji, eval, template execution, arbitralnych obiektów, zmiennych środowiskowych, pętli, DAG, include ani dowolnego shell. Referencje muszą prowadzić do wcześniejszych kroków i istniejących pól; walidacja zgodności typów. when: left referencja, op eq/ne/gt/gte/lt/lte, right skalar. Bez konwersji typów; porównania porządkowe tylko integer; eq/ne dla zgodnych skalarów. Null, brak danych, unknown i wynik pominiętego kroku nie są false/zero: nierozstrzygnięty warunek blokuje host, false daje skipped. Nie korzystać z outputu innego hosta.

Nie przyjmować hosts, risk, sudo, become, command, executable, adapter, credential, approval lub disable_validation z manifestu. Cele, zgody, ryzyko i okna wynikają z planu i rejestru, nie z dowolnej deklaracji.

## Jedna akcja, wiele adapterów

| Akcja | Ubuntu/Debian | Oracle Linux/RHEL |
| --- | --- | --- |
| linux.facts | wspólny Linux | wspólny Linux |
| linux.resources | proc/sysfs/API | proc/sysfs/API |
| linux.packages.info | APT/dpkg | DNF/RPM |
| linux.updates.audit | adapter APT (cache) | adapter DNF (cache) |
| linux.updates.refresh | adapter APT | adapter DNF |
| linux.updates.security/selected | zatwierdzony plan APT | zatwierdzony plan DNF |
| linux.services.* | systemd | systemd |
| linux.certificates.audit | odczyt dopuszczonych certyfikatów | odczyt dopuszczonych certyfikatów |

ActionContract jest zaufanym, wersjonowanym kontraktem dostarczanym z release (Go): ID, version, typed parameter/output schema, efekty, risk, role, capabilities, timeout i dozwolone adaptery. Adapter wybierany na podstawie zweryfikowanych faktów OS, menedżera pakietów, wersji i capabilities, lokalnie sprawdzany przez agenta. Nie wymuszać dowolnego adaptera, nie zgadywać systemu i nie stosować losowego fallback. Brak kompatybilnego adaptera = unsupported przed startem; brak klasyfikacji security = unknown, nie zero. Wspólny output zawiera znormalizowane pola, jednostki, observed_at, adapter_id/version, status, error_code i changed. Klasy i warunki zgodne z 04-task-catalog.md. Wersje i szczegółowe parametry adapterów w przyszłych kontraktach i testach.

## Walidacja

Parser ma limity wejścia (propozycja 64 KiB, głębokość 16, 32 kroki i inputy), UTF-8, jeden dokument, odrzucenie duplicate keys, aliases/anchors/merge, tagów niestandardowych, nie-string keys, floatów, NaN/Inf, timestamp coercion, niejawnych niestandardowych typów. JSON także odrzuca duplicate keys. Zwykłe stringi Unicode bez normalizacji zmieniającej wartości. Schema Draft 2020-12 sprawdza strukturę; Go sprawdza semantykę (unikalne ID, references, typy, akcje z rejestru, required params, constraints, enum, limits, brak sekretów). Nie pobierać zdalnych $ref. Błędy z path/line dla YAML, JSON pointer i czytelnym kodem. Nie uruchamiać części definicji po błędzie.

Normatywny model danych to JSON; canonicalizacja po ścisłym parsowaniu, ujednoliceniu opcjonalnych brakujących map i weryfikacji typu. Nie wstawiać defaultów niejawnie na różnych warstwach: binding inputów i defaultów odbywa się raz podczas tworzenia planu. Canonical JSON (np. RFC 8785 dla ograniczonych liczb po spike), SHA-256, nie hash tekstu YAML. Równoważny YAML/JSON ma ten sam digest definicji; wersja, kolejność kroków i wartości znaczą. Nie deklarować canonical Protobuf. Test fixtures w etapie 014.

## Wykonanie

Import -> draft -> validate -> publish immutable TaskVersion -> plan. Publikowanie nie uruchamia zadań. Plan wiąże digest definicji, wersje akcji/adapterów, frozen host IDs, inputy, politykę, okno i zatwierdzenia. Risk/efekty/capabilities agregowane konserwatywnie z całej sekwencji, także kroków warunkowych. Każdy host osobna sekwencja, kolejne kroki dopiero po wyniku poprzedniego, domyślnie stop on failure. Worker orkiestruje kroki; agent otrzymuje wyłącznie podpisaną typowaną akcję i lokalnie weryfikuje tożsamość, TTL, nonce, schema, politykę oraz adapter. Agent nie interpretuje YAML ani workflow. D0 nie-root, D1/Z1/Z2 tylko po odrębnym odbiorze i zgodzie; deklaracja nie uruchamia helpera.

Akcja zmieniająca: przed wykonaniem niezależny dokładny efekt plan (np. pakiety, wersje, zależności, usunięcia), freeze i approval; wynik wcześniejszego kroku nie może rozszerzyć zatwierdzonego zakresu. Jeśli effect plan nie jest możliwy przed zgodą, zatrzymaj i poproś o nowy preflight. Unknown po awarii blokuje dalsze zmiany; brak automatycznego retry mutacji lub obietnicy rollback. Reboot i agent upgrade oddzielne zadania, ich sekwencje wymagają odrębnej obsługi reconnect. Wyniki per host/krok, skipped/failed/unknown i zmiany czytelne w GUI i skr.

## Wbudowane i własne

Wszystkie manifesty builtin/tasks/*.yaml ładowane z podpisanego release, indeks informacyjny, identyczny parser i wykonawca jak dla custom. Podstawowe zadania gotowe do użycia po odbiorze konkretnej akcji; pozostałe niewidoczne jako aktywne lub jednoznacznie oznaczone niedostępne. Wbudowane wersje nieedytowalne; Kopiuj jako własne tworzy draft z nowym ID, bez dziedziczenia zgody. Aktualizacja release dodaje nową wersję, nie zmienia starego planu i nie nadpisuje własnej kopii. Harmonogramy pinują wersję i wymagają review przy zmianie. Git opcjonalny, brak auto-execution po push. Katalog ma wspólne filtry, parametry i historię. GUI generuje formularz z inputów; CodeMirror YAML/JSON, walidacja z błędami; kreator blokowy zapisuje ten sam model, nie osobny format.

Rozróżniamy akcję (kod Go z adapterami) i zadanie (deklaratywna kompozycja). Nowa kompozycja nie wymaga aktualizacji agenta, o ile wszystkie kontrakty już obsługuje. Nowy typ akcji wymaga implementacji, review, testów i podpisanego release. Bez zdalnego pobierania dowolnego kodu.

## Przykłady i testy

- examples/tasks/system-health.yaml i .json to identyczna definicja: fakty, zasoby, audyt security.
- package-audit.yaml: input listy pakietów.
- conditional-audit.yaml: warunek na typowanym wyniku.
- builtin/tasks: wszystkie 18 pozycji katalogu, deklaracje projektowe.

CI: schema i semantyka wszystkich builtin i custom fixtures; równoważność YAML/JSON; błędne pole, duplicate keys, aliases, niestandardowe tagi, przepełnienie, rozmiar, brak parametru, zły typ, unknown action/version, forward ref, skipped output, risk escalation, plan tampering, replay, zmiana adaptera, revoked host, timeout/cancel i crash recovery. Testy VM na każdej zatwierdzonej wersji Ubuntu/Debian/Oracle Linux: identyczna definicja, wspólne wyniki z właściwym adapterem; unsupported i stale cache widoczne. Testy uprzywilejowanych operacji wyłącznie w lab po odrębnym review. GUI, skr i harmonogram ten sam API/plan. Nie oznaczać wsparcia systemu ani zadania bez testu. To kryteria przyszłego odbioru, nie wyniki już wykonane.
