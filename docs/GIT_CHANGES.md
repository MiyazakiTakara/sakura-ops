# Git — status, commit i świadomy push w SakuraOps

## Obsługa w GUI

Wejdź w **Repozytoria → Zmiany Git**, wybierz repozytorium i kliknij
**Sprawdź status**. Panel pokazuje gałąź, pełny SHA HEAD i zmodyfikowane,
śledzone przez Git pliki `inventory/`, `inventories/`, `playbook/` oraz
`playbooks/`. Wybierz pliki, wpisz opis i świadomie utwórz **lokalny commit**.

Samo utworzenie commita **nie wysyła niczego do Forgejo**. Oddzielny
przycisk **Wyślij do Forgejo (push)** wymaga potwierdzenia i nie używa
`--force`. W przypadku zmiany zdalnej gałęzi Git odmówi non-fast-forward.

## Te same operacje przez konsolę skr

```text
SakuraOps# repository status ansible
Gałąź: main  HEAD: <pełny SHA>
Zmodyfikowane:
inventory/hosts.ini

SakuraOps# repository commit ansible <pełny SHA HEAD> "Aktualizacja inventory" inventory/hosts.ini
Commit lokalny: <nowy SHA> (bez push)

SakuraOps# repository status ansible
SakuraOps# repository push ansible <pełny SHA HEAD po commicie>
```

Wpisuj pełny 40-znakowy SHA, nie skrót. Sprawdzanie SHA zabezpiecza
przed zatwierdzeniem operacji po zmianie stanu lokalnego repozytorium.

## Granice bezpieczeństwa

- Commit obejmuje **wyłącznie jawnie wybrane śledzone i zmodyfikowane**
  pliki inventory/playbooków. Nie da się przekazać dowolnych argumentów Git.
- Commit używa `git commit --only` zamiast `git add .`. Inne pliki
  w indeksie nie zostają przez to dołączone do commita.
- Operacje zapisu i synchronizacji korzystają z jednej blokady repozytorium.
- Zwykły `git push origin HEAD:refs/heads/<branch>` bez `--force`.
  SakuraOps nie wykonuje automatycznego pusha po commicie ani synchronizacji.
- Wymagane jest zalogowanie administratora, a API stosuje ochronę CSRF.
- Push używa zwykłych uprawnień Git użytkownika uruchamiającego serwer
  SakuraOps. Dla repozytorium z dostępem tylko do odczytu push zostanie
  odrzucony przez Forgejo.
- Nie uruchamiamy Ansible, nie wykonujemy `git reset --hard`, nie
  dopuszczamy dowolnej powłoki w konsoli.

## Odbiór

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/git-change-management
source .venv/bin/activate
python -m pytest -q
cd frontend && npm install && npm run build && cd ..
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload --reload-dir backend/app
```

Testy integracyjne działają na lokalnym testowym bare Git i sprawdzają:
wybór plików, commit bez push, osobne push, odmowę nieaktualnego SHA oraz
odmowę non-fast-forward. Nie wymagają połączenia z Forgejo.

**W GUI przetestuj commit i push tylko na osobnym repozytorium testowym**,
nigdy na produkcyjnym `semaphore-ansible` bez wcześniejszego sprawdzenia
zmian. Wdrożenie do środowiska uczelni wymaga dodatkowego audytu uprawnień,
ewentualnych reguł gałęzi i rejestrowania operacji.
