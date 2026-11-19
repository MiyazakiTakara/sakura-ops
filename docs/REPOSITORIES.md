# Repozytoria Git — wspólna konfiguracja GUI i CLI

Wersja do przeglądu. **SakuraOps tylko zapisuje metadane repozytorium w SQLite.**
Dodanie konfiguracji **nie klonuje ani nie synchronizuje repozytorium** i nie uruchamia Ansible.
Późniejsza obsługa SSH i uwierzytelniania zostanie wdrożona w osobnym etapie.

## GUI
Zaloguj się na konto administratora, wejdź w **Repozytoria** w lewej kapsułce,
podaj nazwę, HTTPS/SSH URL oraz gałąź i kliknij **Dodaj**.
Poniżej znajduje się lista i możliwość usuwania wpisów. Zmiany są trwałe
w bazie `data/sakuraops.db`.

## CLI
Z tego samego konta administratora w konsoli SakuraOps:

```text
SakuraOps# show repositories
SakuraOps# configure terminal
SakuraOps(config)# repository add ansible https://git.amuz.wroc.pl/patryk.nowicki/semaphore-ansible main
SakuraOps(config)# end
SakuraOps# show repositories
```

Usunięcie: `configure terminal`, a następnie `repository delete ansible`.
Nazwy mają 2–40 znaków (małe litery, cyfry, `_` i `-`).
HTTPS i `ssh://git@host/owner/repo.git` to formaty URL obsługiwane na etapie konfiguracji.
Nie wpisuj tokenów ani haseł do adresu URL; walidacja odrzuca znane formaty poświadczeń.

## Bezpieczeństwo i aktualne ograniczenia
- Odczyt, dodawanie i usuwanie wymaga konta `admin` i ważnej sesji/CSRF.
- GUI i CLI korzystają z tej samej warstwy `repositories.py`, więc lista jest wspólna.
- Nie ma jeszcze klonowania, sprawdzania kluczy SSH, pobierania inventory z tych wpisów ani uruchamiania playbooków.
- Poprzednie `show inventory` / `show playbooks` korzystają nadal z istniejącego mechanizmu lokalnego checkoutu (zmienna `SAKURAOPS_ANSIBLE_REPO_PATH`). Nie są jeszcze automatycznie połączone z konfiguracją w SQLite.
- Nie zmieniaj repozytoriów na produkcyjnym Semaphore na podstawie tej funkcji.
- Przed udostępnieniem aplikacji poza localhost potrzebne są dodatkowe zabezpieczenia logowania, audyt zmian i wdrożenie TLS.

## Test lokalny
```bash
cd ~/sakura-ops
git fetch origin
git switch feat/repository-config-gui-cli
source .venv/bin/activate
cd frontend && npm install && npm run build && cd ..
python -m pytest -q tests/test_repository_settings.py tests/test_ui_shell.py tests/test_editor_inventory.py
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload
```

Zweryfikuj dodanie repozytorium w GUI i odczyt komendą `show repositories`.
Następnie usuń je poleceniem w trybie konfiguracji i odśwież widok GUI.
