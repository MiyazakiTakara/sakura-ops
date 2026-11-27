# 02. Produkt, interfejs i przepływy

## Kierunek

Administrator ma wiedzieć: co wymaga uwagi, które serwery są dostępne, jakie działanie może wykonać. Nie musi rozumieć mTLS, kolejek, certyfikatów czy adapterów pakietów. Szczegóły widoczne na żądanie. Zgodnie z istniejącym docs/UI_DESIGN.md zachować grafit #15191F, powierzchnie #252B34, terminal #101820, przygaszony róż #D9A0B7 dla wyboru i akcji głównej, zieleń #98CFA8 dla sukcesu terminala. Kontrast do weryfikacji. ENERGY 1 / RHYTHM 2 / MOTION 1: spokojny, treściwy panel, hierarchia przez tekst i odstępy; bez wykresów tylko dla dekoracji, gradientów i nadmiaru kart. Systemowa czcionka, mono tylko dla kodu/logów. Zachować niewysoką kapsułę nawigacji na desktopie, jeśli nie pogarsza dostępności. Ciemny motyw jako zaakceptowana baza; jasny dopiero po pełnym zaprojektowaniu i testach.

## Nawigacja

| Miejsce | Podstawowy widok | Szczegóły na żądanie |
| --- | --- | --- |
| Pulpit | Wymaga uwagi, ostatnie zadania, następne harmonogramy, Dodaj serwer | stan centralnego systemu w ustawieniach |
| Serwery | nazwa, stan agenta, system, grupa, aktualizacje, ostatni kontakt | zasoby, wersja agenta, możliwości, rejestracja, historia |
| Zadania | katalog i zakładka Historia | parametry, wynik per host, logi, zatwierdzenia |
| Harmonogramy | nazwa, następne uruchomienie, cel, ostatni wynik, aktywny/wstrzymany | polityka błędów, strefa, grant, okno |
| Ustawienia | instalacja, użytkownicy, bezpieczeństwo, integracje, kopie | certyfikaty, profile, audyt, limity |
| Terminal | otwiera okno ograniczonej konsoli skr | historia, Tab, help |

Połączenie zdalne jest akcją na karcie/listach serwerów i w Connect, nie osobnym ekranem konfiguracji sieci dla każdego użytkownika. Nie pokazujemy zakładek nieistniejących ani fałszywych wskaźników. Admin widzi dodatkowe sekcje tylko gdy potrzebne.

## Pulpit

Jedna sekcja Wymaga uwagi sortowana po znaczeniu: nieudane zadanie, agent odłączony, przeterminowany certyfikat/wersja, brak sprawdzenia, aktualizacje, wymagany reboot. Wiersz: serwer, przyczyna, czas, akcja. Brak danych oznacza Nie sprawdzono, nie zielone Wszystko działa. Przy pierwszym logowaniu pusty stan z Dodaj pierwszy serwer. Poniżej maks. 5 ostatnich uruchomień i maks. 3 nadchodzące harmonogramy (proponowane limity). Nie udawać monitoringu w czasie rzeczywistym; heartbeat i obserwacja mają czas. Nie zastępujemy NMS.

## Serwery

Lista: wyszukiwanie, filtry grupy/OS/stanu, sortowanie, paginacja. Stan rejestracji pending/active/suspended/retired jest oddzielny od łączności online/offline/unknown. Online wyliczane z ostatniego uwierzytelnionego heartbeat i progu; „offline” nie dowodzi awarii OS, tylko braku kontaktu. Przeterminowane dane oznaczone. Brak odświeżenia audytu nie zeruje liczby aktualizacji.

Karta serwera: nazwa, grupa, stan i czas, trzy główne akcje: Sprawdź teraz, Uruchom zadanie, Połącz (tylko jeśli skonfigurowane i dozwolone). Sekcje Podsumowanie (OS/kernel/uptime/dysk/RAM, aktualizacje, usługi), Historia, Konfiguracja. Certyfikat i identyfikator agenta w szczegółach, z przyczyną ostrzeżeń. Zmiana grup/opisu nie może niejawnie rozszerzać uprawnień. Zmiana tożsamości hosta i dostępu wymaga ponownego potwierdzenia. Retire nie usuwa historii.

### Dodawanie pierwszego lub kolejnego hosta

1. Dodaj serwer: nazwa, środowisko lab/prod, grupa (opcjonalna), Linux. Wybierz profil podstawowy bez uprawnień root.
2. Panel tworzy jednorazowe zaproszenie i pokazuje pobranie zweryfikowanego skryptu oraz polecenie uruchomienia. Token kopiowany osobno, z datą ważności, bez adresu wrażliwego w publicznym repo. Nie wymaga ręcznego YAML ani SSH key.
3. Administrator uruchamia skrypt na nowym hoście. Widzi co zostanie zainstalowane, katalog, konto i adres kontrolera. Kreator pokazuje Etap: pobrano/zgłoszono/oczekuje na potwierdzenie/połączono. Nie obiecujemy wiedzy o pobraniu, jeśli klient go nie raportuje.
4. Porównaj fingerprint zgłoszonego klucza z lokalną konsolą lub zaufanym kanałem, potwierdź host. Po prawidłowym heartbeat i teście diagnostycznym host aktywny.
5. Zobacz fakty i opcjonalnie ustaw harmonogram odczytu. Żadnego automatycznego podwyższenia uprawnień, włączenia zdalnej sesji ani aktualizacji.

