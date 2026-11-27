# DESIGN.md — SakuraOps Next

Status: specyfikacja do implementacji i późniejszego odbioru wizualnego. Nie jest gotową makietą w przeglądarce ani deklaracją zgodności WCAG.

## Charakter

Spokojne centrum administracji Linux. Jedno główne zadanie na ekran, proste polskie nazwy, techniczna głębia po rozwinięciu. Zachowujemy poprzednią identyfikację SakuraOps i terminal przypominający konsolę urządzeń sieciowych. ENERGY 1 / RHYTHM 2 / MOTION 1. Kapsuła nawigacji i realny pływający terminal są wyróżnikiem produktu. Nie kopiować stylistyki innych narzędzi. Bez hero, zbędnych kart statystyk, gradientów, neonów, glassmorphism i dekoracyjnych animacji.

## Tokeny projektu

| Token | Wartość proponowana | Cel |
| --- | --- | --- |
| bg | #15191F | grafit aplikacji |
| panel | #252B34 | powierzchnia treści |
| terminal | #101820 | obszar poleceń |
| elevated | #303741 | menu, dialog |
| text | #F0F2F5 | główny tekst |
| muted | #B9C1CB | opis i metadane |
| line | #424B58 | separatory nieinteraktywne |
| control-border | #788594 | granice kontrolek |
| accent | #D9A0B7 | aktywna pozycja i główna akcja |
| on-accent | #15191F | tekst na różowym |
| success | #98CFA8 | potwierdzony sukces i prompt |
| warning | #E9BC83 | ostrzeżenie |
| danger | #F0A0A0 | błędy i ryzyko |

Kolory stanów służą wyłącznie informacji, nie dekoracji. Dla każdej pary i stanu sprawdzić kontrast 4.5:1 zwykłego tekstu, 3:1 dużego i kontrolek, także fokus. Border line nie zastępuje widocznej granicy inputa. Dark-first (jedyny motyw MVP), jasny dopiero po osobnej kompletnej implementacji.

Font system-ui 14–16 px w jednostkach rem, tytuły 20–24, śródtytuły 16–18; monospaced systemowy dla skr/SSH/YAML/logów. Hierarchia przez wagi i odstępy. Siatka odstępów 4/8/12/16/24/32 px; radius 6 px kontrolki, 10 px panel, 14 px kapsuła; cień tylko dla terminala/dialogu (warstwa nad stroną). Interakcje natychmiastowe, maks. krótka animacja 120–180 ms dla focus/otwarcia; prefers-reduced-motion.

## Shell i nawigacja

Topbar: nazwa produktu, stan instancji dostępny w ustawieniach, powiadomienia gdy wdrożone, konto. Po lewej kompaktowa pływająca kapsuła fit-content z Pulpit, Serwery, Zadania, Harmonogramy (po wdrożeniu), Ustawienia oraz osobna akcja Terminal. Bez pełnej wysokości sidebaru. Historia w Zadaniach, grupy/profile i Aktualizacje w Serwerach, użytkownicy w Ustawieniach. Zaawansowane Git/SSH nie zaśmiecają startu. Pozycja aktywna ma tekst i kolor, nie tylko ikonę. Każda pozycja prowadzi do działającej funkcji; niedostępne opcje nie są pozornymi kontrolkami.

Desktop >=1200 px: kapsuła ok. 180 px, treść płynna, nagłówki i tabele. Tablet 768–1199: kompaktowa nawigacja z czytelnymi etykietami/menu, formularze w jednej kolumnie gdy potrzeba. Telefon <768: widoczny przycisk Menu, lista serwerów jako wiersze z najważniejszymi polami, drugorzędne w szczegółach, terminal pełny dostępny obszar. Brak globalnego poziomego scrolla; lokalny scroll kodu/tabeli z opisem tylko gdy konieczny. Touch 44px, focus ring 2px i kontrast, skip link, aria-live dla wyniku nie dla całych logów, Escape dla dialogów, przywracanie fokusu, 200% zoom i klawiatura. Nie polegać na hover, kolorze lub ikonie bez etykiety.

## Wzorce komponentów

Primary action jedna na widok. Teksty: Dodaj serwer, Sprawdź teraz, Uruchom diagnostykę, Pokaż plan, Zatwierdź zmianę. Destrukcyjne akcje z konkretnym celem i konsekwencją; brak mylącego „OK”. Lista/tabela: sort, filtr, paginacja, liczba zaznaczonych, czytelna selekcja. Puste pole = Nie sprawdzono, Nie dotyczy, Brak danych lub Niedostępne z przyczyną, nie 0. Status Online / Offline / Nieznany dotyczy kontaktu agenta. Stan systemu i świeżość obserwacji oddzielnie. Loading informuje co pobieramy, retry nie powtarza mutacji. Błąd ma kod/request ID w szczegółach i możliwy następny krok. 403 nie zdradza ukrytych obiektów. Stale nie wygląda jak aktualne dane.

## Terminal

Jedno ruchome okno z tytułem, minimize, maximize/restore, close, resize, drag i przywracaniem pozycji. Kapsuła przywołuje okno, routing nie niszczy sesji. Kontekstowe help/?, Tab, historia, Ctrl+L, tryby skr, edytor CodeMirror. Klawiaturowa alternatywa dla drag/resize. Brak powłoki kontrolera. Tekst outputu nie jest HTML; filtrowanie niebezpiecznych escape. SSH dopiero po odbiorze oddzielnego gateway: osobna karta z hostem/kontem, statusem i Rozłącz. Nie zmieniaj skr w SSH. Zamknięcie okna nie anuluje zadań w tle; aktywny SSH wymaga decyzji o rozłączeniu.

## Odbiór

docs/12-design-system.md definiuje widoki, układ i przepływy. Każdy ekran ma realne API i testy 360/768/1280/1920 px, 200% zoom, keyboard-only, empty/loading/error/forbidden/stale. Sprawdzić konsolę przeglądarki, wszystkie kontrolki, kontrast, focus i reflow; zrzuty tylko z fikcyjnymi danymi. Nie nazywać prototypu gotowym UI. Zmiana tożsamości wizualnej lub architektury nawigacji wymaga review właściciela.
