# SakuraOps PR #15 — lokalny wykonawca testowy Ansible

Pierwsza implementacja kolejki działa **wyłącznie na localhost** i służy
do sprawdzenia mechanizmów wykonywania, logów i zatrzymywania testu.
Nie uruchamia playbooków z Forgejo ani inventory użytkownika.

## Co wykonuje test

Jedyny dozwolony proces:
```text
ansible-playbook --check -i localhost, -c local --limit localhost backend/app/local_test.yml
```
Ścieżka playbooka jest wewnętrzna i stała (wyznaczana z kodu), nie pochodzi
z GUI, CLI, repozytorium Git ani planu zadań. Playbook zawiera wyłącznie
`ansible.builtin.debug`, nie pobiera facts i nie ma operacji zdalnych.
Nawet wybranie planu z inventory `glpi` i `libre` nie może wpłynąć na test.

## Włączenie testów na maszynie deweloperskiej

Domyślnie funkcja jest **wyłączona**. Zainstaluj `ansible-core` i świadomie
włącz ją dla lokalnej sesji deweloperskiej:

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/local-ansible-job-runner
source .venv/bin/activate
python -m pip install ansible-core
export SAKURAOPS_LOCAL_TESTS_ENABLED=true
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload --reload-dir backend/app
```

Przypomnienie: do wykonania testów trzeba przebudować frontend:
`cd frontend && npm install && npm run build && cd ..`.

W **Zadania** jest dodatkowa sekcja **Wykonawca — test lokalny**.
Kliknij **Uruchom test localhost**, wybierz zadanie i obserwuj log.
Przycisk **Zatrzymaj** działa na oczekującym i uruchomionym zadaniu.

Te same funkcje z konsoli `skr`:

```text
SakuraOps# task test
SakuraOps# show jobs
SakuraOps# show job 1
SakuraOps# task cancel 1
```

## Mechanizmy bezpieczeństwa

- Wymagana aktywna sesja administratora i CSRF; funkcja wyłączona domyślnie.
- Jedna aktywna kolejka w procesach API dzięki blokadzie pliku systemowego.
  Limit 10 zadań oczekujących/uruchomionych.
- Stan zadania, ograniczony log do 64 KiB i status anulowania zapisane w SQLite.
- Logi dostarczane przez uwierzytelnione SSE (co ok. 0,5 sekundy).
- Subprocess bez powłoki, stdin zamknięte, pusta konfiguracja Ansible,
  osobny tymczasowy HOME/cwd, ograniczenie CPU, pamięci i pliku wyjściowego.
- Limit czasu 60 sekund; zatrzymanie całej grupy procesu przy anulowaniu.
- Restart procesu oznacza porzucone zadania jako `failed`.
- **Brak** zdalnego uruchamiania playbooków i brak integracji z SSH.
- To ograniczona izolacja procesowa, **nie kontener/pełny sandbox**.
  Do dalszego etapu potrzebne będzie wykonanie w wydzielonym użytkowniku
  lub kontenerze, polityka sekretów, audyt zdarzeń, kontrola współbieżności
  i odporność na awarie.

Nie włączaj `SAKURAOPS_LOCAL_TESTS_ENABLED` na publicznym serwerze
produkcyjnym. Przed obsługą prawdziwych hostów wymagany będzie osobny etap
projektowania wykonawcy z uprawnieniami i weryfikacją allowlist.