Błędy: wygasł token (Wygeneruj nowe zaproszenie), zły CA (Sprawdź certyfikat kontrolera), brak połączenia (Sprawdź DNS/HTTPS), konflikt instalacji (Pokaż instrukcję), fingerprint mismatch (Zablokowano, wymagane wyjaśnienie). Komunikat podaje etap, przyczynę i akcję bez wrażliwego stack trace.

## Zadania

Kategorie: Diagnostyka, System i aktualizacje, Usługi, Agent, Własne zadania (po wdrożeniu). Nazwa po polsku, jednozdaniowy efekt, informacja o zmianie i możliwym przestoju. Jeden katalog, nie ekran per runner.

Kreator:
1. Zadanie i typowane parametry. Jeśli z karty serwera, cel wstępnie wskazany.
2. Cele: konkretne hosty/grupa, wyszukiwanie, lista i liczba, zgodność systemów, offline, wykluczenia. Brak domyślnego „Wszystkie”.
3. Podsumowanie: co zrobi, gdzie, czy zmieni system, czy może przerwać usługę, wymagany profil, okno. Diagnostyka: jawne kliknięcie Uruchom. Zmiana: plan i zatwierdzenie zgodne z polityką, dodatkowa osoba dla ryzykownych operacji. Po zmianie zakresu podgląd od nowa.

Nie wspierany host nie jest cicho pomijany. Operator jawnie usuwa go z zakresu lub naprawia problem. Zaawansowane szczegóły: digest, wersja akcji, parametry, policy revision, agent capability, zdarzenia. Nigdy pole raw shell dla zwykłego operatora.

## Wynik i historia

Nagłówek zadania, cele, stan, start/czas trwania, podsumowanie, Anuluj gdy możliwe. Per host: powodzenie, zmieniono, błąd, niedostępny, pominięty, nieznany stan po przerwaniu. Nie utożsamiać przyjęcia zlecenia z wynikiem. Szczegóły i ograniczone logi na żywo; filtr, stop autoscroll, link do run ID. SSE reconnect; przejście na inną stronę nie przerywa runu. Ponów wybrane nieudane odczyty = nowy plan; brak automatycznego retry zmian. Eksport sanitized CSV/JSON w późniejszym etapie. Przykład fikcyjny: lab-web-01, 7 pakietów dostępnych, 2 oznaczone bezpieczeństwa, cache z podaną datą, nic nie zainstalowano. Jeżeli źródło nie rozróżnia security, pokaż Klasyfikacja niedostępna.

## Harmonogramy

Kreator: zadanie, cele, kiedy (np. codziennie 07:00, Europe/Warsaw jako konfigurowalny przykład), powiadomienia i włącz. Podgląd trzech kolejnych wystąpień. Domyślnie tylko odczyt. Wstrzymaj/aktywuj, historia i Uruchom teraz z normalnym preflightem. Wygasły grant/zmiana zakresu wyświetla blokadę. Nie nadrabiać lawinowo po awarii.

## SakuraOps Connect (desktop)

Aplikacja dla administratora, proponowany pierwszy klient Windows z późniejszym buildem Linux/macOS. Logowanie do kontrolera, lista dostępnych hostów z grupami, filtrem i ulubionymi; brak własnego drugiego rejestru. Kliknięcie Połącz otwiera kartę SSH albo uruchamia wspierany klient systemowy; rozłączenie widoczne, przyczyna odmowy czytelna. Użytkownik nadal uwierzytelnia się do SSH jako indywidualne konto, nie root/agent. Funkcja RDP później, z normalnym logowaniem do systemu. Nie pokazywać przycisku Pulpit zdalny na hostach bez konfiguracji. Nie przechowywać hasła hosta w centralnym systemie. Mechanizm sesji w 06-remote-access.md.

## Konsola skr

Pływające okno jak w istniejącym projekcie: drag/resize/minimize/maximize, fokus, stan przy nawigacji, alternatywy klawiaturowe; na telefonie cały obszar. To nie powłoka OS. Przykładowe komendy projektowe:
- skr server list
- skr server show lab-web-01
- skr task list
- skr run plan linux.resources --host lab-web-01
- skr run start PLAN_ID
- skr run show RUN_ID
- skr run cancel RUN_ID

Prawdziwy CLI używa tego samego API, scoped tokenów i kodów wyjścia, --json. Skr nie omija polityk i nie jest kanałem SSH; Connect to oddzielny interfejs sesji.

## Dostępność i odbiór

Loading/empty/error/forbidden/stale/retry w odpowiednich widokach; formularze zachowują dane bez sekretów. Widoczny fokus, etykiety, Escape i przywrócenie fokusu, komunikat stanu nie tylko kolorem; WCAG AA 4.5:1 dla zwykłego i 3:1 dla dużego tekstu. Reflow desktop/tablet/mobile, bez globalnego poziomego scrolla, touch targets 44px, reduced motion. Test 360, 768, 1280, 1920 px, 200% zoom i klawiatura. Desktop Connect: reflow okien, klawiatura, high DPI. Każdy przycisk działa lub nie jest pokazywany. Odbiór użytkowy: administrator bez dokumentacji technicznej dodaje host, uruchamia diagnostykę, znajduje wynik, rozpoznaje problem z połączeniem, nawiązuje dozwoloną sesję. To kryteria przyszłego testu, nie wyniki już wykonane.
