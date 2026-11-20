# PR #19 — statyczny preflight i zatwierdzenie (bez wykonania)

Ten etap nie uruchamia Ansible, SSH, `ansible-playbook --syntax-check`
ani żadnych playbooków z repozytorium. Zapisywane jest **jedynie**
statyczne badanie źródeł Git i zatwierdzenie administratora.

## Użycie

1. W zakładce **Zadania** wybierz repozytorium, inventory, playbook i grupę.
2. W sekcji **Preflight — zatwierdzanie bez wykonania** wybierz tryb
   `diagnostic` lub `check` (to tylko deklaracja planu).
3. Kliknij **Przeprowadź preflight**.
4. Raport prezentuje commit Git SHA, ścieżki, hosty, moduły, fingerprint
   oraz blokady. Zatwierdzany jest wyłącznie raport bez blokad.
5. **Zatwierdź raport** — backend jeszcze raz sprawdza commit, pliki i
   hosty. Wpis w bazie otrzymuje stan `approved`, ale żadne zadanie
   wykonawcze nie jest tworzone.

## Konfiguracja

Domyślnie lista dozwolonych hostów jest **pusta**, więc wszystkie raporty
są zablokowane. Na potrzeby testów można jawnie dopuszczać nazwy hostów
(zgodne z aliasami widocznymi w inventory):

```bash
export SAKURAOPS_PREFLIGHT_HOSTS="semaphore"
```

Ta zmienna dotyczy **tylko** etapu preflightu. Nie nadaje dostępu SSH i
nie upoważnia późniejszych wykonawców do żadnych zdalnych operacji.

## Restrykcyjny profil diagnostyczny

Obecnie dopuszczane są wyłącznie proste, literalne playbooki YAML:

- `hosts: all` i `gather_facts: false`;
- `ansible.builtin.ping` bez parametrów;
- `ansible.builtin.debug` z krótką literalną wiadomością ASCII;
- `name` opcjonalne; brak innych dyrektyw, ról, include/import,
  pętli, warunków, `become`, `vars`, pluginów i szablonów;
- inventory statyczne INI/YAML, bez hostvars/groupvars, zmiennych,
  aliasów/kotwic YAML i dynamicznych źródeł.

Lista wspieranych modułów jest celowo bardzo krótka. To nie jest
pełna walidacja Ansible — playbooki produkcyjne będą często blokowane.
Odblokowanie nowych modułów wymaga osobnego przeglądu bezpieczeństwa.

## Reguły Git i odbiór

- Inventory i playbook muszą być śledzone, mieć bezpieczne ścieżki
  i nie mogą prowadzić przez symlinki.
- Lokalny HEAD musi odpowiadać ostatnio zsynchronizowanemu SHA.
- Repozytorium nie może mieć niezacommitowanych zmian śledzonych plików.
- Przed zatwierdzeniem ponownie liczone są SHA, skróty plików, lista
  hostów, moduły i odciski raportów. Rozbieżność = odmowa.
- Raport i zatwierdzenie przechowywane są w osobnej tabeli SQLite.
- Maksymalnie 20 hostów oraz 40 zadań; historia ostatnich 30 raportów.
- Endpointy preflight wymagają sesji administratora oraz CSRF.
- Nie istnieje żaden endpoint wykonawczy korzystający z tych raportów.

**Kryterium odbioru:** nowe UI, historia i zatwierdzenie działają;
nieautoryzowane operacje, niebezpieczne playbooki, zmienione SHA i
hosty spoza allowlisty są blokowane; żadne połączenie ze zdalnym hostem
ani uruchomienie Ansible nie następuje.
