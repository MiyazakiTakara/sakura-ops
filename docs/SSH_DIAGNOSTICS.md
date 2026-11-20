# PR #16 — diagnostyka SSH z allowlistą (bez playbooków)

Ten etap dodaje **kontrolowany test łączności SSH**, a nie wykonawcę
dowolnych playbooków. Polecenie jest stałe: na zatwierdzonym adresie IP
jako konto `sakuraops-diag` uruchamiane jest wyłącznie `true`.
Nie są ładowane pliki inventory, `ansible.cfg`, playbooki ani zmienne
z repozytorium Git. Nie są wykonywane `sudo`, `become` ani skrypty.

## Wymagania przed włączeniem

Na **oddzielnej maszynie testowej**, nie na serwerze produkcyjnym uczelni:

1. Utwórz nieuprzywilejowane konto `sakuraops-diag` bez sudo i bez dostępu
   do poufnych danych. Udziel mu wyłącznie prawa logowania SSH.
2. Przygotuj osobny klucz SSH do diagnostyki — nigdy nie używaj istniejącego
   prywatnego klucza administratora ani konta root.
3. Zweryfikuj **poza SakuraOps**, znanym i zaufanym kanałem, fingerprint
   klucza hosta i zapisz go do osobnego pliku `known_hosts`. Nie ustawiaj
   `StrictHostKeyChecking=no` ani nie ufaj automatycznie wynikowi
   `ssh-keyscan`.
4. Ustaw plik klucza prywatnego w trybie 0600. Pliki klucza i `known_hosts`
   muszą należeć do użytkownika procesu SakuraOps. Nie mogą być symlinkami
   ani plikami zapisywalnymi przez grupę lub innych użytkowników.

Konfiguracja środowiska procesu SakuraOps (zastąp przykładowe wartości
rzeczywistymi adresami IP **maszyn laboratoryjnych**):

```bash
export SAKURAOPS_SSH_DIAG_ENABLED=true
export SAKURAOPS_SSH_DIAG_TARGETS="lab=192.0.2.10"
export SAKURAOPS_SSH_DIAG_KEY="/sciezka/poza/repo/sakuraops_diag_ed25519"
export SAKURAOPS_SSH_DIAG_KNOWN_HOSTS="/sciezka/poza/repo/sakuraops_known_hosts"
```

Adresy z puli `192.0.2.0/24` są jedynie przykładami dokumentacyjnymi.
W tej wersji używamy jawnych **numerycznych adresów IP**, bez DNS i bez
adresów lokalnych, multicast, unspecified ani link-local. Lista maksymalnie
20 aliasów w formacie `alias=IPv4` lub `alias=IPv6`, oddzielonych przecinkami.
Nie wpisuj do niej jeszcze adresów GLPI, Libre ani innych maszyn Akademii.

Gdy flaga nie jest ustawiona albo nie ma ważnej konfiguracji kluczy,
przycisk **Sprawdź SSH** pozostaje zablokowany.

## Test w SakuraOps

Po aktualizacji gałęzi i przebudowie frontendu otwórz **Zadania →
Diagnostyka SSH**. Wybierz dopuszczony host i kliknij **Sprawdź SSH**.
SakuraOps żąda potwierdzenia, tworzy zadanie w osobnej kolejce
SQLite, pokazuje historię oraz logi na żywo i umożliwia anulowanie.
Wynik `success` oznacza, że połączenie i stałe polecenie `true`
zakończyły się kodem 0.

Diagnostyka dostępna jest też w konsoli `skr`:
```text
SakuraOps# ssh diagnose lab
SakuraOps# show ssh diagnostics
SakuraOps# show ssh job 1
SakuraOps# ssh cancel 1
```
Tab podpowiada dozwolone aliasy hostów i identyfikatory zadań.

Kolejka ogranicza się do 10 oczekujących/aktywnych wpisów.
Proces działa bez shell, bez agent forwarding, z `BatchMode=yes`,
`StrictHostKeyChecking=yes`, `UserKnownHostsFile` i jawnym kluczem.
Limit 20 sekund na połączenie i wykonanie, odrębna blokada procesu,
zapis wyniku w historii. Przed samym wykonaniem ponownie sprawdzana
jest allowlista i uprawnienia plików.

## Zakres i ograniczenia

**To nie jest sandbox** — proces systemowy posiada uprawnienia konta,
na którym uruchomiono SakuraOps. Docelowo wymaga wydzielonej usługi
wykonawczej, izolacji sieciowej i osobnego nadzoru uprawnień.

W szczególności SSH zdalnie uruchamia jedynie `true`, więc test nie
dowodzi, że na hoście zadziałają moduły Ansible ani Python.
Nie wykonujemy `ansible.builtin.ping`, ani zbierania `facts`.
Nie uruchamiamy też żadnych playbooków z Forgejo w tej wersji.

## Odbiór gałęzi PR #16

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/ssh-diagnostics-preflight
source .venv/bin/activate
python -m pytest -q
cd frontend && npm install && npm run build && cd ..
```

Najpierw uruchom aplikację **bez** zmiennych `SAKURAOPS_SSH_DIAG_*` i
sprawdź, że diagnostyka pozostaje wyłączona. Dopiero po utworzeniu
osobnej laboratoryjnej maszyny testowej skonfiguruj zmienne oraz
przeprowadź próbę rzeczywistego połączenia.

**Nie scalać i nie uruchamiać na produkcji przed ręcznym odbiorem.**
