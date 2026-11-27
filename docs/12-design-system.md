# 12. Ekrany i przepływy UI

Źródło nadrzędne: DESIGN.md. Makiety opisowe (nie działający UI); nie tworzą nowych pozycji menu bez funkcji. Polskie UI, klucze API w English; lokalizację przewidzieć, nie wymagać tłumaczenia wszystkiego w MVP.

| Ekran / route | Główny cel | Układ i akcje |
| --- | --- | --- |
| /login | wejście | nazwa, login, hasło, Zaloguj; opcjonalny IdP tylko jeśli skonfigurowany; błąd ogólny |
| /setup (po auth) | pierwsza konfiguracja | nazwa/timezone, status usług, Dodaj pierwszy serwer; żadnego publicznego bootstrap admin |
| / | co wymaga uwagi | lista problemów z przyczyną i akcją; ostatnie max 5 runów; nadchodzące harmonogramy dopiero po wdrożeniu |
| /hosts | odnaleźć host | search, grupa, system, stan, lista; Dodaj serwer; opcjonalny filtr aktualizacji |
| /hosts/new | onboarding | nazwa/grupa/środowisko, profil, zweryfikowany instalator, zaproszenie, fingerprint, postęp |
| /hosts/:id | zarządzanie hostem | nagłówek ze stanem i czasem, Sprawdź teraz / Uruchom zadanie / SSH gdy gotowy; zakładki Podsumowanie, Historia, Konfiguracja |
| /hosts/groups | organizacja | lista grup/tagów, członkowie, zakres; preview zmiany przed zapisem |
| /hosts/profiles | konfiguracja | lista profili, wersja, zgodność, dziedziczenie; preview różnic; publikuj |
| /hosts/updates | audyt pakietów | filtr security/unknown/stale, ostatni cache, tabela host/pakiet/wersja; bez instalacji w MVP |
| /tasks | katalog | wyszukiwanie i kategorie, builtin/custom, opis, wymagania; skopiuj jako własne, edycja po wdrożeniu |
| /tasks/:id | definicja | wersja, wejścia, kroki, kompatybilność, kod YAML/JSON; Uruchom / Kopiuj |
| /tasks/new | własne zadanie | edytor i walidacja, później bloki z tej samej definicji; preview, draft, publish |
| /runs/new | plan | krok 1 parametry, 2 cele, 3 skutki/blokady, 4 wykonanie lub zatwierdzenie; frozen targets |
| /runs | historia | filtry czasu, zadania, hosta, stanu, aktora; wyniki i eksport po odbiorze |
| /runs/:id | wynik | status globalny i per host/krok, czas, logi na żywo, cancel, odświeżanie bez utraty runu |
| /schedules | harmonogramy (5) | zadanie, scope, next occurrence, pause/resume; preview 3 terminów |
| /settings | instancja | sekcje Ogólne, Użytkownicy i dostęp, Bezpieczeństwo, Integracje, Kopie/status; zaawansowane zwinięte |
| Terminal overlay | skr | nie jest route; patrz DESIGN i 09-terminal; oddzielna karta SSH od etapu 6 |

## Makiety tekstowe

Pulpit (desktop): topbar [SakuraOps | konto]; po lewej kapsuła; treść [Wymaga uwagi] tabela przyczyn i przycisków; [Ostatnie wykonania] lista; bez pustych wykresów. Pierwsze wejście zamiast tabel: „Nie dodano żadnego serwera. Dodaj pierwszy serwer”.

Serwery: nagłówek [Serwery | Dodaj serwer], wiersz [Szukaj | Grupa | Stan | System], tabela [Nazwa | Kontakt agenta | System | Grupa | Aktualizacje | Ostatni kontakt]. Szczegóły mają własny route, nie 12 ikon w wierszu. Na telefonie nazwa/status/akcja w wierszu, szczegóły rozwijane, filtr jako dialog.

Karta hosta: nagłówek alias + środowisko + last seen; pod nim podstawowe akcje; Podsumowanie: stan i świeżość zasobów, aktualizacje, usługi. Konfiguracja: grupy, profil, wersja agenta i polityka lokalna, efekt ustawień z pochodzeniem. Nie ujawniać tokenu, cert key lub konfiguracji helpera użytkownikowi bez uprawnienia.

Dodaj serwer: 1 metadane, 2 wygeneruj zaproszenie, 3 uruchom zweryfikowany instalator (token w bezpiecznym prompt), 4 porównaj fingerprint, 5 aktywny po heartbeat/teście. Wygasły token -> nowe zaproszenie; zły fingerprint -> odrzuć; brak połączenia -> konkretna diagnostyka DNS/TLS. Nie zaznaczać etapów na podstawie timera.

Zadania: lista z nazwą, efektem, kategorią, zgodnością i wersją. Wbudowane nieedytowalne; kopia z nowym ID. Formularz generowany z inputs i kontraktu; domyślne wartości jawne. W kreatorze wizualnym krok ma action, parametry, warunek i podgląd YAML; keyboard move up/down zamiast wyłącznie drag. Import błędnej definicji pokazuje ścieżkę/wiersz i nie publikuje.

Plan: pokaż liczbę i listę konkretnych hostów, wersję, skutki, unsupported i ryzyko, okno oraz powód blokady; zmiana zakresu odświeża plan. Nie ma domyślnego „Wszystkie” ani ukrytego uruchomienia. D0 jawne Uruchom, zmiany dodatkowa zgoda zgodnie z polityką. Stan queued != sukces. Wynik per host i krok, obserwacja z timestamp, raw log ograniczony, można wyłączyć autoscroll, reconnect SSE. Cancel jest żądaniem, nie rollback.

## Komunikaty i stany

Wczytywanie danych nie wyświetla wymyślonych liczb. Brak historii -> „Nie uruchomiono jeszcze żadnego zadania”; stale -> „Ostatni odczyt: ...; Sprawdź teraz”; brak uprawnienia -> „Nie masz uprawnienia do tej operacji”; unsupported -> „Ta wersja agenta nie obsługuje zadania”. Nie zdradzaj obcych hostów. Wrażliwe potwierdzenia podają dokładną nazwę hosta i efekt. Podstawowe formularze mają pola konieczne, zaawansowane sekcje zwinięte; przy błędzie dane niesekretne zachowane. Dostępność i QA: DESIGN.md, 17-testing.md.

Każdy widok ma docelowe API w 23-api-contract.md. Nie udostępniać route/przycisku, dopóki rzeczywista funkcja i autoryzacja nie działają. Zakres MVP 11-mvp.md wyznacza kolejność.
